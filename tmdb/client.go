package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/felipemarinho97/torrent-indexer/schema"
)

const apiBase = "https://api.themoviedb.org/3"

type Cache interface {
	Get(context.Context, string) ([]byte, error)
	SetWithExpiration(context.Context, string, []byte, time.Duration) error
}

type Client struct {
	apiKey string
	cache  Cache
	http   *http.Client
}

type metadata struct {
	ID          int      `json:"id"`
	Type        string   `json:"type"`
	URL         string   `json:"url"`
	Overview   string   `json:"overview"`
	Poster      string   `json:"poster"`
	Backdrop    string   `json:"backdrop"`
	Genres      []string `json:"genres"`
	Runtime     int      `json:"runtime"`
	Rating      float64  `json:"rating"`
	ReleaseDate string   `json:"release_date"`
	NotFound    bool     `json:"not_found,omitempty"`
}

type movieResult struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	OriginalName string  `json:"name"`
	ReleaseDate  string  `json:"release_date"`
	FirstAirDate string  `json:"first_air_date"`
	MediaType    string  `json:"media_type"`
}

type findResponse struct {
	Movies []movieResult `json:"movie_results"`
	Shows  []movieResult `json:"tv_results"`
}

type searchResponse struct {
	Results []movieResult `json:"results"`
}

type detailResponse struct {
	ID             int `json:"id"`
	Overview       string `json:"overview"`
	PosterPath     string `json:"poster_path"`
	BackdropPath   string `json:"backdrop_path"`
	VoteAverage    float64 `json:"vote_average"`
	ReleaseDate    string `json:"release_date"`
	FirstAirDate   string `json:"first_air_date"`
	Runtime        int `json:"runtime"`
	EpisodeRuntime []int `json:"episode_run_time"`
	Genres         []struct { Name string `json:"name"` } `json:"genres"`
}

var imdbIDPattern = regexp.MustCompile(`tt\d+`)

func NewClient(apiKey string, cache Cache) *Client {
	if strings.TrimSpace(apiKey) == "" {
		return nil
	}
	return &Client{
		apiKey: strings.TrimSpace(apiKey),
		cache:  cache,
		http:   &http.Client{Timeout: 5 * time.Second},
	}
}

// Enrich fills missing TMDB fields. Work is bounded so a large scrape cannot
// fan out an unbounded number of API requests.
func (c *Client) Enrich(ctx context.Context, torrents []schema.IndexedTorrent) []schema.IndexedTorrent {
	if c == nil || len(torrents) == 0 {
		return torrents
	}
	workers := 4
	if len(torrents) < workers { workers = len(torrents) }
	jobs := make(chan int)
	var wg sync.WaitGroup
	for n := 0; n < workers; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				if ctx.Err() != nil { continue }
				item := &torrents[index]
				if item.TMDBID != 0 { continue }
				lookupCtx, cancel := context.WithTimeout(ctx, 7*time.Second)
				info, err := c.lookup(lookupCtx, *item)
				cancel()
				if err == nil && info != nil && !info.NotFound {
					item.TMDBID = info.ID
					item.TMDBType = info.Type
					item.TMDBURL = info.URL
					item.TMDBOverview = info.Overview
					item.TMDBPoster = info.Poster
					item.TMDBBackdrop = info.Backdrop
					item.TMDBGenres = info.Genres
					item.TMDBRuntime = info.Runtime
					item.TMDBRating = info.Rating
					item.TMDBRelease = info.ReleaseDate
				}
			}
		}()
	}
	for index := range torrents { jobs <- index }
	close(jobs)
	wg.Wait()
	return torrents
}

func (c *Client) lookup(ctx context.Context, item schema.IndexedTorrent) (*metadata, error) {
	key, imdbID := cacheKey(item)
	if c.cache != nil {
		if raw, err := c.cache.Get(ctx, key); err == nil {
			var value metadata
			if json.Unmarshal(raw, &value) == nil { return &value, nil }
		}
	}

	var found movieResult
	if imdbID != "" {
		values := url.Values{"external_source": {"imdb_id"}, "api_key": {c.apiKey}}
		var response findResponse
		if err := c.get(ctx, "/find/"+url.PathEscape(imdbID), values, &response); err != nil { return nil, err }
		if len(response.Movies) > 0 {
			found = response.Movies[0]
			found.MediaType = "movie"
		} else if len(response.Shows) > 0 {
			found = response.Shows[0]
			found.MediaType = "tv"
		}
	}
	if found.ID == 0 {
		query := strings.TrimSpace(item.OriginalTitle)
		if query == "" { query = strings.TrimSpace(item.Title) }
		if query == "" { return nil, nil }
		mediaType := "movie"
		if item.Season > 0 || item.Episode > 0 { mediaType = "tv" }
		values := url.Values{"query": {query}, "language": {"pt-BR"}, "api_key": {c.apiKey}}
		if year, err := strconv.Atoi(item.Year); err == nil && year >= 1888 && year <= time.Now().Year()+3 {
			if mediaType == "tv" { values.Set("first_air_date_year", strconv.Itoa(year)) } else { values.Set("year", strconv.Itoa(year)) }
		}
		var response searchResponse
		if err := c.get(ctx, "/search/"+mediaType, values, &response); err != nil { return nil, err }
		found = chooseResult(response.Results, query, item.Year, mediaType)
		if found.ID == 0 { return c.cacheResult(ctx, key, &metadata{NotFound: true}, 6*time.Hour) }
		found.MediaType = mediaType
	}
	mediaType := found.MediaType
	if mediaType == "" { mediaType = "movie" }
	var details detailResponse
	if err := c.get(ctx, "/"+mediaType+"/"+strconv.Itoa(found.ID), url.Values{"language": {"pt-BR"}, "api_key": {c.apiKey}}, &details); err != nil { return nil, err }
	info := &metadata{
		ID: details.ID, Type: mediaType,
		URL: "https://www.themoviedb.org/"+mediaType+"/"+strconv.Itoa(details.ID),
		Overview: details.Overview, Rating: details.VoteAverage,
		ReleaseDate: details.ReleaseDate,
	}
	if info.ReleaseDate == "" { info.ReleaseDate = details.FirstAirDate }
	if details.PosterPath != "" { info.Poster = "https://image.tmdb.org/t/p/w500"+details.PosterPath }
	if details.BackdropPath != "" { info.Backdrop = "https://image.tmdb.org/t/p/w1280"+details.BackdropPath }
	info.Runtime = details.Runtime
	if info.Runtime == 0 && len(details.EpisodeRuntime) > 0 { info.Runtime = details.EpisodeRuntime[0] }
	for _, genre := range details.Genres { info.Genres = append(info.Genres, genre.Name) }
	if info.ID == 0 { info.ID = found.ID }
	_, err := c.cacheResult(ctx, key, info, 14*24*time.Hour)
	return info, err
}

func (c *Client) get(ctx context.Context, path string, query url.Values, target any) error {
	requestURL := apiBase+path+"?"+query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil { return err }
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return fmt.Errorf("tmdb returned status %d", resp.StatusCode) }
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 2<<20))
	return decoder.Decode(target)
}

func (c *Client) cacheResult(ctx context.Context, key string, value *metadata, ttl time.Duration) (*metadata, error) {
	if c.cache != nil {
		if raw, err := json.Marshal(value); err == nil { _ = c.cache.SetWithExpiration(ctx, key, raw, ttl) }
	}
	return value, nil
}

func cacheKey(item schema.IndexedTorrent) (string, string) {
	imdbID := imdbIDPattern.FindString(item.IMDB)
	if imdbID != "" { return "tmdb:imdb:"+imdbID, imdbID }
	name := strings.ToLower(strings.TrimSpace(item.OriginalTitle))
	if name == "" { name = strings.ToLower(strings.TrimSpace(item.Title)) }
	year := strings.TrimSpace(item.Year)
	mediaType := "movie"
	if item.Season > 0 || item.Episode > 0 { mediaType = "tv" }
	return "tmdb:search:"+mediaType+":"+url.QueryEscape(name)+":"+year, ""
}

func chooseResult(results []movieResult, query, year, mediaType string) movieResult {
	normalizedQuery := normalize(query)
	queryYear, _ := strconv.Atoi(year)
	for _, result := range results {
		name := result.Title
		date := result.ReleaseDate
		if mediaType == "tv" { name, date = result.OriginalName, result.FirstAirDate }
		if name == "" { name = result.OriginalName }
		result.MediaType = mediaType
		if normalize(name) != normalizedQuery { continue }
		if queryYear != 0 && len(date) >= 4 {
			resultYear, _ := strconv.Atoi(date[:4])
			if resultYear != 0 && resultYear != queryYear { continue }
		}
		return result
	}
	return movieResult{}
}

func normalize(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		switch r {
		case '\u00e1', '\u00e0', '\u00e2', '\u00e3', '\u00e4':
			r = 'a'
		case '\u00e9', '\u00e8', '\u00ea', '\u00eb':
			r = 'e'
		case '\u00ed', '\u00ec', '\u00ee', '\u00ef':
			r = 'i'
		case '\u00f3', '\u00f2', '\u00f4', '\u00f5', '\u00f6':
			r = 'o'
		case '\u00fa', '\u00f9', '\u00fb', '\u00fc':
			r = 'u'
		case '\u00e7':
			r = 'c'
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
