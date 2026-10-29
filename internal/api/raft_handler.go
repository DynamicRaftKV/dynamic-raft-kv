package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
)

// NewRaftHandler serves KV operations through consensus, including GET. Reads
// therefore require a leader and quorum; there is no stale local-read fallback.
// Membership operations use serialized durable configuration entries.
func NewRaftHandler(node raft.RaftNode) http.Handler {
	mux := http.NewServeMux()
	operate := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		command := state.Command{Op: state.OpType(r.Method), Key: r.PathValue("key")}
		if r.Method == http.MethodPut {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			var body PutRequest
			if !decode(r, &body) || body.Value == nil {
				respondError(w, 400, "expected one JSON object with string value, at most 1 MiB")
				return
			}
			command.Value = *body.Value
		}
		data, err := state.EncodeCommand(command)
		if err != nil {
			respondError(w, 400, err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		result, err := node.Propose(ctx, data)
		if err != nil {
			switch {
			case errors.Is(err, raft.ErrNotLeader):
				address := node.Status().LeaderHTTPAddress
				if address != "" {
					if !strings.Contains(address, "://") {
						address = "http://" + address
					}
					base, parseErr := url.Parse(address)
					if parseErr == nil && base.Host != "" && (base.Scheme == "http" || base.Scheme == "https") && base.User == nil && base.RawQuery == "" && base.Fragment == "" {
						location := strings.TrimRight(base.String(), "/") + r.URL.EscapedPath()
						if r.URL.RawQuery != "" {
							location += "?" + r.URL.RawQuery
						}
						w.Header().Set("Location", location)
						respondError(w, http.StatusTemporaryRedirect, "retry at leader; an earlier proposal may have committed")
						return
					}
				}
				respondError(w, 503, "leader unavailable; proposal outcome may be unknown")
			case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
				respondError(w, 504, "proposal wait ended; outcome may be unknown")
			case errors.Is(err, state.ErrInvalidCommand):
				respondError(w, 400, err.Error())
			case errors.Is(err, raft.ErrProposalTooLarge):
				respondError(w, http.StatusRequestEntityTooLarge, err.Error())
			default:
				respondError(w, 503, "node unavailable; proposal outcome may be unknown")
			}
			return
		}
		var decoded state.Result
		if err := json.Unmarshal(result, &decoded); err != nil {
			respondError(w, 500, "invalid state-machine result")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if command.Op == state.OpGet && !decoded.Found {
			w.WriteHeader(http.StatusNotFound)
		}
		_, _ = w.Write(result)
	}
	mux.HandleFunc("PUT /kv/{key}", operate)
	mux.HandleFunc("GET /kv/{key}", operate)
	mux.HandleFunc("DELETE /kv/{key}", operate)
	mux.HandleFunc("GET /cluster/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(node.Status())
	})
	mux.HandleFunc("POST /cluster/join", membershipHandler(node, "add_learner"))
	mux.HandleFunc("POST /cluster/promote", membershipHandler(node, "promote"))
	mux.HandleFunc("POST /cluster/remove", membershipHandler(node, "remove"))
	return mux
}
