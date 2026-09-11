package storage_test

import (
	"errors"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
)

type mockStorage struct{}

var errStub = errors.New("test stub")

func (*mockStorage) Recover() (storage.Recovered, error)          { return storage.Recovered{}, errStub }
func (*mockStorage) SaveHardState(storage.HardState) error        { return errStub }
func (*mockStorage) Append([]storage.Entry) error                 { return errStub }
func (*mockStorage) InstallSnapshot(storage.Snapshot, bool) error { return errStub }
func (*mockStorage) Compact(uint64) error                         { return errStub }

var _ storage.Storage = (*mockStorage)(nil)
