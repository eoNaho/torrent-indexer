package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/felipemarinho97/torrent-indexer/logging"
	"github.com/felipemarinho97/torrent-indexer/magnet"
	"github.com/felipemarinho97/torrent-indexer/schema"
	goscrape "github.com/felipemarinho97/torrent-indexer/scrape"
	"github.com/felipemarinho97/torrent-indexer/utils"
)

var semtorrent = IndexerMeta{
	Label:       "semtorrent",
	URL:         utils.GetIndexerURLFromEnv("INDEXER_SEMTORRENT_URL", "https://semtorrent.com/"),
	SearchURL:   "?busca=",
	PagePattern: "pagina/%s/",
}

var semTorrentDatePublishedRegex = regexp.MustCompile(`"datePublished"\s*:\s*"([^"]+)"`)

func (i *Indexer) HandlerSemTorrentIndexer(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	metadata := semtorrent

	defer func() {
		i.metrics.IndexerDuration.WithLabelValues(metadata.Label).Observe(time.Since(start).Seconds())
		i.metrics.IndexerRequests.WithLabelValues(metadata.Label).Inc()
	}()

	ctx := r.Context()
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page := strings.TrimSpace(r.URL.Query().Get("page"))

	targetURL := buildSemTorrentURL(metadata, q, page)

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

	links := extractSemTorrentLinks(doc)
	if len(links) == 0 {
		_ = i.requester.ExpireDocument(ctx, targetURL)
	}

	indexedTorrents := utils.ParallelFlatMap(links, func(link string) ([]schema.IndexedTorrent, error) {
		return getTorrentsSemTorrent(ctx, i, link, targetURL)
	})

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

func buildSemTorrentURL(metadata IndexerMeta, q, page string) string {
	baseURL := strings.TrimRight(metadata.URL, "/") + "/"
	if q != "" {
		return fmt.Sprintf("%s%s%s", baseURL, metadata.SearchURL, url.QueryEscape(q))
	}
	if page != "" && page != "1" {
		return fmt.Sprintf("%s"+metadata.PagePattern, baseURL, url.PathEscape(page))
	}
	return baseURL
}

func extractSemTorrentLinks(doc *goquery.Document) []string {
	var links []string
	seen := make(map[string]struct{})
	doc.Find("a.media-card-link").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		href = strings.TrimSpace(href)
		if !exists || href == "" {
			return
		}
		if _, ok := seen[href]; ok {
			return
		}
		seen[href] = struct{}{}
		links = append(links, href)
	})
	return links
}

type semTorrentPostData struct {
	Title       string
	Year        string
	IMDB        string
	Audio       []schema.Audio
	Date        time.Time
	Sizes       []string
	MagnetLinks []string
}

func parseSemTorrentPost(doc *goquery.Document) semTorrentPostData {
	title := strings.TrimSpace(doc.Find("h1.item-title").First().Text())
	if title == "" {
		title = strings.TrimSpace(doc.Find("h1").First().Text())
	}

	date := getPublishedDateFromMeta(doc)
	if date.IsZero() {
		doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
			if match := semTorrentDatePublishedRegex.FindStringSubmatch(s.Text()); len(match) > 1 {
				if parsed, err := time.Parse(time.RFC3339, match[1]); err == nil {
					date = parsed
					return false
				}
			}
			return true
		})
	}

	var imdbLink string
	doc.Find(`a[href*="imdb.com"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if href, ok := s.Attr("href"); ok {
			if parsed, err := getIMDBLink(href); err == nil {
				imdbLink = parsed
				return false
			}
		}
		return true
	})

	var audio []schema.Audio
	var year string
	var sizes []string

	doc.Find("p, li").Each(func(_ int, s *goquery.Selection) {
		text := s.Text()
		audio = append(audio, findAudioFromText(text)...)
		if year == "" {
			year = findYearFromText(text, title)
		}
		sizes = append(sizes, findSizesFromText(text)...)
	})
	if year == "" {
		year = findYearFromText("", title)
	}
	audio = getAudioFromTitle(title, audio)
	audio = getAudioFromTitle(doc.Find(".download-name").Text(), audio)
	uniqueAudio := make([]schema.Audio, 0, len(audio))
	seenAudio := make(map[schema.Audio]struct{}, len(audio))
	for _, a := range audio {
		if _, ok := seenAudio[a]; ok {
			continue
		}
		seenAudio[a] = struct{}{}
		uniqueAudio = append(uniqueAudio, a)
	}

	var magnetLinks []string
	seenMagnets := make(map[string]struct{})
	doc.Find(`a[href^="magnet"]`).Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		href = strings.TrimSpace(href)
		if !ok || href == "" {
			return
		}
		if _, dup := seenMagnets[href]; dup {
			return
		}
		seenMagnets[href] = struct{}{}
		magnetLinks = append(magnetLinks, href)
	})

	return semTorrentPostData{
		Title:       title,
		Year:        year,
		IMDB:        imdbLink,
		Audio:       uniqueAudio,
		Date:        date,
		Sizes:       utils.StableUniq(sizes),
		MagnetLinks: magnetLinks,
	}
}

func getTorrentsSemTorrent(ctx context.Context, i *Indexer, link, referer string) ([]schema.IndexedTorrent, error) {
	doc, err := getDocument(ctx, i, link, referer)
	if err != nil {
		return nil, err
	}

	post := parseSemTorrentPost(doc)
	if len(post.MagnetLinks) == 0 {
		return nil, nil
	}

	var chanIndexedTorrent = make(chan schema.IndexedTorrent)
	for idx, magnetLink := range post.MagnetLinks {
		idx := idx
		magnetLink := magnetLink
		go func() {
			parsedMagnet, err := magnet.ParseMagnetUri(magnetLink)
			if err != nil {
				logging.Error().Err(err).Str("magnet_link", magnetLink).Msg("Failed to parse magnet URI")
				chanIndexedTorrent <- schema.IndexedTorrent{}
				return
			}

			releaseTitle := parsedMagnet.DisplayName
			infoHash := parsedMagnet.InfoHash.String()
			trackers := parsedMagnet.Trackers
			magnetAudio := getAudioFromTitle(releaseTitle, post.Audio)

			peer, seed, err := goscrape.GetLeechsAndSeeds(ctx, i.redis, i.metrics, infoHash, trackers)
			if err != nil {
				logging.Error().Err(err).Str("info_hash", infoHash).Msg("Failed to get leechers and seeders")
			}

			var mySize string
			if len(post.Sizes) == len(post.MagnetLinks) {
				mySize = post.Sizes[idx]
			}
			if mySize == "" {
				go func() {
					_, _ = i.magnetMetadataAPI.FetchMetadata(ctx, magnetLink)
				}()
			}

			chanIndexedTorrent <- schema.IndexedTorrent{
				Title:         releaseTitle,
				OriginalTitle: post.Title,
				Details:       link,
				Year:          post.Year,
				IMDB:          post.IMDB,
				Audio:         magnetAudio,
				MagnetLink:    magnetLink,
				Date:          post.Date,
				InfoHash:      infoHash,
				Trackers:      trackers,
				LeechCount:    peer,
				SeedCount:     seed,
				Size:          mySize,
			}
		}()
	}

	var indexedTorrents []schema.IndexedTorrent
	for range post.MagnetLinks {
		it := <-chanIndexedTorrent
		if it.InfoHash != "" {
			indexedTorrents = append(indexedTorrents, it)
		}
	}

	return indexedTorrents, nil
}
