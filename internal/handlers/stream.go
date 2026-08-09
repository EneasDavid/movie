package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"movie/internal/appctx"
	"movie/internal/drive"
	"movie/internal/httpx"
)

// maxCacheableRangeBytes bounds how large a single Range request can be
// before we stop trying to serve it chunk-by-chunk from cache and just
// pass it straight through to Drive instead. Small/aligned ranges are the
// common case for seeking and for repeated views of the same intro —
// exactly what benefits from caching. A player's big sequential "give me
// the rest of the file" request wouldn't benefit (it's read once) and
// would just mean looping fetch-then-cache over dozens of chunks for
// nothing, so it goes straight through.
const maxCacheableRangeBytes = 8 * 1024 * 1024 // 8MB

const metaTTL = 30 * time.Minute

type fileMeta struct {
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
}

// Stream serves GET/HEAD /api/stream?id=<fileID>, proxying video bytes
// from Google Drive with full Range support so the browser's native
// seek/buffer behavior works exactly like it would against a real CDN —
// while never exposing the Drive API key to the client and keeping the
// hot path (recently played/seeked ranges) served from Redis instead of
// re-fetched from Drive every time.
//
// Method/rate-limit/security-header handling lives in the thin /api
// wrapper (see api/stream.go), not here.
func Stream(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()

	fileID := r.URL.Query().Get("id")
	if !drive.IsValidFileID(fileID) {
		httpx.WriteJSONError(w, http.StatusBadRequest, "id inválido")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	meta, err := getMeta(ctx, a, fileID)
	if err != nil {
		log.Printf("stream: getMeta(%s) failed: %v", fileID, err)
		httpx.WriteJSONError(w, http.StatusBadGateway, "não foi possível obter informações do vídeo")
		return
	}
	if meta.Size <= 0 || !strings.HasPrefix(meta.MimeType, "video/") {
		httpx.WriteJSONError(w, http.StatusNotFound, "arquivo de vídeo não encontrado ou não compartilhado publicamente")
		return
	}

	start, end, hasRange, valid := parseRange(r.Header.Get("Range"), meta.Size)
	if hasRange && !valid {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", meta.Size))
		httpx.WriteJSONError(w, http.StatusRequestedRangeNotSatisfiable, "range inválido")
		return
	}
	if !hasRange {
		start, end = 0, meta.Size-1
	}

	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", meta.MimeType)
	w.Header().Set("Cache-Control", "private, max-age=3600")

	if r.Method == http.MethodHead {
		if hasRange {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, meta.Size))
			w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
			w.WriteHeader(http.StatusPartialContent)
		} else {
			w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
			w.WriteHeader(http.StatusOK)
		}
		return
	}

	length := end - start + 1
	if hasRange && length <= maxCacheableRangeBytes {
		serveFromCache(ctx, w, a, fileID, start, end, meta.Size)
		return
	}
	servePassThrough(ctx, w, a, fileID, r.Header.Get("Range"), hasRange, meta.Size)
}

func getMeta(ctx context.Context, a *appctx.App, fileID string) (fileMeta, error) {
	var m fileMeta
	if a.Redis.GetMeta(ctx, fileID, &m) {
		return m, nil
	}
	f, mimeType, err := a.Drive.FileMeta(ctx, fileID)
	if err != nil {
		return fileMeta{}, err
	}
	m = fileMeta{MimeType: mimeType, Size: f.SizeBytes}
	_ = a.Redis.SetMeta(ctx, fileID, m, metaTTL)
	return m, nil
}

// parseRange parses a single-range "bytes=start-end" Range header (the
// only form real video players send). hasRange is false when the header
// is absent; valid is false when present but unsatisfiable.
func parseRange(header string, size int64) (start, end int64, hasRange, valid bool) {
	if header == "" {
		return 0, 0, false, true
	}
	hasRange = true
	if !strings.HasPrefix(header, "bytes=") {
		return 0, 0, true, false
	}
	spec := strings.Split(strings.TrimPrefix(header, "bytes="), ",")[0]
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, true, false
	}

	if parts[0] == "" {
		// Suffix range "-N": last N bytes.
		n, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, true, false
		}
		start = size - n
		if start < 0 {
			start = 0
		}
		return start, size - 1, true, true
	}

	s, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || s < 0 {
		return 0, 0, true, false
	}
	start = s
	if parts[1] == "" {
		end = size - 1
	} else {
		e, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, true, false
		}
		end = e
	}
	if end >= size {
		end = size - 1
	}
	if start > end || start >= size {
		return 0, 0, true, false
	}
	return start, end, true, true
}

// serveFromCache assembles the requested [start,end] range from
// fixed-size cached chunks, fetching-and-caching only the chunks that are
// missing. Chunk-aligned repeat requests (rewinding to the start, two
// people watching the same title) end up served entirely from Redis.
func serveFromCache(ctx context.Context, w http.ResponseWriter, a *appctx.App, fileID string, start, end, size int64) {
	chunkSize := a.Config.ChunkSize
	firstIdx := start / chunkSize
	lastIdx := end / chunkSize

	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	w.WriteHeader(http.StatusPartialContent)

	flusher, canFlush := w.(http.Flusher)

	for idx := firstIdx; idx <= lastIdx; idx++ {
		chunkStart := idx * chunkSize
		chunkEnd := chunkStart + chunkSize - 1
		if chunkEnd >= size {
			chunkEnd = size - 1
		}

		data, ok := a.Redis.GetChunk(ctx, fileID, idx)
		if !ok {
			fetched, err := a.Drive.FetchRange(ctx, fileID, chunkStart, chunkEnd)
			if err != nil {
				return // response already committed as 206; stop best-effort
			}
			data = fetched
			_ = a.Redis.SetChunk(ctx, fileID, idx, data, a.Config.ChunkTTL)
		}

		lo, hi := int64(0), int64(len(data))
		if idx == firstIdx {
			lo = start - chunkStart
		}
		if idx == lastIdx {
			hi = end - chunkStart + 1
		}
		if lo < 0 {
			lo = 0
		}
		if hi > int64(len(data)) {
			hi = int64(len(data))
		}
		if lo < hi {
			if _, err := w.Write(data[lo:hi]); err != nil {
				return
			}
		}
		if canFlush {
			flusher.Flush()
		}
	}
}

// servePassThrough forwards the request directly to Drive and streams the
// response body straight to the client without buffering — used for large
// or unaligned ranges where chunk caching wouldn't help.
func servePassThrough(ctx context.Context, w http.ResponseWriter, a *appctx.App, fileID, rangeHeader string, hasRange bool, size int64) {
	resp, release, err := a.Drive.OpenStream(ctx, fileID, rangeHeader)
	if err != nil {
		log.Printf("stream: OpenStream(%s) failed: %v", fileID, err)
		httpx.WriteJSONError(w, http.StatusBadGateway, "erro ao transmitir vídeo")
		return
	}
	defer release()
	defer resp.Body.Close()

	if cr := resp.Header.Get("Content-Range"); cr != "" {
		w.Header().Set("Content-Range", cr)
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}

	status := http.StatusOK
	if hasRange && resp.StatusCode == http.StatusPartialContent {
		status = http.StatusPartialContent
	}
	w.WriteHeader(status)
	_, _ = io.Copy(w, resp.Body)
}
