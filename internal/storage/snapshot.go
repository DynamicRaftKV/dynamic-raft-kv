package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
)

// MaxSnapshotBytes bounds one in-memory snapshot and its WAL record encoding.
const MaxSnapshotBytes = 32 << 20

// Kept private: fault injection is not a runtime/admin feature.
type checkpointIO struct {
	create  func(string) (walFile, string, error)
	rename  func(string, string) error
	syncDir func(string) error
	open    func(string) (walFile, error)
}

func defaultCheckpointIO() checkpointIO {
	return checkpointIO{
		create: func(dir string) (walFile, string, error) {
			f, err := os.CreateTemp(dir, ".raft-checkpoint-*")
			if err != nil {
				return nil, "", err
			}
			return f, f.Name(), nil
		},
		rename: os.Rename, syncDir: syncDirectory,
		open: func(path string) (walFile, error) { return os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0600) },
	}
}

func cloneSnapshot(s *Snapshot) *Snapshot {
	if s == nil {
		return nil
	}
	out := *s
	out.Data, out.Configuration = bytes.Clone(s.Data), bytes.Clone(s.Configuration)
	return &out
}

func (w *WAL) boundary() (uint64, uint64) {
	if w.state.Snapshot == nil {
		return 0, 0
	}
	return w.state.Snapshot.LastIncludedIndex, w.state.Snapshot.LastIncludedTerm
}

// Installation is one checksummed record: a torn tail leaves the previous
// snapshot/log view intact, and a complete record publishes both together.
func (w *WAL) InstallSnapshot(s Snapshot, retain bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.persist(walRecord{Type: "snapshot", Snapshot: &s, Retain: retain})
}

// Compact replaces the WAL with a synced checkpoint and suffix, then syncs
// the directory before accepting further writes. A crash observes either
// complete generation. Orphan temporary files are never used for recovery.
func (w *WAL) Compact(through uint64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.usable(); err != nil {
		return err
	}
	base, _ := w.boundary()
	if base == 0 || through > base {
		return errors.New("storage: compaction requires a covering snapshot")
	}
	ops := defaultCheckpointIO()
	if w.checkpointIO != nil {
		ops = *w.checkpointIO
	}
	f, name, err := ops.create(w.dir)
	if err != nil {
		return w.poison(err)
	}
	defer os.Remove(name)
	defer f.Close()
	checkpoint := &WAL{file: f}
	if err = writeAll(f, []byte(walMagic)); err == nil {
		err = checkpoint.persist(walRecord{Type: "hard_state", Hard: &w.state.HardState})
	}
	if err == nil {
		err = checkpoint.persist(walRecord{Type: "snapshot", Snapshot: w.state.Snapshot})
	}
	// Per-entry frames keep a large retained suffix below the record size cap.
	for _, e := range w.state.Entries {
		if err != nil {
			break
		}
		err = checkpoint.persist(walRecord{Type: "append", Entries: []Entry{e}})
	}
	if err != nil {
		return w.poison(err)
	}
	if w.checkpointHook != nil {
		w.checkpointHook("before_rename")
	}
	if err = ops.rename(name, filepath.Join(w.dir, "raft.wal")); err != nil {
		return w.poison(err)
	}
	if w.checkpointHook != nil {
		w.checkpointHook("after_rename")
	}
	if err = ops.syncDir(w.dir); err != nil {
		return w.poison(err)
	}
	if w.checkpointHook != nil {
		w.checkpointHook("after_directory_sync")
	}
	// Reopen after rename; never continue appending to the retired inode.
	replacement, err := ops.open(filepath.Join(w.dir, "raft.wal"))
	if err != nil {
		return w.poison(err)
	}
	old := w.file
	w.file = replacement
	if err = old.Close(); err != nil {
		return w.poison(err)
	}
	return nil
}
