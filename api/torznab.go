package handler

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/felipemarinho97/torrent-indexer/schema"
	"github.com/felipemarinho97/torrent-indexer/utils"
)

type rss struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Atom    string   `xml:"xmlns:atom,attr"`
	Torznab string   `xml:"xmlns:torznab,attr"`
	Channel channel  `xml:"channel"`
}

type channel struct {
	Title string `xml:"title"`
	Items []item `xml:"item"`
}

type item struct {
	Title     string    `xml:"title"`
	GUID      string    `xml:"guid"`
	Link      string    `xml:"link"`
	PubDate   string    `xml:"pubDate"`
	Size      int64     `xml:"size"`
	Category  int       `xml:"category"`
	Enclosure enclosure `xml:"enclosure"`
	Attrs     []attr    `xml:"http://torznab.com/schemas/2015/feed attr"`
}

type enclosure struct {
	URL    string `xml:"url,attr"`
	Length int64  `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

type attr struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

func (i *Indexer) HandlerTorznabAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml")

	t := r.URL.Query().Get("t")
	query := r.URL.Query()
	q := query.Get("q")

	if t == "caps" {
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<caps>
  <server version="1.0" title="torrent-indexer"/>
  <searching>
    <search available="yes" supportedParams="q"/>
    <tv-search available="yes" supportedParams="q,season,ep"/>
    <movie-search available="yes" supportedParams="q"/>
  </searching>
  <categories>
    <category id="2000" name="Movies"/>
    <category id="5000" name="TV"/>
  </categories>
</caps>`)
		return
	}

	if t == "" || (q == "" && query.Get("season") == "" && query.Get("ep") == "" && query.Get("imdbid") == "") {
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom" xmlns:torznab="http://torznab.com/schemas/2015/feed">
  <channel><title>torrent-indexer</title></channel>
</rss>`)
		return
	}

	results, indexerName := i.callIndexer(r)
	xml := convertToTorznab(indexerName, results)
	_, _ = w.Write(xml)
}

func (i *Indexer) callIndexer(r *http.Request) ([]schema.IndexedTorrent, string) {
	path := r.URL.Path
	indexerName := strings.TrimPrefix(path, "/api")
	indexerName = strings.TrimPrefix(indexerName, "/")
	indexerName = strings.TrimSuffix(indexerName, "/api")
	indexerName = strings.TrimSuffix(indexerName, "/")
	if indexerName == "" {
		indexerName = r.URL.Query().Get("indexer")
	}
	if indexerName == "" {
		indexerName = "rede_torrent"
	}

	handlerPath := fmt.Sprintf("/indexers/%s", indexerName)
	params := r.URL.Query()
	params.Del("t")
	params.Del("apikey")
	params.Del("indexer")
	if ep := params.Get("ep"); ep != "" {
		params.Set("episode", ep)
		params.Del("ep")
	}
	targetURL := handlerPath
	if encoded := params.Encode(); encoded != "" {
		targetURL += "?" + encoded
	}

	rr := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(r.Context(), "GET", targetURL, nil)
	if err != nil {
		return nil, indexerName
	}

	mux := http.NewServeMux()
	mux.HandleFunc(handlerPath, func(w http.ResponseWriter, r *http.Request) {
		switch indexerName {
		case "rede_torrent":
			i.HandlerRedeTorrentIndexer(w, r)
		case "comando_torrents":
			i.HandlerComandoIndexer(w, r)
		case "bludv":
			i.HandlerBluDVIndexer(w, r)
		case "starck-filmes":
			i.HandlerStarckFilmesIndexer(w, r)
		case "vaca_torrent":
			i.HandlerVacaTorrentIndexer(w, r)
		case "torrent-dos-filmes":
			i.HandlerTorrentDosFilmesIndexer(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	mux.ServeHTTP(rr, req)

	var resp Response
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		return nil, indexerName
	}

	return resp.Results, indexerName
}

func convertToTorznab(indexerName string, results []schema.IndexedTorrent) []byte {
	ch := channel{
		Title: indexerName,
		Items: make([]item, 0, len(results)),
	}

	for _, r := range results {
		if r.MagnetLink == "" {
			continue
		}

		title := r.Title
		if title == "" {
			title = r.OriginalTitle
		}

		sizeBytes := utils.ParseSize(r.Size)

		pubDate := r.Date.Format(time.RFC1123Z)
		if r.Date.IsZero() {
			pubDate = time.Now().Format(time.RFC1123Z)
		}

		category := 2000
		if r.Season != 0 || r.Episode != 0 || r.ReleaseType != "" {
			category = 5000
		}

		it := item{
			Title:    title,
			GUID:     r.InfoHash,
			Link:     r.Details,
			PubDate:  pubDate,
			Size:     sizeBytes,
			Category: category,
			Enclosure: enclosure{
				URL:    r.MagnetLink,
				Length: sizeBytes,
				Type:   "application/x-bittorrent",
			},
			Attrs: []attr{
				{Name: "magneturl", Value: r.MagnetLink},
				{Name: "infohash", Value: r.InfoHash},
				{Name: "category", Value: strconv.Itoa(category)},
				{Name: "seeders", Value: strconv.Itoa(r.SeedCount)},
				{Name: "peers", Value: strconv.Itoa(r.LeechCount + r.SeedCount)},
				{Name: "size", Value: strconv.FormatInt(sizeBytes, 10)},
				{Name: "imdb", Value: r.IMDB},
				{Name: "year", Value: r.Year},
			},
		}
		if r.Season != 0 {
			it.Attrs = append(it.Attrs, attr{Name: "season", Value: strconv.Itoa(r.Season)})
		}
		if r.Episode != 0 {
			it.Attrs = append(it.Attrs, attr{Name: "episode", Value: strconv.Itoa(r.Episode)})
		}
		ch.Items = append(ch.Items, it)
	}

	rssDoc := rss{
		Version: "2.0",
		Atom:    "http://www.w3.org/2005/Atom",
		Torznab: "http://torznab.com/schemas/2015/feed",
		Channel: ch,
	}

	out, _ := xml.MarshalIndent(rssDoc, "", "  ")
	return []byte(xml.Header + string(out))
}
