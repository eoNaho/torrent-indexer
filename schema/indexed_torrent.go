package schema

import "time"

type IndexedTorrent struct {
	Title         string    `json:"title"`
	OriginalTitle string    `json:"original_title"`
	Details       string    `json:"details"`
	Year          string    `json:"year"`
	IMDB          string    `json:"imdb"`
	TMDBID        int       `json:"tmdb_id,omitempty"`
	TMDBType      string    `json:"tmdb_type,omitempty"`
	TMDBURL       string    `json:"tmdb_url,omitempty"`
	TMDBOverview  string    `json:"tmdb_overview,omitempty"`
	TMDBPoster    string    `json:"tmdb_poster,omitempty"`
	TMDBBackdrop  string    `json:"tmdb_backdrop,omitempty"`
	TMDBGenres    []string  `json:"tmdb_genres,omitempty"`
	TMDBRuntime   int       `json:"tmdb_runtime,omitempty"`
	TMDBRating    float64   `json:"tmdb_rating,omitempty"`
	TMDBRelease   string    `json:"tmdb_release,omitempty"`
	Audio         []Audio   `json:"audio"`
	MagnetLink    string    `json:"magnet_link"`
	Date          time.Time `json:"date"`
	InfoHash      string    `json:"info_hash"`
	Trackers      []string  `json:"trackers"`
	Size          string    `json:"size"`
	Files         []File    `json:"files,omitempty"`
	LeechCount    int       `json:"leech_count"`
	SeedCount     int       `json:"seed_count"`
	Similarity    float32   `json:"similarity"`
	Season        int       `json:"season,omitempty"`
	Episode       int       `json:"episode,omitempty"`
	ReleaseType   string    `json:"release_type,omitempty"`
}

type File struct {
	Path string `json:"path"`
	Size string `json:"size"`
}
