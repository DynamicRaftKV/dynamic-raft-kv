package raft

import (
	"context"
	"testing"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
)

// This is a durable single-node fsync baseline, not distributed throughput.
func BenchmarkDurableSingleton(b *testing.B) {
	disk, err := storage.OpenWAL(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	defer disk.Close()
	n, err := NewNode(Config{ID: "one", Peers: []Peer{{ID: "one"}}, Storage: disk, StateMachine: state.NewStore(), Transport: testTransport{}, SnapshotEntries: 128})
	if err != nil {
		b.Fatal(err)
	}
	defer n.Stop()
	if err := n.Campaign(context.Background()); err != nil {
		b.Fatal(err)
	}
	command := []byte(`{"op":"PUT","key":"bench","value":"value"}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := n.Propose(context.Background(), command); err != nil {
			b.Fatal(err)
		}
	}
}
