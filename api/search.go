package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/felipemarinho97/torrent-indexer/schema"
	meilisearch "github.com/felipemarinho97/torrent-indexer/search"
	"github.com/felipemarinho97/torrent-indexer/tmdb"
)

// MeilisearchHandler handles HTTP requests for Meilisearch integration.
type MeilisearchHandler struct {
	Module   *meilisearch.SearchIndexer
	TMDB     *tmdb.Client
	Indexers *Indexer
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string                 `json:"status"`
	Service   string                 `json:"service"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp string                 `json:"timestamp"`
}

// StatsResponse represents the stats endpoint response
type StatsResponse struct {
	Status            string           `json:"status"`
	NumberOfDocuments int64            `json:"numberOfDocuments"`
	IsIndexing        bool             `json:"isIndexing"`
	FieldDistribution map[string]int64 `json:"fieldDistribution"`
	Service           string           `json:"service"`
}

// NewMeilisearchHandler creates a new instance of MeilisearchHandler.
func NewMeilisearchHandler(module *meilisearch.SearchIndexer, tmdbClient *tmdb.Client) *MeilisearchHandler {
	return &MeilisearchHandler{Module: module, TMDB: tmdbClient}
}

// SetIndexers configures the live indexers used as fallback when Meilisearch has no hits.
func (h *MeilisearchHandler) SetIndexers(idx *Indexer) {
	h.Indexers = idx
}

// SearchTorrentHandler handles the searching of torrent items.
func (h *MeilisearchHandler) SearchTorrentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	query := rawQuery
	if query == "" {
		query = time.Now().Format("2006-01-02") // needed for prowlar/jackett empty queries
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10 // Default limit
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			http.Error(w, "Invalid limit parameter", http.StatusBadRequest)
			return
		}
	}

	searchLimit := limit
	if hasSearchFilters(r) {
		searchLimit = limit * 5
		if searchLimit < 100 {
			searchLimit = 100
		}
	}

	liveParam := r.URL.Query().Get("live")
	var results []schema.IndexedTorrent
	var err error
	if liveParam != "1" && h.Module != nil {
		results, err = h.Module.SearchTorrent(query, searchLimit)
	}

	if (err != nil || len(results) == 0) && rawQuery != "" && liveParam != "0" && h.Indexers != nil {
		results = h.searchLiveIndexers(r.Context(), r, rawQuery)
		err = nil
	} else if err != nil {
		http.Error(w, "Failed to search torrents", http.StatusInternalServerError)
		return
	}

	for i := range results {
		if results[i].Season != 0 || results[i].Episode != 0 || results[i].ReleaseType != "" {
			continue
		}
		season, episode, releaseType := ParseSeasonEpisode(results[i].Title)
		if season == 0 && episode == 0 && releaseType == "" {
			season, episode, releaseType = ParseSeasonEpisode(results[i].OriginalTitle)
		}
		results[i].Season = season
		results[i].Episode = episode
		results[i].ReleaseType = releaseType
	}

	results = FilterBy(nil, r, results)
	if r.URL.Query().Get("sortBy") != "" {
		results = ApplySorting(nil, r, results)
	}
	if len(results) > limit {
		results = results[:limit]
	}
	if h.TMDB != nil && len(results) > 0 {
		needsEnrichment := false
		for _, result := range results {
			if result.TMDBID == 0 {
				needsEnrichment = true
				break
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		results = h.TMDB.Enrich(ctx, results)
		cancel()
		// Keep the enriched result in the search index for later requests.
		if needsEnrichment && h.Module != nil {
			updated := false
			for _, result := range results {
				if result.TMDBID != 0 {
					updated = true
					break
				}
			}
			if updated {
				snapshot := append([]schema.IndexedTorrent(nil), results...)
				go func() { _ = h.Module.IndexTorrents(snapshot) }()
			}
		}
	}

	// Format response to match indexers structure
	response := map[string]interface{}{
		"results": results,
		"count":   len(results),
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HealthHandler provides a health check endpoint for Meilisearch.
func (h *MeilisearchHandler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Check if Meilisearch is healthy
	isHealthy := h.Module.IsHealthy()

	response := HealthResponse{
		Service:   "meilisearch",
		Timestamp: getCurrentTimestamp(),
	}

	if isHealthy {
		// Try to get additional stats for more detailed health info
		stats, err := h.Module.GetStats()
		if err == nil {
			response.Status = "healthy"
			response.Details = map[string]interface{}{
				"documents": stats.NumberOfDocuments,
				"indexing":  stats.IsIndexing,
			}
			w.WriteHeader(http.StatusOK)
		} else {
			// Service is up but can't get stats
			response.Status = "degraded"
			response.Details = map[string]interface{}{
				"error": "Could not retrieve stats",
			}
			w.WriteHeader(http.StatusOK)
		}
	} else {
		// Service is down
		response.Status = "unhealthy"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// StatsHandler provides detailed statistics about the Meilisearch index.
func (h *MeilisearchHandler) StatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Get detailed stats from Meilisearch
	stats, err := h.Module.GetStats()
	if err != nil {
		// Check if it's a connectivity issue
		if !h.Module.IsHealthy() {
			http.Error(w, "Meilisearch service is unavailable", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "Failed to retrieve statistics", http.StatusInternalServerError)
		return
	}

	response := StatsResponse{
		Status:            "healthy",
		Service:           "meilisearch",
		NumberOfDocuments: stats.NumberOfDocuments,
		IsIndexing:        stats.IsIndexing,
		FieldDistribution: stats.FieldDistribution,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// getCurrentTimestamp returns the current timestamp in RFC3339 format
func getCurrentTimestamp() string {
	return time.Now().Format(time.RFC3339)
}

func hasSearchFilters(r *http.Request) bool {
	for _, key := range []string{"min_seeds", "season", "episode", "audio", "year", "imdb"} {
		if r.URL.Query().Get(key) != "" {
			return true
		}
	}
	return false
}

func (h *MeilisearchHandler) searchLiveIndexers(parentCtx context.Context, r *http.Request, query string) []schema.IndexedTorrent {
	type liveSource struct {
		name    string
		handler http.HandlerFunc
	}

	sources := []liveSource{
		{name: "catalogo-betor", handler: h.Indexers.HandlerCatalogoBetorIndexer},
		{name: "semtorrent", handler: h.Indexers.HandlerSemTorrentIndexer},
		{name: "comando_torrents", handler: h.Indexers.HandlerComandoIndexer},
		{name: "bludv", handler: h.Indexers.HandlerBluDVIndexer},
		{name: "rede_torrent", handler: h.Indexers.HandlerRedeTorrentIndexer},
		{name: "starck-filmes", handler: h.Indexers.HandlerStarckFilmesIndexer},
		{name: "torrent-dos-filmes", handler: h.Indexers.HandlerTorrentDosFilmesIndexer},
	}

	ctx, cancel := context.WithTimeout(parentCtx, 12*time.Second)
	defer cancel()

	var mu sync.Mutex
	var wg sync.WaitGroup
	byHash := make(map[string]schema.IndexedTorrent)
	var withoutHash []schema.IndexedTorrent

	for _, src := range sources {
		src := src
		wg.Add(1)
		go func() {
			defer wg.Done()

			targetURL := "/indexers/" + src.name + "?q=" + url.QueryEscape(query)
			if r != nil {
				params := url.Values{}
				params.Set("q", query)
				for _, key := range []string{"page", "audio", "year", "imdb", "min_seeds", "season", "episode"} {
					if v := r.URL.Query().Get(key); v != "" {
						params.Set(key, v)
					}
				}
				targetURL = "/indexers/" + src.name + "?" + params.Encode()
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
			if err != nil {
				return
			}
			rr := httptest.NewRecorder()
			src.handler(rr, req)
			if rr.Code != http.StatusOK {
				return
			}

			var resp Response
			if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
				return
			}

			mu.Lock()
			defer mu.Unlock()
			for _, item := range resp.Results {
				if item.Similarity <= 0 {
					continue
				}
				key := strings.ToLower(strings.TrimSpace(item.InfoHash))
				if key == "" {
					withoutHash = append(withoutHash, item)
					continue
				}
				existing, exists := byHash[key]
				if !exists || item.SeedCount > existing.SeedCount || (existing.Size == "" && item.Size != "") {
					byHash[key] = item
				}
			}
		}()
	}

	wg.Wait()

	merged := make([]schema.IndexedTorrent, 0, len(byHash)+len(withoutHash))
	for _, item := range byHash {
		merged = append(merged, item)
	}
	merged = append(merged, withoutHash...)

	slices.SortFunc(merged, func(a, b schema.IndexedTorrent) int {
		if a.Similarity != b.Similarity {
			if b.Similarity > a.Similarity {
				return 1
			}
			return -1
		}
		return b.SeedCount - a.SeedCount
	})

	return merged
}
