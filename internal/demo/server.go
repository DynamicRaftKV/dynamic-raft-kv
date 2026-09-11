// Package demo provides an opt-in presentation adapter for the standalone
// state engine. It is not the client API and makes no consensus guarantees.
package demo

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"sync"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
)

//go:embed web/*
var assets embed.FS

type server struct {
	mu       sync.Mutex
	sm       state.StateMachine
	commands uint64
}

// NewHandler serves a separate /demo/ surface. Restarting loses demo data.
func NewHandler(sm state.StateMachine) http.Handler {
	s := &server{sm: sm}
	mux := http.NewServeMux()
	web, _ := fs.Sub(assets, "web")
	mux.Handle("GET /demo/", http.StripPrefix("/demo/", http.FileServer(http.FS(web))))
	mux.HandleFunc("GET /demo/api/state", s.view)
	mux.HandleFunc("POST /demo/api/command", s.apply)
	mux.HandleFunc("GET /demo/api/snapshot", s.snapshot)
	mux.HandleFunc("POST /demo/api/restore", s.restore)
	return http.NewCrossOriginProtection().Handler(mux)
}

func reply(w http.ResponseWriter, code int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_, _ = w.Write(data)
}
func fail(w http.ResponseWriter, code int, err error) {
	data, _ := json.Marshal(map[string]string{"error": err.Error()})
	reply(w, code, data)
}
func body(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	// Presentation adapter limit only; the engine's contract is unchanged.
	return io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
}
func (s *server) apply(w http.ResponseWriter, r *http.Request) {
	data, err := body(w, r)
	if err != nil {
		fail(w, http.StatusRequestEntityTooLarge, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := s.sm.Apply(data)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	s.commands++
	reply(w, http.StatusOK, result)
}
func (s *server) view(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, err := s.sm.Snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	data, err := json.Marshal(struct {
		Snapshot json.RawMessage `json:"snapshot"`
		Commands uint64          `json:"commands"`
	}{snapshot, s.commands})
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	reply(w, http.StatusOK, data)
}
func (s *server) snapshot(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.sm.Snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	reply(w, http.StatusOK, data)
}
func (s *server) restore(w http.ResponseWriter, r *http.Request) {
	data, err := body(w, r)
	if err != nil {
		fail(w, http.StatusRequestEntityTooLarge, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.sm.Restore(data); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	reply(w, http.StatusOK, []byte(`{"restored":true}`))
}
