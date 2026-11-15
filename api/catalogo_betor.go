package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/felipemarinho97/torrent-indexer/logging"
	"github.com/felipemarinho97/torrent-indexer/magnet"
	"github.com/felipemarinho97/torrent-indexer/schema"
	"github.com/felipemarinho97/torrent-indexer/utils"
)

var catalogo_betor = IndexerMeta{
	Label:       "catalogo_betor",
	URL:         utils.GetIndexerURLFromEnv("INDEXER_CATALOGO_BETOR_URL", "https://catalogo.betor.top/"),
	SearchURL:   "search/?q=",
	PagePattern: "filmes-e-series/%s/",
}

var (
	imdbIDRegex      = regexp.MustCompile(`^tt\d+$`)
	releaseYearRegex = regexp.MustCompile(`\b((?:19|20)\d{2})\b`)
)

func (i *Indexer) HandlerCatalogoBetorIndexer(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	metadata := catalogo_betor

	defer func() {
		i.metrics.IndexerDuration.WithLabelValues(metadata.Label).Observe(time.Since(start).Seconds())
		i.metrics.IndexerRequests.WithLabelValues(metadata.Label).Inc()
	}()

	ctx := r.Context()
	query := r.URL.Query()
	q := strings.TrimSpace(query.Get("q"))
	page := strings.TrimSpace(query.Get("page"))
	imdbID := strings.TrimSpace(query.Get("imdb"))
	if imdbID == "" {
		imdbID = strings.TrimSpace(query.Get("imdbid"))
	}
	season := strings.TrimSpace(query.Get("season"))

	targetURL := buildCatalogoBetorURL(metadata, q, imdbID, season, page)

	logging.InfoWithRequest(r).Str("target_url", targetURL).Msg("Processing indexer request")
	resp, err := i.requester.GetDocument(ctx, targetURL)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); encErr != nil {
			logging.ErrorWithRequest(r).Err(encErr).Msg("Failed to encode error response")
		}
		i.metrics.IndexerErrors.WithLabelValues(metadata.Label).Inc()
		return
	}
	defer resp.Close()

	doc, err := goquery.NewDocumentFromReader(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); encErr != nil {
			logging.ErrorWithRequest(r).Err(encErr).Msg("Failed to encode error response")
		}
		i.metrics.IndexerErrors.WithLabelValues(metadata.Label).Inc()
		return
	}

	indexedTorrents := parseCatalogoBetorDocument(doc)
	if len(indexedTorrents) == 0 {
		_ = i.requester.ExpireDocument(ctx, targetURL)
	}

	postProcessedTorrents := indexedTorrents
	for _, processor := range i.postProcessors {
		postProcessedTorrents = processor(i, r, postProcessedTorrents)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(Response{
		Results:      postProcessedTorrents,
		Count:        len(postProcessedTorrents),
		IndexedCount: len(indexedTorrents),
	}); err != nil {
		logging.Error().Err(err).Msg("Failed to encode response")
	}
}

func buildCatalogoBetorURL(metadata IndexerMeta, q, imdbID, season, page string) string {
	baseURL := strings.TrimRight(metadata.URL, "/") + "/"

	if imdbID == "" && imdbIDRegex.MatchString(q) {
		imdbID = q
	}

	if imdbID != "" {
		if season != "" {
			return fmt.Sprintf("%simdb/%s/season/%s/", baseURL, url.PathEscape(imdbID), url.PathEscape(season))
		}
		return fmt.Sprintf("%simdb/%s/", baseURL, url.PathEscape(imdbID))
	}

	if q != "" {
		return fmt.Sprintf("%s%s%s", baseURL, metadata.SearchURL, url.QueryEscape(q))
	}

	if page != "" && page != "1" {
		return fmt.Sprintf("%s"+metadata.PagePattern, baseURL, url.PathEscape(page))
	}

	return fmt.Sprintf("%sfilmes-e-series/", baseURL)
}

func parseCatalogoBetorDocument(doc *goquery.Document) []schema.IndexedTorrent {
	var torrents []schema.IndexedTorrent

	doc.Find("[data-torrent]").Each(func(_ int, s *goquery.Selection) {
		magnetLink := strings.TrimSpace(s.AttrOr("data-torrent-magnet-uri", ""))
		if magnetLink == "" {
			return
		}

		name := strings.TrimSpace(s.AttrOr("data-torrent-name", ""))
		var infoHash string
		var trackers []string
		if parsedMagnet, err := magnet.ParseMagnetUri(magnetLink); err == nil {
			infoHash = parsedMagnet.InfoHash.String()
			trackers = parsedMagnet.Trackers
			if name == "" {
				name = parsedMagnet.DisplayName
			}
		} else {
			logging.Error().Err(err).Str("magnet_link", magnetLink).Msg("Failed to parse magnet URI")
		}

		var size string
		if rawSize := strings.TrimSpace(s.AttrOr("data-torrent-size", "")); rawSize != "" {
			if sizeBytes, err := strconv.ParseInt(rawSize, 10, 64); err == nil && sizeBytes > 0 {
				size = utils.FormatBytes(sizeBytes)
			}
		}

		seedCount, _ := strconv.Atoi(strings.TrimSpace(s.AttrOr("data-torrent-num-seeds", "")))
		leechCount, _ := strconv.Atoi(strings.TrimSpace(s.AttrOr("data-torrent-num-peers", "")))

		audio := parseCatalogoBetorLanguages(s.AttrOr("data-torrent-languages", ""))
		audio = getAudioFromTitle(name, audio)

		torrents = append(torrents, schema.IndexedTorrent{
			Title:         name,
			OriginalTitle: name,
			Details:       strings.TrimSpace(s.AttrOr("data-provider-url", "")),
			Year:          extractYearFromReleaseName(name),
			IMDB:          strings.TrimSpace(s.AttrOr("data-item-imdb-id", "")),
			Audio:         audio,
			MagnetLink:    magnetLink,
			Date:          parseCatalogoBetorDate(s.AttrOr("data-torrent-inserted-at", "")),
			InfoHash:      infoHash,
			Trackers:      trackers,
			Size:          size,
			Files:         parseCatalogoBetorFiles(s.AttrOr("data-torrent-files", "")),
			LeechCount:    leechCount,
			SeedCount:     seedCount,
		})
	})

	return torrents
}

func parseCatalogoBetorLanguages(raw string) []schema.Audio {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var audio []schema.Audio
	seen := make(map[schema.Audio]struct{})
	addAudio := func(a schema.Audio) {
		if _, ok := seen[a]; ok {
			return
		}
		seen[a] = struct{}{}
		audio = append(audio, a)
	}

	for _, part := range strings.Split(raw, ",") {
		lang := strings.ToLower(strings.TrimSpace(part))
		if lang == "" {
			continue
		}
		switch lang {
		case "pt", "pt-br", "br", "por":
			addAudio(schema.AudioPortuguese)
		case "en", "en-us", "en-gb", "eng":
			addAudio(schema.AudioEnglish)
		case "es", "spa":
			addAudio(schema.AudioSpanish)
		case "fr", "fra", "fre":
			addAudio(schema.AudioFrench)
		case "de", "deu", "ger":
			addAudio(schema.AudioGerman)
		case "it", "ita":
			addAudio(schema.AudioItalian)
		case "ja", "jp", "jpn":
			addAudio(schema.AudioJapanese)
		case "ko", "kor":
			addAudio(schema.AudioKorean)
		case "zh", "chi", "zho":
			addAudio(schema.AudioMandarin)
		case "ru", "rus":
			addAudio(schema.AudioRussian)
		default:
			if mapped := schema.GetAudioFromString(lang); mapped != nil {
				addAudio(*mapped)
			}
		}
	}

	return audio
}

func parseCatalogoBetorFiles(raw string) []schema.File {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var files []schema.File
	for _, entry := range strings.Split(raw, ";") {
		path := strings.TrimSpace(entry)
		if path == "" {
			continue
		}
		files = append(files, schema.File{Path: path})
	}
	return files
}

func extractYearFromReleaseName(name string) string {
	if year := findYearFromText("", name); year != "" {
		return year
	}
	matches := releaseYearRegex.FindAllStringSubmatch(name, -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1][1]
}

func parseCatalogoBetorDate(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return t
		}
	}
	return time.Time{}
}
