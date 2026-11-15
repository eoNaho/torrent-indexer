package handler

import (
	"strings"
	"testing"
)

func Test_SemTorrentSearchListing(t *testing.T) {
	doc := loadFixture(t, "semtorrent_search.html")
	links := extractSemTorrentLinks(doc)
	if len(links) == 0 {
		t.Fatal("expected at least 1 link in semtorrent_search.html")
	}
	for _, l := range links {
		if !strings.HasPrefix(l, "https://semtorrent.com/") {
			t.Errorf("unexpected link %q", l)
		}
	}
}

func Test_SemTorrentPostParsing(t *testing.T) {
	doc := loadFixture(t, "semtorrent_post.html")
	post := parseSemTorrentPost(doc)

	if !strings.Contains(post.Title, "Russell Brand") {
		t.Errorf("post.Title = %q, want Russell Brand", post.Title)
	}
	if post.Year != "2026" {
		t.Errorf("post.Year = %q, want 2026", post.Year)
	}
	if post.IMDB != "https://www.imdb.com/title/tt50969289/" {
		t.Errorf("post.IMDB = %q, want https://www.imdb.com/title/tt50969289/", post.IMDB)
	}
	if post.Date.IsZero() {
		t.Errorf("post.Date should not be zero")
	}
	if len(post.MagnetLinks) == 0 {
		t.Fatal("expected at least 1 magnet link in semtorrent_post.html")
	}
	if len(post.Audio) == 0 {
		t.Errorf("expected at least 1 audio language")
	}
}

func Test_buildSemTorrentURL(t *testing.T) {
	meta := IndexerMeta{
		Label:       "semtorrent",
		URL:         "https://semtorrent.com/",
		SearchURL:   "?busca=",
		PagePattern: "pagina/%s/",
	}

	tests := []struct {
		name string
		q    string
		page string
		want string
	}{
		{
			name: "default homepage",
			want: "https://semtorrent.com/",
		},
		{
			name: "page 1 uses homepage",
			page: "1",
			want: "https://semtorrent.com/",
		},
		{
			name: "page 2",
			page: "2",
			want: "https://semtorrent.com/pagina/2/",
		},
		{
			name: "search query",
			q:    "matrix reloaded",
			want: "https://semtorrent.com/?busca=matrix+reloaded",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildSemTorrentURL(meta, tc.q, tc.page)
			if got != tc.want {
				t.Errorf("buildSemTorrentURL() = %q, want %q", got, tc.want)
			}
		})
	}
}
