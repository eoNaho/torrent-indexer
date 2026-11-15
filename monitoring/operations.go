package monitoring

import (
	"net/http"
	"sort"
	"sync"
	"time"
)

type IndexerOperation struct {
	Name          string    `json:"name"`
	Requests      uint64    `json:"requests"`
	Errors        uint64    `json:"errors"`
	LastRequest   *time.Time `json:"last_request,omitempty"`
	LastDuration  string    `json:"last_duration,omitempty"`
	LastStatus    int       `json:"last_status,omitempty"`
}

type Operations struct {
	started time.Time
	mu      sync.RWMutex
	state   map[string]IndexerOperation
}

func NewOperations(indexers ...string) *Operations {
	operations := &Operations{started: time.Now().UTC(), state: make(map[string]IndexerOperation, len(indexers))}
	for _, name := range indexers { operations.state[name] = IndexerOperation{Name: name} }
	return operations
}

func (o *Operations) Wrap(name string, next http.Handler) http.Handler {
	return o.WrapDynamic(func(*http.Request) string { return name }, next)
}

func (o *Operations) WrapDynamic(nameForRequest func(*http.Request) string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := nameForRequest(r)
		started := time.Now()
		writer := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(writer, r)
		duration := time.Since(started)
		o.mu.Lock()
		entry := o.state[name]
		entry.Name = name
		entry.Requests++
		if writer.status >= http.StatusBadRequest { entry.Errors++ }
		lastRequest := time.Now().UTC()
		entry.LastRequest = &lastRequest
		entry.LastDuration = duration.Round(time.Millisecond).String()
		entry.LastStatus = writer.status
		o.state[name] = entry
		o.mu.Unlock()
	})
}

func (o *Operations) Snapshot() ([]IndexerOperation, time.Duration) {
	o.mu.RLock()
	entries := make([]IndexerOperation, 0, len(o.state))
	for _, entry := range o.state { entries = append(entries, entry) }
	o.mu.RUnlock()
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, time.Since(o.started).Round(time.Second)
}

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(code int) {
	if w.wrote { return }
	w.status, w.wrote = code, true
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if !w.wrote { w.WriteHeader(http.StatusOK) }
	return w.ResponseWriter.Write(body)
}
