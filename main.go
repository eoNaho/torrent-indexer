package main

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	handler "github.com/felipemarinho97/torrent-indexer/api"
	"github.com/felipemarinho97/torrent-indexer/cache"
	"github.com/felipemarinho97/torrent-indexer/logging"
	"github.com/felipemarinho97/torrent-indexer/magnet"
	"github.com/felipemarinho97/torrent-indexer/monitoring"
	"github.com/felipemarinho97/torrent-indexer/public"
	"github.com/felipemarinho97/torrent-indexer/requester"
	meilisearch "github.com/felipemarinho97/torrent-indexer/search"
	"github.com/felipemarinho97/torrent-indexer/tmdb"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	str2duration "github.com/xhit/go-str2duration/v2"
)

func main() {
	// Initialize logging first
	logging.InitLogger()

	redis := cache.NewRedis()
	searchIndex := meilisearch.NewSearchIndexer(os.Getenv("MEILISEARCH_ADDRESS"), os.Getenv("MEILISEARCH_KEY"), "torrents")
	var magnetMetadataAPI *magnet.MetadataClient
	if os.Getenv("MAGNET_METADATA_API_ENABLED") == "true" {
		timeout := 10 * time.Second
		if v := os.Getenv("MAGNET_METADATA_API_TIMEOUT_SECONDS"); v != "" {
			if t, err := strconv.Atoi(v); err == nil {
				timeout = time.Duration(t) * time.Second
			}
		}
		magnetMetadataAPI = magnet.NewClient(os.Getenv("MAGNET_METADATA_API_ADDRESS"), timeout, redis)
	}
	metrics := monitoring.NewMetrics()
	metrics.Register()

	timeoutFlaresolverrMilli := 30000
	if v := os.Getenv("FLARESOLVERR_TIMEOUT_SECONDS"); v != "" {
		if t, err := strconv.Atoi(v); err == nil {
			timeoutFlaresolverrMilli = t * 1000
		}
	}

	flaresolverrPoolSize := 5
	if v := os.Getenv("FLARESOLVERR_POOL_SIZE"); v != "" {
		if t, err := strconv.Atoi(v); err == nil {
			flaresolverrPoolSize = t
		}
	}

	flaresolverr := requester.NewFlareSolverr(os.Getenv("FLARESOLVERR_ADDRESS"), timeoutFlaresolverrMilli, flaresolverrPoolSize)

	timeoutRequester := 5000 * time.Millisecond
	if v := os.Getenv("REQUEST_TIMEOUT_MILLISECONDS"); v != "" {
		if t, err := strconv.Atoi(v); err == nil {
			timeoutRequester = time.Duration(t) * time.Millisecond
		}
	}
	req := requester.NewRequester(flaresolverr, redis, timeoutRequester)

	// get shot-lived and long-lived cache expiration from env
	shortLivedCacheExpiration, err := str2duration.ParseDuration(os.Getenv("SHORT_LIVED_CACHE_EXPIRATION"))
	if err == nil {
		logging.Info().Dur("expiration", shortLivedCacheExpiration).Msg("Setting short-lived cache expiration")
		req.SetShortLivedCacheExpiration(shortLivedCacheExpiration)
	}
	if v := os.Getenv("LONG_LIVED_CACHE_EXPIRATION"); v != "" {
		longLivedCacheExpiration, err := str2duration.ParseDuration(v)
		if err == nil {
			logging.Info().Dur("expiration", longLivedCacheExpiration).Msg("Setting long-lived cache expiration")
			redis.SetDefaultExpiration(longLivedCacheExpiration)
		} else {
			logging.Error().Err(err).Msg("Failed to parse long-lived cache expiration")
		}
	}

	icfg := handler.IndexersConfig{
		FallbackTitleEnabled: os.Getenv("FALLBACK_TITLE_ENABLED") == "true",
	}

	tmdbClient := tmdb.NewClient(os.Getenv("TMDB_API_KEY"), redis)
	indexers := handler.NewIndexers(icfg, redis, metrics, req, searchIndex, magnetMetadataAPI, tmdbClient)
	search := handler.NewMeilisearchHandler(searchIndex, tmdbClient)
	search.SetIndexers(indexers)
	operations := monitoring.NewOperations("bludv", "comando", "rede_torrent", "starck-filmes", "torrent-dos-filmes", "vaca_torrent", "catalogo-betor", "semtorrent", "manual", "torznab")

	indexerMux := http.NewServeMux()
	metricsMux := http.NewServeMux()

	indexerMux.HandleFunc("/", handler.HandlerIndex)
	indexerMux.Handle("/indexers/bludv", operations.Wrap("bludv", http.HandlerFunc(indexers.HandlerBluDVIndexer)))
	indexerMux.Handle("/indexers/comando_torrents", operations.Wrap("comando", http.HandlerFunc(indexers.HandlerComandoIndexer)))
	indexerMux.Handle("/indexers/rede_torrent", operations.Wrap("rede_torrent", http.HandlerFunc(indexers.HandlerRedeTorrentIndexer)))
	indexerMux.Handle("/indexers/starck-filmes", operations.Wrap("starck-filmes", http.HandlerFunc(indexers.HandlerStarckFilmesIndexer)))
	indexerMux.Handle("/indexers/torrent-dos-filmes", operations.Wrap("torrent-dos-filmes", http.HandlerFunc(indexers.HandlerTorrentDosFilmesIndexer)))
	indexerMux.Handle("/indexers/vaca_torrent", operations.Wrap("vaca_torrent", http.HandlerFunc(indexers.HandlerVacaTorrentIndexer)))
	indexerMux.Handle("/indexers/catalogo-betor", operations.Wrap("catalogo-betor", http.HandlerFunc(indexers.HandlerCatalogoBetorIndexer)))
	indexerMux.Handle("/indexers/semtorrent", operations.Wrap("semtorrent", http.HandlerFunc(indexers.HandlerSemTorrentIndexer)))
	indexerMux.Handle("/indexers/manual", operations.Wrap("manual", http.HandlerFunc(indexers.HandlerManualIndexer)))
	indexerMux.HandleFunc("/ops", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/ui/ops.html", http.StatusTemporaryRedirect) })
	indexerMux.HandleFunc("/ops/status", handler.HandlerOperations(operations, redis, searchIndex, tmdbClient != nil))
	torznabHandler := http.HandlerFunc(indexers.HandlerTorznabAPI)
	torznabOperations := operations.WrapDynamic(func(r *http.Request) string {
		name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api"), "/")
		if name == "" { name = r.URL.Query().Get("indexer") }
		if name == "" { name = "torznab" }
		if name == "comando_torrents" { return "comando" }
		return name
	}, torznabHandler)
	indexerMux.Handle("/api", torznabOperations)
	indexerMux.Handle("/api/", torznabOperations)
	indexerMux.HandleFunc("/healthz", handler.HandlerHealth)
	indexerMux.HandleFunc("/search", search.SearchTorrentHandler)
	indexerMux.HandleFunc("/search/health", search.HealthHandler)
	indexerMux.HandleFunc("/search/stats", search.StatsHandler)
	indexerMux.HandleFunc("/domains", indexers.HandlerDomains)
	uiHandler := http.StripPrefix("/ui/", http.FileServer(http.FS(public.UIFiles)))
	indexerMux.Handle("/ui/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		uiHandler.ServeHTTP(w, r)
	}))

	loggedIndexerMux := logging.HTTPLoggingMiddleware(indexerMux)

	metricsMux.Handle("/metrics", promhttp.Handler())
	host := os.Getenv("HOST")
	if host == "" {
		host = "127.0.0.1"
	}

	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9081"
	}

	go func() {
		err := http.ListenAndServe(net.JoinHostPort(host, metricsPort), metricsMux)
		if err != nil {
			panic(err)
		}
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "7006"
	}

	logging.Info().Str("port", port).Msg("HTTP server listening (press Ctrl+C to stop)")
	err = http.ListenAndServe(net.JoinHostPort(host, port), loggedIndexerMux)
	if err != nil {
		logging.Fatal().Err(err).Msg("Server failed to start")
	}
}
