// Command fixaudio automates the E-AC-3-audio fix end-to-end against a
// live Google Drive catalog folder: list every video (root + one level of
// subfolders, mirroring internal/drive/catalog.go's category structure),
// download each one, inspect its audio codec with ffprobe, remux
// (video copy + AAC audio, via ffmpeg — see scripts/fix-audio-codec.sh for
// the same logic as a standalone script) anything incompatible, and
// upload the fixed content back onto the SAME Drive file ID.
//
// Reusing the file ID instead of delete+recreate matters: the catalog
// cache, thumbnail cache, and any user's "continuar assistindo" progress
// are all keyed by fileId, so an in-place content replacement needs no
// follow-up cleanup anywhere else in the app.
//
// WHY THIS RUNS LOCALLY, NOT INSIDE THE DEPLOYED APP:
// Same constraint as scripts/fix-audio-codec.sh — Vercel's Go runtime has
// no ffmpeg and no execution budget for transcoding a feature-length
// file, so this has to run wherever ffmpeg actually lives (your machine).
// What THIS tool adds over the plain script is not having to manually
// download/upload each file yourself — it drives the Drive API directly.
//
// SETUP (one-time, in Google Cloud Console — see README.md for the full
// walkthrough):
//  1. Create a Service Account on the same project as your Drive API key.
//  2. Create a JSON key for it and download it.
//  3. Share the catalog's root Drive folder with the service account's
//     email (…@…iam.gserviceaccount.com) as Editor — a service account
//     has no access to your files until you explicitly share with it,
//     same as sharing with any other Google account.
//
// USAGE (one-shot — fixes what's there right now, then exits):
//
//	go run ./cmd/fixaudio -credentials=/path/to/service-account.json -folder=<DRIVE_FOLDER_ID>
//
// Add -dry-run to see what would be fixed without changing anything.
//
// USAGE (watch mode — this is the "native" fix: run it once, leave it
// running, and every new/replaced video gets caught automatically without
// you re-running anything by hand):
//
//	go run ./cmd/fixaudio -credentials=... -folder=... -watch -interval=10m
//
// Polling, not a Drive push subscription — Drive's real-time change
// notifications need a webhook endpoint with a public HTTPS URL and
// periodic renewal, which is more moving parts than a personal catalog
// warrants. A poll loop is simpler to run unattended (cron, a systemd
// unit, a spare Raspberry Pi, whatever machine has ffmpeg on it) and the
// -state file means each pass after the first only re-downloads files
// that are new or have changed (by Drive's modifiedTime) — it does not
// re-fetch the whole catalog's bytes every interval.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const videoMimePrefix = "video/"

func main() {
	credentials := flag.String("credentials", "", "path to the service account JSON key file (required)")
	folderID := flag.String("folder", os.Getenv("GOOGLE_DRIVE_FOLDER_ID"), "root Drive folder ID (defaults to $GOOGLE_DRIVE_FOLDER_ID)")
	dryRun := flag.Bool("dry-run", false, "list what would be fixed without downloading/uploading anything")
	forceRemux := flag.Bool("force-remux", false, "rewrite compatible MP4 files as streaming-friendly fragmented MP4")
	watch := flag.Bool("watch", false, "keep running, re-scanning the folder every -interval instead of exiting after one pass")
	interval := flag.Duration("interval", 10*time.Minute, "how often to re-scan the folder in -watch mode")
	statePath := flag.String("state", "fixaudio-state.json", "where to persist which file versions have already been checked (-watch mode only, so restarts don't re-download everything)")
	flag.Parse()

	if *credentials == "" {
		log.Fatal("missing -credentials (path to the service account JSON key file)")
	}
	if *folderID == "" {
		log.Fatal("missing -folder (root Drive folder ID) and $GOOGLE_DRIVE_FOLDER_ID is not set")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		log.Fatal("ffmpeg not found on PATH — install with 'brew install ffmpeg'")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		log.Fatal("ffprobe not found on PATH — install with 'brew install ffmpeg'")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv, err := drive.NewService(ctx, option.WithCredentialsFile(*credentials))
	if err != nil {
		log.Fatalf("drive.NewService: %v", err)
	}

	state := loadState(*statePath)

	if !*watch {
		failed := runPass(ctx, srv, *folderID, *dryRun, *forceRemux, state)
		saveState(*statePath, state)
		if failed > 0 {
			os.Exit(1)
		}
		return
	}

	log.Printf("watch mode: re-scanning %q every %s (Ctrl-C to stop)", *folderID, *interval)
	for {
		runPass(ctx, srv, *folderID, *dryRun, *forceRemux, state)
		saveState(*statePath, state)

		select {
		case <-ctx.Done():
			log.Printf("stopping (signal received)")
			return
		case <-time.After(*interval):
		}
	}
}

var errSkip = fmt.Errorf("skip")

// watchState maps a Drive file ID to the modifiedTime it had the last time
// this tool inspected it — so a re-scan can tell "already checked, nothing
// changed since" apart from "new or replaced, needs a look" without
// re-downloading every file's bytes just to find out.
type watchState struct {
	path    string
	Checked map[string]string `json:"checked"` // fileID -> modifiedTime
}

func loadState(path string) *watchState {
	s := &watchState{path: path, Checked: map[string]string{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return s // no state file yet — first run, that's fine
	}
	if err := json.Unmarshal(data, s); err != nil {
		log.Printf("state: %q is unreadable (%v) — starting fresh", path, err)
		return &watchState{path: path, Checked: map[string]string{}}
	}
	s.path = path
	return s
}

func saveState(path string, s *watchState) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		log.Printf("state: marshal failed: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		log.Printf("state: write %q failed: %v", path, err)
	}
}

// runPass does one full scan-and-fix cycle and returns the number of
// failures (0 in the common case). Files whose modifiedTime matches what's
// already recorded in state are skipped without downloading — the whole
// point of -watch mode is that steady-state passes are cheap.
func runPass(ctx context.Context, srv *drive.Service, folderID string, dryRun, forceRemux bool, state *watchState) int {
	videos, err := collectVideos(srv, folderID)
	if err != nil {
		log.Printf("listing catalog: %v", err)
		return 1
	}

	var toCheck []*drive.File
	for _, v := range videos {
		if state.Checked[v.Id] == v.ModifiedTime {
			continue
		}
		toCheck = append(toCheck, v)
	}
	log.Printf("found %d video(s), %d new/changed since last check", len(videos), len(toCheck))
	if len(toCheck) == 0 {
		return 0
	}

	tmpDir, err := os.MkdirTemp("", "fixaudio-*")
	if err != nil {
		log.Printf("create temp dir: %v", err)
		return 1
	}
	defer os.RemoveAll(tmpDir)

	var fixed, skipped, failed int
	for i, v := range toCheck {
		if ctx.Err() != nil {
			break // shutting down — leave the rest for the next pass
		}
		log.Printf("[%d/%d] %s (%s)", i+1, len(toCheck), v.Name, v.Id)

		modifiedTime := v.ModifiedTime
		if err := processOne(ctx, srv, v, tmpDir, dryRun, forceRemux); err != nil {
			if err == errSkip {
				skipped++
				if !dryRun {
					state.Checked[v.Id] = modifiedTime
				}
				continue
			}
			log.Printf("  FAIL: %v", err)
			failed++
			continue // don't record state — worth retrying next pass
		}
		fixed++
		// The upload changed modifiedTime again; re-fetch it so the next
		// pass recognizes this exact (now-fixed) version and leaves it
		// alone instead of reprocessing its own output forever.
		if updated, err := srv.Files.Get(v.Id).Fields("modifiedTime").Context(ctx).Do(); err == nil {
			state.Checked[v.Id] = updated.ModifiedTime
		}
	}

	log.Printf("pass done. fixed=%d skipped=%d failed=%d", fixed, skipped, failed)
	return failed
}

// collectVideos mirrors internal/drive/catalog.go's BuildCatalog: root
// folder's own videos, plus one level of subfolders' videos. Kept
// separate (not imported from internal/drive) since that package is built
// around the read-only API-key client, not an OAuth-authenticated
// *drive.Service — different auth model, not worth forcing into one
// interface for a tool that runs once in a while.
func collectVideos(srv *drive.Service, rootFolderID string) ([]*drive.File, error) {
	children, err := listChildren(srv, rootFolderID)
	if err != nil {
		return nil, err
	}

	var videos []*drive.File
	var folders []*drive.File
	for _, f := range children {
		switch {
		case f.MimeType == "application/vnd.google-apps.folder":
			folders = append(folders, f)
		case strings.HasPrefix(f.MimeType, videoMimePrefix):
			videos = append(videos, f)
		}
	}

	for _, folder := range folders {
		items, err := listChildren(srv, folder.Id)
		if err != nil {
			log.Printf("  skipping subfolder %q: %v", folder.Name, err)
			continue
		}
		for _, it := range items {
			if strings.HasPrefix(it.MimeType, videoMimePrefix) {
				videos = append(videos, it)
			}
		}
	}
	return videos, nil
}

func listChildren(srv *drive.Service, folderID string) ([]*drive.File, error) {
	var out []*drive.File
	call := srv.Files.List().
		Q(fmt.Sprintf("'%s' in parents and trashed = false", folderID)).
		Fields("nextPageToken, files(id, name, mimeType, modifiedTime)").
		PageSize(1000)

	return out, call.Pages(context.Background(), func(page *drive.FileList) error {
		out = append(out, page.Files...)
		return nil
	})
}

// processOne downloads one file, checks its audio codec, remuxes it if
// needed, and uploads the result back onto the same file ID. Returns
// errSkip (not a real error) when the file is already fine.
func processOne(ctx context.Context, srv *drive.Service, f *drive.File, tmpDir string, dryRun, forceRemux bool) error {
	inPath := filepath.Join(tmpDir, f.Id+"-in"+filepath.Ext(f.Name))

	if err := downloadFile(ctx, srv, f.Id, inPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer os.Remove(inPath)

	codecs, err := probeCodecs(inPath)
	if err != nil {
		return fmt.Errorf("ffprobe: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(f.Name))
	videoCompatible := codecs.Video == "h264" && (codecs.PixelFormat == "yuv420p" || codecs.PixelFormat == "yuvj420p")
	audioCompatible := codecs.Audio == "aac" || codecs.Audio == "mp3"
	containerCompatible := ext == ".mp4" || ext == ".m4v"
	if videoCompatible && audioCompatible && containerCompatible && !forceRemux {
		log.Printf("  skip: already video=%s profile=%s pix_fmt=%s level=%d audio=%s in %s", codecs.Video, codecs.VideoProfile, codecs.PixelFormat, codecs.VideoLevel, codecs.Audio, ext)
		return errSkip
	}

	log.Printf("  video=%s profile=%s pix_fmt=%s level=%d audio=%s container=%s -> writing streaming-friendly h264/yuv420p/aac/mp4", codecs.Video, codecs.VideoProfile, codecs.PixelFormat, codecs.VideoLevel, codecs.Audio, ext)
	if dryRun {
		log.Printf("  (dry-run, not uploading)")
		return errSkip
	}

	outPath := filepath.Join(tmpDir, f.Id+"-out.mp4")
	if err := convertForBrowsers(inPath, outPath, videoCompatible, audioCompatible); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	defer os.Remove(outPath)

	newName := strings.TrimSuffix(f.Name, filepath.Ext(f.Name)) + ".mp4"
	if err := uploadInPlace(ctx, srv, f.Id, newName, outPath); err != nil {
		return fmt.Errorf("upload: %w", err)
	}

	log.Printf("  ok: uploaded as %s (same file ID, catalog/progress links stay valid)", newName)
	return nil
}

func downloadFile(ctx context.Context, srv *drive.Service, fileID, destPath string) error {
	resp, err := srv.Files.Get(fileID).Context(ctx).Download()
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.ReadFrom(resp.Body)
	return err
}

type mediaCodecs struct {
	Video        string
	Audio        string
	VideoProfile string
	PixelFormat  string
	VideoLevel   int
}

func probeCodecs(path string) (mediaCodecs, error) {
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_type,codec_name,profile,pix_fmt,level", "-of", "json", path).Output()
	if err != nil {
		return mediaCodecs{}, err
	}
	var probe struct {
		Streams []struct {
			CodecType   string `json:"codec_type"`
			CodecName   string `json:"codec_name"`
			Profile     string `json:"profile"`
			PixelFormat string `json:"pix_fmt"`
			Level       int    `json:"level"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		return mediaCodecs{}, err
	}
	var codecs mediaCodecs
	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			if codecs.Video == "" {
				codecs.Video = stream.CodecName
				codecs.VideoProfile = stream.Profile
				codecs.PixelFormat = stream.PixelFormat
				codecs.VideoLevel = stream.Level
			}
		case "audio":
			if codecs.Audio == "" {
				codecs.Audio = stream.CodecName
			}
		}
	}
	if codecs.Video == "" || codecs.Audio == "" {
		return mediaCodecs{}, fmt.Errorf("missing video or audio stream (video=%q audio=%q)", codecs.Video, codecs.Audio)
	}
	return codecs, nil
}

func convertForBrowsers(inPath, outPath string, copyVideo, copyAudio bool) error {
	args := []string{"-y", "-v", "error", "-i", inPath, "-map", "0:v:0", "-map", "0:a:0"}
	if copyVideo {
		args = append(args, "-c:v", "copy")
	} else {
		args = append(args, "-c:v", "libx264", "-preset", "medium", "-crf", "20", "-pix_fmt", "yuv420p")
	}
	if copyAudio {
		args = append(args, "-c:a", "copy")
	} else {
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	}
	// A regular MP4 with moov moved to the front exposes the real total
	// duration immediately. Do not use empty_moov here: it deliberately
	// writes duration zero, which leaves mobile Safari stuck at 0:00.
	args = append(args, "-movflags", "+faststart", outPath)
	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func uploadInPlace(ctx context.Context, srv *drive.Service, fileID, newName, contentPath string) error {
	f, err := os.Open(contentPath)
	if err != nil {
		return err
	}
	defer f.Close()

	update := &drive.File{Name: newName, MimeType: "video/mp4"}
	_, err = srv.Files.Update(fileID, update).Media(f).Context(ctx).Do()
	return err
}
