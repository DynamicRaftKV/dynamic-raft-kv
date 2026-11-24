package raft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestCommitEventsTrackReplicationWithoutPayloads(t *testing.T) {
	c := newCluster(t, 3, false)
	buffers := make([]bytes.Buffer, len(c.nodes))
	for i, n := range c.nodes {
		n.c.Logger = slog.New(slog.NewJSONHandler(&buffers[i], nil))
	}
	c.campaign(0)
	c.propose(0, `{"op":"PUT","key":"private-key","value":"private-value"}`)
	// Repeated heartbeats must not emit duplicate progress events.
	c.flush(0)
	counts := make([]int, len(c.nodes))
	inspect := func(i int) int {
		t.Helper()
		n := c.nodes[i]
		n.mu.Lock()
		raw := buffers[i].String()
		commit := n.commit
		n.mu.Unlock()
		if strings.Contains(raw, "private-key") || strings.Contains(raw, "private-value") {
			t.Fatal("client payload leaked into logs")
		}
		var previous uint64
		count := 0
		for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
			var event struct {
				Message  string `json:"msg"`
				Node     string `json:"node"`
				Commit   uint64 `json:"commit"`
				Previous uint64 `json:"previous_commit"`
				Applied  uint64 `json:"applied"`
				Source   string `json:"source"`
			}
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				t.Fatal(err)
			}
			if event.Message != "commit_advanced" {
				continue
			}
			source := "leader_append"
			if i == 0 {
				source = "quorum"
			}
			if event.Node != n.c.ID || event.Source != source || event.Previous != previous || event.Commit <= previous || event.Applied > event.Commit {
				t.Fatalf("invalid progress event: %+v", event)
			}
			previous = event.Commit
			count++
		}
		if count == 0 || previous != commit {
			t.Fatalf("events end at %d, commit %d, count %d", previous, commit, count)
		}
		return count
	}
	for i := range c.nodes {
		counts[i] = inspect(i)
	}
	c.isolate(0)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := c.nodes[0].Propose(ctx, []byte(`{"op":"PUT","key":"private-key","value":"private-value"}`))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	c.flush(0)
	for i := range c.nodes {
		if inspect(i) != counts[i] {
			t.Fatal("commit event without quorum or index change")
		}
	}
	c.heal()
	c.flush(0)
	for i := range c.nodes {
		if inspect(i) <= counts[i] {
			t.Fatal("missing recovered commit progress")
		}
	}
}
