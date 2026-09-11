package state_test

import (
	"encoding/json"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
	"testing"
)

func TestInterfaceHarness(t *testing.T) {
	drive(t, state.NewStore(), state.NewStore())
}

func drive(t *testing.T, source, restored state.StateMachine) {
	t.Helper()
	put, _ := state.EncodeCommand(state.Command{Op: state.OpPut, Key: "hello", Value: "world"})
	if _, err := source.Apply(put); err != nil {
		t.Fatal(err)
	}
	snapshot, err := source.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := restored.Restore(snapshot); err != nil {
		t.Fatal(err)
	}
	get, _ := state.EncodeCommand(state.Command{Op: state.OpGet, Key: "hello"})
	result, err := restored.Apply(get)
	if err != nil {
		t.Fatal(err)
	}
	var got state.Result
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Found || got.Value != "world" {
		t.Fatalf("unexpected result %+v", got)
	}
}
