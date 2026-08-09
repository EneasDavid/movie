package drive

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// FetchRange downloads an exact byte range of a file into memory. Used by
// the streaming proxy's chunk-cache path — callers are expected to pass
// small, bounded ranges (one cache chunk at a time), never a whole video.
func (c *Client) FetchRange(ctx context.Context, fileID string, start, end int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.MediaURL(fileID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	c.sem.Acquire()
	defer c.sem.Release()

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return nil, fmt.Errorf("drive media status %d: %s", resp.StatusCode, truncate(string(data), 300))
	}
	return io.ReadAll(resp.Body)
}

// OpenStream issues a request for the media of a file, forwarding the
// given Range header verbatim (or none, for the whole file). It returns
// the upstream response for the caller to copy directly to the client —
// used for the pass-through path (large/unaligned ranges we don't want to
// pull through the chunk cache). The caller MUST call the returned release
// func once done reading the body.
func (c *Client) OpenStream(ctx context.Context, fileID, rangeHeader string) (*http.Response, func(), error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.MediaURL(fileID), nil)
	if err != nil {
		return nil, nil, err
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	return c.StreamDo(req)
}
