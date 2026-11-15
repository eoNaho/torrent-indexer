package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/felipemarinho97/torrent-indexer/cache"
	"github.com/felipemarinho97/torrent-indexer/monitoring"
	meilisearch "github.com/felipemarinho97/torrent-indexer/search"
)

type operationDependency struct {
	Status string `json:"status"`
}

// HandlerOperations returns an operational snapshot for the built-in dashboard.
func HandlerOperations(operations *monitoring.Operations, redis *cache.Redis, search *meilisearch.SearchIndexer, tmdbEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		redisStatus := "healthy"
		if err := redis.Ping(ctx); err != nil { redisStatus = "unhealthy" }
		meiliStatus := "not_configured"
		if search != nil && search.BaseURL != "" {
			meiliStatus = "healthy"
			if !search.IsHealthy() { meiliStatus = "unhealthy" }
		}
		indexers, uptime := operations.Snapshot()
		status := "healthy"
		if redisStatus == "unhealthy" || meiliStatus == "unhealthy" { status = "degraded" }
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": status,
			"uptime": uptime.String(),
			"dependencies": map[string]any{
				"redis": operationDependency{Status: redisStatus},
				"meilisearch": operationDependency{Status: meiliStatus},
				"tmdb": operationDependency{Status: map[bool]string{true: "enabled", false: "disabled"}[tmdbEnabled]},
			},
			"indexers": indexers,
		})
	}
}
