// Package drive talks to the public Google Drive REST API (v3) using a
// simple API key — no OAuth, no service account, matching the "single
// user, no login" requirement. It only works against files/folders shared
// as "Anyone with the link".
package drive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"movie/internal/store"
)

const apiBase = "https://www.googleapis.com/drive/v3/files"

type Client struct {
	apiKey string
	http   *http.Client
	sem    *store.Semaphore // bounds concurrent upstream calls
}

func NewClient(apiKey string, maxInFlight int) *Client {
	return &Client{
		apiKey: apiKey,
		http: &http.Client{
			Timeout: 20 * time.Second,
		},
		sem: store.NewSemaphore(maxInFlight),
	}
}

// rawFile mirrors the subset of the Drive `File` resource we care about.
type rawFile struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	MimeType           string `json:"mimeType"`
	ThumbnailLink      string `json:"thumbnailLink"`
	Size               string `json:"size"`
	CreatedTime        string `json:"createdTime"`
	ModifiedTime       string `json:"modifiedTime"`
	VideoMediaMetadata *struct {
		DurationMillis string `json:"durationMillis"`
		Width          int    `json:"width"`
		Height         int    `json:"height"`
	} `json:"videoMediaMetadata"`
}

type listResponse struct {
	Files         []rawFile `json:"files"`
	NextPageToken string    `json:"nextPageToken"`
}

// File is our simplified, JSON-friendly view of a Drive item.
type File struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	MimeType     string `json:"mimeType"`
	IsFolder     bool   `json:"isFolder"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"` // proxied, not the raw Drive link
	SizeBytes    int64  `json:"sizeBytes,omitempty"`
	DurationMs   int64  `json:"durationMs,omitempty"`
	CreatedTime  string `json:"createdTime,omitempty"`
	ModifiedTime string `json:"modifiedTime,omitempty"`
}

var apiErrStatus = map[int]string{
	403: "acesso negado (verifique se a pasta/arquivo está compartilhado como 'Qualquer pessoa com o link' e se a API key é válida)",
	404: "não encontrado (verifique o ID da pasta)",
	429: "limite de requisições do Google Drive atingido",
}

// listChildren lists all direct children of a folder, following pagination.
func (c *Client) listChildren(ctx context.Context, folderID string) ([]File, error) {
	var out []File
	pageToken := ""

	for {
		q := url.Values{}
		q.Set("q", fmt.Sprintf("'%s' in parents and trashed = false", folderID))
		q.Set("fields", "nextPageToken,files(id,name,mimeType,thumbnailLink,size,createdTime,modifiedTime,videoMediaMetadata(durationMillis,width,height))")
		q.Set("pageSize", "1000")
		q.Set("key", c.apiKey)
		q.Set("orderBy", "folder,name_natural")
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}

		reqURL := apiBase + "?" + q.Encode()

		body, err := c.getWithRetry(ctx, reqURL)
		if err != nil {
			return nil, err
		}

		var resp listResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("decode drive response: %w", err)
		}

		for _, rf := range resp.Files {
			out = append(out, toFile(rf))
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return out, nil
}

func toFile(rf rawFile) File {
	f := File{
		ID:           rf.ID,
		Name:         rf.Name,
		MimeType:     rf.MimeType,
		IsFolder:     rf.MimeType == "application/vnd.google-apps.folder",
		CreatedTime:  rf.CreatedTime,
		ModifiedTime: rf.ModifiedTime,
	}
	if rf.ThumbnailLink != "" {
		f.ThumbnailURL = "/api/thumbnail?id=" + url.QueryEscape(rf.ID)
	}
	if rf.Size != "" {
		if n, err := strconv.ParseInt(rf.Size, 10, 64); err == nil {
			f.SizeBytes = n
		}
	}
	if rf.VideoMediaMetadata != nil && rf.VideoMediaMetadata.DurationMillis != "" {
		if n, err := strconv.ParseInt(rf.VideoMediaMetadata.DurationMillis, 10, 64); err == nil {
			f.DurationMs = n
		}
	}
	return f
}

// getWithRetry performs a GET with a couple of retries on transient
// failures (429/5xx), respecting context cancellation, and bounded by the
// concurrency semaphore so we never flood Drive with parallel requests.
func (c *Client) getWithRetry(ctx context.Context, reqURL string) ([]byte, error) {
	c.sem.Acquire()
	defer c.sem.Release()

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 400 * time.Millisecond):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			return data, err
		}

		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("drive API status %d: %s", resp.StatusCode, truncate(string(data), 300))
			continue // retryable
		}

		if msg, ok := apiErrStatus[resp.StatusCode]; ok {
			return nil, fmt.Errorf("drive API erro %d: %s", resp.StatusCode, msg)
		}
		return nil, fmt.Errorf("drive API status %d: %s", resp.StatusCode, truncate(string(data), 300))
	}
	return nil, fmt.Errorf("drive API: esgotadas as tentativas: %w", lastErr)
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// FileMeta fetches metadata for a single file (used by the streaming proxy
// to get its size/mimeType before serving range requests).
func (c *Client) FileMeta(ctx context.Context, fileID string) (*File, string, error) {
	q := url.Values{}
	q.Set("fields", "id,name,mimeType,thumbnailLink,size,createdTime,videoMediaMetadata(durationMillis,width,height)")
	q.Set("key", c.apiKey)
	reqURL := apiBase + "/" + url.PathEscape(fileID) + "?" + q.Encode()

	body, err := c.getWithRetry(ctx, reqURL)
	if err != nil {
		return nil, "", err
	}
	var rf rawFile
	if err := json.Unmarshal(body, &rf); err != nil {
		return nil, "", fmt.Errorf("decode drive file: %w", err)
	}
	f := toFile(rf)
	return &f, rf.MimeType, nil
}

// MediaURL returns the direct downloadable/streamable URL for a file,
// including the API key. Never expose this URL to the browser directly —
// it must only be used server-side by the streaming proxy, so the key
// stays secret and every request passes through our cache/rate-limit
// layer.
func (c *Client) MediaURL(fileID string) string {
	q := url.Values{}
	q.Set("alt", "media")
	q.Set("key", c.apiKey)
	return apiBase + "/" + url.PathEscape(fileID) + "?" + q.Encode()
}

// ThumbnailSourceURL fetches the current thumbnailLink for a file (these
// are short-lived Google-hosted URLs), used once per cache-miss by the
// thumbnail proxy.
func (c *Client) ThumbnailSourceURL(ctx context.Context, fileID string) (string, error) {
	q := url.Values{}
	q.Set("fields", "thumbnailLink")
	q.Set("key", c.apiKey)
	reqURL := apiBase + "/" + url.PathEscape(fileID) + "?" + q.Encode()

	body, err := c.getWithRetry(ctx, reqURL)
	if err != nil {
		return "", err
	}
	var rf rawFile
	if err := json.Unmarshal(body, &rf); err != nil {
		return "", err
	}
	if rf.ThumbnailLink == "" {
		return "", fmt.Errorf("sem thumbnail disponível")
	}
	return rf.ThumbnailLink, nil
}

// StreamDo performs a request meant to stream a (potentially large) body
// back to a client. It acquires the semaphore for the duration of the
// whole call and returns a release func the caller MUST invoke once done
// reading/copying the response body.
func (c *Client) StreamDo(req *http.Request) (*http.Response, func(), error) {
	c.sem.Acquire()
	resp, err := c.http.Do(req)
	if err != nil {
		c.sem.Release()
		return nil, nil, err
	}
	released := false
	release := func() {
		if !released {
			released = true
			c.sem.Release()
		}
	}
	return resp, release, nil
}
