package demo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
)

// NewClusterHandler keeps the sandbox separate and bridges only explicitly
// configured nodes. It never follows redirects to arbitrary hosts.
func NewClusterHandler(sm state.StateMachine, peers []raft.Peer) http.Handler {
	targets := map[string]raft.Peer{}
	for _, p := range peers {
		targets[p.ID] = p
	}
	client := &http.Client{Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	statusClient := &http.Client{Timeout: 700 * time.Millisecond, CheckRedirect: client.CheckRedirect}
	mux := http.NewServeMux()
	mux.Handle("/", NewHandler(sm))
	mux.HandleFunc("GET /demo/api/cluster/status", func(w http.ResponseWriter, r *http.Request) {
		type row struct {
			Peer        raft.Peer         `json:"peer"`
			Status      *raft.NodeStatus  `json:"status,omitempty"`
			Error       string            `json:"error,omitempty"`
			Diagnostics *raft.Diagnostics `json:"diagnostics,omitempty"`
		}
		rows := make([]row, 0, len(peers))
		for _, p := range peers {
			item := row{Peer: p}
			req, _ := http.NewRequestWithContext(r.Context(), "GET", strings.TrimRight(p.HTTPAddress, "/")+"/cluster/status", nil)
			if req == nil {
				item.Error = "Invalid configured address"
				rows = append(rows, item)
				continue
			}
			resp, err := statusClient.Do(req)
			if err != nil {
				item.Error = "Offline or unreachable"
			} else {
				var status raft.NodeStatus
				if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&status) != nil {
					item.Error = "Status unavailable"
				} else {
					item.Status = &status
				}
				resp.Body.Close()
				if item.Status != nil {
					probe, _ := http.NewRequestWithContext(r.Context(), "GET", strings.TrimRight(p.HTTPAddress, "/")+"/debug/raft", nil)
					if probe != nil {
						if result, e := statusClient.Do(probe); e == nil {
							var d raft.Diagnostics
							if result.StatusCode == 200 && json.NewDecoder(io.LimitReader(result.Body, 1<<20)).Decode(&d) == nil {
								item.Diagnostics = &d
							}
							result.Body.Close()
						}
					}
				}
			}
			rows = append(rows, item)
		}
		data, _ := json.Marshal(rows)
		reply(w, 200, data)
	})
	mux.HandleFunc("POST /demo/api/cluster/command", func(w http.ResponseWriter, r *http.Request) {
		var command struct {
			Node      string `json:"node"`
			Operation string `json:"operation"`
			Key       string `json:"key"`
			Value     string `json:"value"`
			Member    string `json:"member"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&command); err != nil {
			fail(w, 400, err)
			return
		}
		if decoder.Decode(new(any)) != io.EOF {
			fail(w, 400, fmt.Errorf("expected one command"))
			return
		}
		p, ok := targets[command.Node]
		if !ok {
			fail(w, 400, fmt.Errorf("unknown configured node"))
			return
		}
		var method, path string
		var payload any
		switch command.Operation {
		case "PUT", "GET", "DELETE":
			if command.Key == "" || command.Key == "." || command.Key == ".." {
				fail(w, 400, fmt.Errorf("enter a valid key"))
				return
			}
			method, path = command.Operation, "/kv/"+url.PathEscape(command.Key)
			if method == "PUT" {
				payload = map[string]string{"value": command.Value}
			}
		case "join", "promote", "remove":
			member, ok := targets[command.Member]
			if !ok {
				fail(w, 400, fmt.Errorf("unknown configured member"))
				return
			}
			method, path = "POST", "/cluster/"+command.Operation
			if command.Operation == "join" {
				payload = map[string]string{"id": member.ID, "grpc_address": member.GRPCAddress, "http_address": member.HTTPAddress}
			} else {
				payload = map[string]string{"id": member.ID}
			}
		default:
			fail(w, 400, fmt.Errorf("unsupported operation"))
			return
		}
		data, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(r.Context(), method, strings.TrimRight(p.HTTPAddress, "/")+path, bytes.NewReader(data))
		if err != nil {
			fail(w, 400, err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			fail(w, 503, fmt.Errorf("node unavailable; outcome may be unknown, refresh status"))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode == 307 {
			fail(w, 409, fmt.Errorf("leader changed; refresh and submit to the current leader"))
			return
		}
		result, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		if err != nil {
			fail(w, 502, err)
			return
		}
		reply(w, resp.StatusCode, result)
	})
	return http.NewCrossOriginProtection().Handler(mux)
}
