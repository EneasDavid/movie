package store

import (
	"context"
	"encoding/json"
	"time"
)

const (
	keyProgressPfx      = "progress:"       // + userID + ":" + fileID -> ProgressEntry JSON
	keyProgressIndexPfx = "progress:index:" // + userID -> sorted set (score=updatedAt, member=fileID)
	maxProgressEntries  = 50
)

type ProgressEntry struct {
	FileID    string  `json:"fileId"`
	Time      float64 `json:"time"`
	Duration  float64 `json:"duration"`
	Title     string  `json:"title,omitempty"`
	UpdatedAt int64   `json:"updatedAt"`
}

// SetProgress is the server-side replacement for the old localStorage
// write: upserts the entry and its position in the per-user recency
// index in one call. Idempotent — writing the same {time, duration} again
// just overwrites the same keys with equivalent data, safe to call from a
// throttled timeupdate handler same as before.
func (r *Redis) SetProgress(ctx context.Context, userID, fileID string, timeSec, duration float64, title string) error {
	entry := ProgressEntry{
		FileID:    fileID,
		Time:      timeSec,
		Duration:  duration,
		Title:     title,
		UpdatedAt: time.Now().Unix(),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := r.SetString(ctx, keyProgressPfx+userID+":"+fileID, string(data), noExpiry); err != nil {
		return err
	}
	_, err = r.command(ctx, "ZADD", keyProgressIndexPfx+userID, entry.UpdatedAt, fileID)
	return err
}

// ListProgress returns the user's most recently updated entries, newest
// first — the "continuar assistindo" row's data source.
func (r *Redis) ListProgress(ctx context.Context, userID string) ([]ProgressEntry, error) {
	res, err := r.command(ctx, "ZREVRANGE", keyProgressIndexPfx+userID, 0, maxProgressEntries-1)
	if err != nil {
		return nil, err
	}
	var fileIDs []string
	if err := json.Unmarshal(res, &fileIDs); err != nil {
		return nil, err
	}

	entries := make([]ProgressEntry, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		s, ok := r.GetString(ctx, keyProgressPfx+userID+":"+fileID)
		if !ok {
			continue // index and data drifted (e.g. manual deletion) — skip, not fatal
		}
		var entry ProgressEntry
		if err := json.Unmarshal([]byte(s), &entry); err == nil {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

// DeleteProgress removes a single entry — "remover do continuar
// assistindo". Removes both the data key and its index entry so it can
// never resurface in ListProgress.
func (r *Redis) DeleteProgress(ctx context.Context, userID, fileID string) error {
	if _, err := r.command(ctx, "DEL", keyProgressPfx+userID+":"+fileID); err != nil {
		return err
	}
	_, err := r.command(ctx, "ZREM", keyProgressIndexPfx+userID, fileID)
	return err
}
