package handler

import (
	"encoding/json"
	"net/http"
)

// HandlerHealth reports that the torrent-indexer HTTP service is responding.
func HandlerHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(struct {
		Service string `json:"service"`
		Status  string `json:"status"`
	}{Service: "torrent-indexer", Status: "ok"})
}
