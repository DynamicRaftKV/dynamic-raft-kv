package observability

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
)

// Handler adds read-only diagnostics without introducing networking into raft.
func Handler(node *raft.Node, dataDir string, next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", next)
	mux.HandleFunc("GET /debug/raft", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(node.Diagnostics())
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if node.Err() != nil {
			http.Error(w, "stopped", 503)
			return
		}
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.Header().Set("Cache-Control", "no-store")
		d := node.Diagnostics()
		metric := func(name, kind string, value uint64) {
			fmt.Fprintf(w, "# TYPE raftkv_%s %s\nraftkv_%s %d\n", name, kind, name, value)
		}
		metric("term", "gauge", d.Term)
		metric("commit_index", "gauge", d.Commit)
		metric("applied_index", "gauge", d.Applied)
		metric("last_log_index", "gauge", d.LastIndex)
		metric("snapshot_index", "gauge", d.SnapshotIndex)
		metric("pending_proposals", "gauge", uint64(d.Pending))
		metric("uncommitted_entries", "gauge", d.UncommittedEntries)
		metric("uncommitted_bytes", "gauge", d.UncommittedBytes)
		metric("proposals_accepted_total", "counter", d.Accepted)
		metric("proposals_rejected_total", "counter", d.Rejected)
		metric("proposal_wait_timeouts_total", "counter", d.TimedOut)
		metric("elections_total", "counter", d.Elections)
		metric("leaderships_total", "counter", d.Leaderships)
		leader := uint64(0)
		if d.Leader {
			leader = 1
		}
		metric("is_leader", "gauge", leader)
		if stat, err := os.Stat(filepath.Join(dataDir, "raft.wal")); err == nil {
			metric("wal_bytes", "gauge", uint64(stat.Size()))
		}
	})
	return mux
}
