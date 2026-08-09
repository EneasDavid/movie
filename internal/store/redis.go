// Package store implements all cross-invocation state the app needs —
// catalog cache, thumbnail cache, video chunk cache, and rate limiting —
// on top of Upstash Redis's REST API.
//
// Why REST instead of a normal Redis TCP client: Vercel Go functions are
// stateless and short-lived; a pooled TCP connection either gets closed
// between invocations or leaks. Upstash's REST API is plain HTTPS, so it
// behaves correctly under that model with no connection management, and
// it's fast enough (single low-latency edge round-trip) for our sizes.
package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Redis struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewRedis(baseURL, token string) *Redis {
	return &Redis{
		baseURL: baseURL,
		token:   token,
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Enabled reports whether Redis is configured at all. Callers use this to
// degrade gracefully (skip caching / rate limiting) instead of failing
// hard when the env vars haven't been set yet, e.g. during initial setup.
func (r *Redis) Enabled() bool {
	return r != nil && r.baseURL != "" && r.token != ""
}

// command issues a single Upstash REST command. See
// https://upstash.com/docs/redis/features/restapi for the calling
// convention: POST /<CMD>/<arg1>/<arg2>/... with the args URL-escaped as
// path segments, or POST / with a JSON array body — we use the JSON body
// form since some of our values (video bytes, base64) contain characters
// that are awkward in URL paths.
func (r *Redis) command(ctx context.Context, args ...any) (json.RawMessage, error) {
	body, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("redis command failed (%d): %s", resp.StatusCode, string(data))
	}

	var out struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode redis response: %w", err)
	}
	if out.Error != "" {
		return nil, fmt.Errorf("redis error: %s", out.Error)
	}
	return out.Result, nil
}

// GetString returns a cached string value, or ("", false) on miss.
func (r *Redis) GetString(ctx context.Context, key string) (string, bool) {
	res, err := r.command(ctx, "GET", key)
	if err != nil || res == nil || string(res) == "null" {
		return "", false
	}
	var s string
	if err := json.Unmarshal(res, &s); err != nil {
		return "", false
	}
	return s, true
}

// SetString stores a string value with a TTL.
func (r *Redis) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	_, err := r.command(ctx, "SET", key, value, "EX", int(ttl.Seconds()))
	return err
}

// GetBytes/SetBytes store binary data base64-encoded (JSON-safe transport).
func (r *Redis) GetBytes(ctx context.Context, key string) ([]byte, bool) {
	s, ok := r.GetString(ctx, key)
	if !ok {
		return nil, false
	}
	data, err := decodeB64(s)
	if err != nil {
		return nil, false
	}
	return data, true
}

func (r *Redis) SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return r.SetString(ctx, key, encodeB64(value), ttl)
}

// Incr increments a counter and sets its TTL only on the first increment
// within a window (classic fixed-window rate limit). Returns the new
// count.
func (r *Redis) IncrWithExpiry(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	res, err := r.command(ctx, "INCR", key)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := json.Unmarshal(res, &count); err != nil {
		return 0, err
	}
	if count == 1 {
		// First hit in this window: arm expiry so the key resets.
		_, _ = r.command(ctx, "EXPIRE", key, int(ttl.Seconds()))
	}
	return count, nil
}
