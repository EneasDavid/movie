package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"movie/internal/appctx"
)

type healthStatus struct {
	OK              bool   `json:"ok"`
	DriveConfigured bool   `json:"driveConfigured"`
	RedisConfigured bool   `json:"redisConfigured"`
	RedisReachable  bool   `json:"redisReachable"`
	Version         string `json:"version"`
}

// Health serves GET /api/health — a plain, unauthenticated liveness/config
// check. It reports *whether* things are configured, never secret values,
// so it's safe to leave public and point uptime monitoring at.
func Health(w http.ResponseWriter, r *http.Request) {
	a := appctx.Get()

	status := healthStatus{
		DriveConfigured: a.Config.DriveAPIKey != "" && a.Config.DriveFolderID != "",
		RedisConfigured: a.Redis.Enabled(),
		Version:         "1.0.0",
	}

	if status.RedisConfigured {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		probeKey := "health:probe"
		if err := a.Redis.SetString(ctx, probeKey, "1", 30*time.Second); err == nil {
			_, status.RedisReachable = a.Redis.GetString(ctx, probeKey)
		}
	}

	status.OK = status.DriveConfigured && (status.RedisConfigured == false || status.RedisReachable)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if !status.OK {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(status)
}
