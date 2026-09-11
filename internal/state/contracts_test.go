package state_test

import (
	"errors"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
)

// These test-only bodies demonstrate implementability, not KV behavior.
type mockSM struct{}

var errStub = errors.New("test stub")

func (*mockSM) Apply([]byte) ([]byte, error) { return nil, errStub }
func (*mockSM) Snapshot() ([]byte, error)    { return nil, errStub }
func (*mockSM) Restore([]byte) error         { return errStub }

var _ state.StateMachine = (*mockSM)(nil)
