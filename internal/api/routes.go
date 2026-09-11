package api

import "net/http"

// NewHandler exposes only Stage 1 stubs; it cannot mutate a KV or Raft node.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /kv/{key}", put)
	mux.HandleFunc("GET /kv/{key}", unimplemented)
	mux.HandleFunc("DELETE /kv/{key}", unimplemented)
	mux.HandleFunc("GET /cluster/status", unimplemented)
	mux.HandleFunc("POST /cluster/join", join)
	mux.HandleFunc("POST /cluster/remove", remove)
	return mux
}
