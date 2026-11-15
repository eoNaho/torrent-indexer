package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	meilisearch "github.com/felipemarinho97/torrent-indexer/search"
)

func Test_SearchTorrentHandler_MeilisearchPayloadAndEmptyResult(t *testing.T) {
	var capturedBody map[string]any
	meiliServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("failed to decode meilisearch request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"hits": []any{}})
	}))
	defer meiliServer.Close()

	si := meilisearch.NewSearchIndexer(meiliServer.URL, "", "torrents")
	h := NewMeilisearchHandler(si, nil)

	req := httptest.NewRequest(http.MethodGet, "/search?q=harry+potter", nil)
	rr := httptest.NewRecorder()
	h.SearchTorrentHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if capturedBody["matchingStrategy"] != "all" {
		t.Errorf("matchingStrategy = %v, want %q", capturedBody["matchingStrategy"], "all")
	}

	rawAttrs, ok := capturedBody["attributesToSearchOn"].([]any)
	if !ok {
		t.Fatalf("attributesToSearchOn missing or wrong type: %T", capturedBody["attributesToSearchOn"])
	}
	attrs := make([]string, 0, len(rawAttrs))
	for _, a := range rawAttrs {
		if s, ok := a.(string); ok {
			attrs = append(attrs, s)
		}
	}
	for _, want := range []string{"title", "original_title", "imdb", "year"} {
		if !slices.Contains(attrs, want) {
			t.Errorf("attributesToSearchOn = %v, missing %q", attrs, want)
		}
	}

	var resp struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Count != 0 {
		t.Errorf("count = %d, want 0", resp.Count)
	}
}
