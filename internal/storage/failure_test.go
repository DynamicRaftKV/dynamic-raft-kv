package storage

import (
	"errors"
	"io"
	"reflect"
	"syscall"
	"testing"
)

type failureFile struct {
	walFile
	writeErr, syncErr, closeErr error
	partial                     bool
}

func (f *failureFile) Write(p []byte) (int, error) {
	if f.writeErr == nil {
		return f.walFile.Write(p)
	}
	if f.partial && len(p) > 0 {
		n, err := f.walFile.Write(p[:1])
		if err != nil {
			return n, err
		}
		return n, f.writeErr
	}
	return 0, f.writeErr
}
func (f *failureFile) Sync() error {
	if f.syncErr != nil {
		return f.syncErr
	}
	return f.walFile.Sync()
}
func (f *failureFile) Close() error {
	err := f.walFile.Close()
	if f.closeErr != nil {
		return f.closeErr
	}
	return err
}

func assertPoisoned(t *testing.T, w *WAL) {
	t.Helper()
	if _, err := w.Recover(); err == nil {
		t.Fatal("recovery allowed on poisoned handle")
	}
	if err := w.SaveHardState(HardState{Term: 9}); err == nil {
		t.Fatal("hard state accepted after failure")
	}
	if err := w.Append(nil); err == nil {
		t.Fatal("append accepted after failure")
	}
	if err := w.InstallSnapshot(Snapshot{LastIncludedIndex: 4, LastIncludedTerm: 3}, false); err == nil {
		t.Fatal("snapshot accepted after failure")
	}
	if err := w.Compact(2); err == nil {
		t.Fatal("compaction accepted after failure")
	}
}

func TestCheckpointIOFailureMatrix(t *testing.T) {
	for _, stage := range []string{"create", "header_write", "record_write", "file_sync", "rename", "directory_sync", "reopen", "old_close"} {
		for _, cause := range []error{syscall.ENOSPC, syscall.EIO} {
			t.Run(stage+"/"+cause.Error(), func(t *testing.T) {
				dir := t.TempDir()
				w := openTestWAL(t, dir)
				seedWAL(t, w)
				must(t, w.InstallSnapshot(Snapshot{LastIncludedIndex: 2, LastIncludedTerm: 2, Data: []byte("stable")}, true))
				want, err := w.Recover()
				must(t, err)
				ops := defaultCheckpointIO()
				create := ops.create
				switch stage {
				case "create":
					ops.create = func(string) (walFile, string, error) { return nil, "", cause }
				case "header_write", "record_write", "file_sync":
					ops.create = func(dir string) (walFile, string, error) {
						f, name, err := create(dir)
						if err != nil {
							return nil, "", err
						}
						broken := &failureFile{walFile: f, partial: true}
						if stage == "file_sync" {
							broken.syncErr = cause
						} else {
							broken.writeErr = cause
						}
						if stage == "record_write" {
							return &afterHeaderFailure{walFile: f, broken: broken}, name, nil
						}
						return broken, name, nil
					}
				case "rename":
					ops.rename = func(string, string) error { return cause }
				case "directory_sync":
					ops.syncDir = func(string) error { return cause }
				case "reopen":
					ops.open = func(string) (walFile, error) { return nil, cause }
				case "old_close":
					w.file = &failureFile{walFile: w.file, closeErr: cause}
				}
				w.checkpointIO = &ops
				if err := w.Compact(2); !errors.Is(err, cause) {
					t.Fatalf("missing failure: %v", err)
				}
				assertPoisoned(t, w)
				w.Close()
				w = openTestWAL(t, dir)
				got, err := w.Recover()
				must(t, err)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("mixed or lost state after %s: %+v", stage, got)
				}
				must(t, w.Append([]Entry{{Index: 4, Term: 3, Data: []byte("after recovery")}}))
				must(t, w.Compact(2))
				must(t, w.Close())
				w = openTestWAL(t, dir)
				got, err = w.Recover()
				must(t, err)
				if len(got.Entries) != 2 || got.Entries[1].Index != 4 {
					t.Fatal("post-failure writes lost")
				}
			})
		}
	}
}

type afterHeaderFailure struct {
	walFile
	broken  *failureFile
	written bool
}

func (f *afterHeaderFailure) Write(p []byte) (int, error) {
	if !f.written {
		f.written = true
		return f.walFile.Write(p)
	}
	return f.broken.Write(p)
}

func TestWALWriteFailureMatrix(t *testing.T) {
	for _, operation := range []string{"hard_state", "append", "snapshot"} {
		for _, mode := range []string{"no_space", "partial_write", "sync"} {
			t.Run(operation+"/"+mode, func(t *testing.T) {
				dir := t.TempDir()
				w := openTestWAL(t, dir)
				seedWAL(t, w)
				before, err := w.Recover()
				must(t, err)
				broken := &failureFile{walFile: w.file}
				switch mode {
				case "no_space":
					broken.writeErr = syscall.ENOSPC
				case "partial_write":
					broken.writeErr = io.ErrShortWrite
					broken.partial = true
				case "sync":
					broken.syncErr = syscall.EIO
				}
				w.file = broken
				s := Snapshot{LastIncludedIndex: 2, LastIncludedTerm: 2, Data: []byte("state")}
				switch operation {
				case "hard_state":
					err = w.SaveHardState(HardState{Term: 4})
				case "append":
					err = w.Append([]Entry{{Index: 4, Term: 3}})
				case "snapshot":
					err = w.InstallSnapshot(s, true)
				}
				if err == nil {
					t.Fatal("acknowledged failed I/O")
				}
				assertPoisoned(t, w)
				w.Close()
				w = openTestWAL(t, dir)
				got, err := w.Recover()
				must(t, err)
				if mode != "sync" && !reflect.DeepEqual(got, before) {
					t.Fatalf("partial operation recovered: %+v", got)
				}
				if mode == "sync" {
					// A completed write followed by failed fsync has unknown durability.
					// This process-level fixture retains the complete write; no mixed view.
					switch operation {
					case "hard_state":
						if got.HardState.Term != 4 {
							t.Fatal(got)
						}
					case "append":
						if len(got.Entries) != 4 {
							t.Fatal(got)
						}
					case "snapshot":
						if got.Snapshot == nil || got.Snapshot.LastIncludedIndex != 2 || len(got.Entries) != 1 {
							t.Fatal(got)
						}
					}
				}
			})
		}
	}
}
