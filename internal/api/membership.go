package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
)

type membershipNode interface {
	ChangeMembership(context.Context, string, string, *raft.Peer) error
}

func membershipHandler(node raft.RaftNode, operation string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, ok := node.(membershipNode)
		if !ok {
			respondError(w, 501, "membership unavailable")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var id string
		var peer *raft.Peer
		if operation == "add_learner" {
			var body JoinRequest
			if !decode(r, &body) || body.ID == "" {
				respondError(w, 400, "expected id, grpc_address and http_address")
				return
			}
			host, port, err := net.SplitHostPort(body.GRPCAddress)
			u, urlErr := url.Parse(body.HTTPAddress)
			if err != nil || host == "" || port == "" || port == "0" || urlErr != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
				respondError(w, 400, "invalid peer addresses")
				return
			}
			peer = &raft.Peer{ID: body.ID, GRPCAddress: body.GRPCAddress, HTTPAddress: body.HTTPAddress, Learner: true}
		} else {
			var body RemoveRequest
			if !decode(r, &body) || body.ID == "" {
				respondError(w, 400, "expected member id")
				return
			}
			id = body.ID
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		err := m.ChangeMembership(ctx, operation, id, peer)
		if err != nil {
			switch {
			case errors.Is(err, raft.ErrNotLeader):
				address := node.Status().LeaderHTTPAddress
				u, parseErr := url.Parse(address)
				if parseErr == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https") && u.User == nil && u.RawQuery == "" && u.Fragment == "" {
					w.Header().Set("Location", strings.TrimRight(address, "/")+r.URL.EscapedPath())
					respondError(w, 307, "retry at leader; inspect status before retrying an uncertain change")
					return
				}
				respondError(w, 503, "leader unavailable")
			case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
				respondError(w, 504, "membership wait ended; outcome may be unknown, inspect status")
			case errors.Is(err, raft.ErrStopped):
				respondError(w, 503, err.Error())
			default:
				respondError(w, 409, err.Error())
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// 202 acknowledges the committed requested entry, not completion of
		// joint consensus. Poll membership_pending before the next operation.
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(node.Status())
	}
}
