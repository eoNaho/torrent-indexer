package handler

import (
	"slices"
	"testing"
	"time"

	"github.com/felipemarinho97/torrent-indexer/schema"
)

func Test_CatalogoBetorListParsing(t *testing.T) {
	doc := loadFixture(t, "catalogo_betor_list.html")
	torrents := parseCatalogoBetorDocument(doc)

	if len(torrents) != 50 {
		t.Fatalf("got %d torrents, want 50", len(torrents))
	}

	first := torrents[0]
	if first.Title != "Russell Brand - Um Segredo Nada Secreto.2026.1080p.WEB-DL.DUAL.5.1" {
		t.Errorf("first.Title = %q", first.Title)
	}
	if first.IMDB != "tt50969289" {
		t.Errorf("first.IMDB = %q, want tt50969289", first.IMDB)
	}
	if first.Year != "2026" {
		t.Errorf("first.Year = %q, want 2026", first.Year)
	}
	if first.SeedCount != 13 {
		t.Errorf("first.SeedCount = %d, want 13", first.SeedCount)
	}
	if first.LeechCount != 23 {
		t.Errorf("first.LeechCount = %d, want 23", first.LeechCount)
	}
	if first.Size == "" {
		t.Errorf("first.Size should not be empty")
	}
	if first.InfoHash == "" {
		t.Errorf("first.InfoHash should not be empty")
	}
	if len(first.Files) != 2 {
		t.Errorf("len(first.Files) = %d, want 2", len(first.Files))
	}
	if !slices.Contains(first.Audio, schema.Audio(schema.AudioPortuguese)) || !slices.Contains(first.Audio, schema.Audio(schema.AudioEnglish)) {
		t.Errorf("first.Audio = %v, want Portuguese and English", first.Audio)
	}
	wantDate := time.Date(2026, 11, 15, 14, 1, 10, 947000000, time.UTC)
	if !first.Date.Equal(wantDate) {
		t.Errorf("first.Date = %v, want %v", first.Date, wantDate)
	}
}

func Test_buildCatalogoBetorURL(t *testing.T) {
	meta := IndexerMeta{
		Label:       "catalogo_betor",
		URL:         "https://catalogo.betor.top/",
		SearchURL:   "search/?q=",
		PagePattern: "filmes-e-series/%s/",
	}

	tests := []struct {
		name   string
		q      string
		imdb   string
		season string
		page   string
		want   string
	}{
		{
			name: "default listing",
			want: "https://catalogo.betor.top/filmes-e-series/",
		},
		{
			name: "page 1 uses default listing",
			page: "1",
			want: "https://catalogo.betor.top/filmes-e-series/",
		},
		{
			name: "page 2 listing",
			page: "2",
			want: "https://catalogo.betor.top/filmes-e-series/2/",
		},
		{
			name: "search query",
			q:    "natal sangrento",
			want: "https://catalogo.betor.top/search/?q=natal+sangrento",
		},
		{
			name: "imdb via q",
			q:    "tt50969289",
			want: "https://catalogo.betor.top/imdb/tt50969289/",
		},
		{
			name:   "imdb with season",
			imdb:   "tt4574334",
			season: "5",
			want:   "https://catalogo.betor.top/imdb/tt4574334/season/5/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildCatalogoBetorURL(meta, tc.q, tc.imdb, tc.season, tc.page)
			if got != tc.want {
				t.Errorf("buildCatalogoBetorURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func Test_parseCatalogoBetorLanguages(t *testing.T) {
	got := parseCatalogoBetorLanguages("pt-BR, en, pt, es")
	want := []schema.Audio{
		schema.AudioPortuguese,
		schema.AudioEnglish,
		schema.AudioSpanish,
	}
	if !slices.Equal(got, want) {
		t.Errorf("parseCatalogoBetorLanguages() = %v, want %v", got, want)
	}
}
