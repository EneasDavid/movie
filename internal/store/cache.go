package store

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
)

const (
	keyCatalog      = "catalog:v1"
	keyThumbnailPfx = "thumb:v1:"
	keyChunkPfx     = "chunk:v1:"
	keyMetaPfx      = "meta:v1:"
)

// Catalog: single JSON blob cached as a whole. Cheap to store/fetch and
// avoids N+1 Redis round-trips on every page load.
func (r *Redis) GetCatalog(ctx context.Context, out any) bool {
	s, ok := r.GetString(ctx, keyCatalog)
	if !ok {
		return false
	}
	return json.Unmarshal([]byte(s), out) == nil
}

func (r *Redis) SetCatalog(ctx context.Context, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.SetString(ctx, keyCatalog, string(data), ttl)
}

// Thumbnails: small images, cached as raw bytes.
func (r *Redis) GetThumbnail(ctx context.Context, fileID string) ([]byte, bool) {
	return r.GetBytes(ctx, keyThumbnailPfx+fileID)
}

func (r *Redis) SetThumbnail(ctx context.Context, fileID string, data []byte, ttl time.Duration) error {
	return r.SetBytes(ctx, keyThumbnailPfx+fileID, data, ttl)
}

// Video chunks: keyed by file + chunk index, so repeated plays/seeks of
// the same range serve straight from Redis instead of re-hitting Drive.
func chunkKey(fileID string, index int64) string {
	return keyChunkPfx + fileID + ":" + strconv.FormatInt(index, 10)
}

func (r *Redis) GetChunk(ctx context.Context, fileID string, index int64) ([]byte, bool) {
	return r.GetBytes(ctx, chunkKey(fileID, index))
}

func (r *Redis) SetChunk(ctx context.Context, fileID string, index int64, data []byte, ttl time.Duration) error {
	return r.SetBytes(ctx, chunkKey(fileID, index), data, ttl)
}

// File metadata (mimeType/size): fetched once from Drive, then cached so
// the many range requests a single playback session generates don't each
// pay for a metadata round-trip.
func (r *Redis) GetMeta(ctx context.Context, fileID string, out any) bool {
	s, ok := r.GetString(ctx, keyMetaPfx+fileID)
	if !ok {
		return false
	}
	return json.Unmarshal([]byte(s), out) == nil
}

func (r *Redis) SetMeta(ctx context.Context, fileID string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.SetString(ctx, keyMetaPfx+fileID, string(data), ttl)
}
