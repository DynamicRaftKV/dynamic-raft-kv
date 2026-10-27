package storage

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func openTestWAL(t *testing.T, dir string) *WAL {
	t.Helper()
	w, err := OpenWAL(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close() })
	return w
}
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func seedWAL(t *testing.T, w *WAL) {
	t.Helper()
	must(t, w.SaveHardState(HardState{Term: 3, VotedFor: "a"}))
	must(t, w.Append([]Entry{{Index: 1, Term: 1, Data: []byte("first")}, {Index: 2, Term: 2, Data: []byte("old")}, {Index: 3, Term: 3, Kind: EntryNoop}}))
}

func TestWALRecoverySuffixReplacementAndOwnership(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "node")
	w := openTestWAL(t, dir)
	seedWAL(t, w)
	replacement := []Entry{{Index: 2, Term: 3, Data: []byte("new")}}
	must(t, w.Append(replacement))
	replacement[0].Data[0] = '!'
	recovered, err := w.Recover()
	must(t, err)
	if len(recovered.Entries) != 2 || string(recovered.Entries[1].Data) != "new" {
		t.Fatal(recovered)
	}
	expected := Recovered{HardState: HardState{Term: 3, VotedFor: "a"}, Entries: []Entry{{Index: 1, Term: 1, Data: []byte("first")}, {Index: 2, Term: 3, Data: []byte("new")}}}
	recovered.Entries[0].Data[0] = '!'
	must(t, w.Close())
	w = openTestWAL(t, dir)
	got, err := w.Recover()
	must(t, err)
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("%+v != %+v", got, expected)
	}
	if err := w.InstallSnapshot(Snapshot{}, false); !errors.Is(err, ErrSnapshotsUnsupported) {
		t.Fatal(err)
	}
	if err := w.Compact(1); !errors.Is(err, ErrSnapshotsUnsupported) {
		t.Fatal(err)
	}
}

func TestWALRejectsInvalidTransitionsWithoutPoisoning(t *testing.T) {
	w := openTestWAL(t, t.TempDir())
	seedWAL(t, w)
	for _, h := range []HardState{{Term: 2}, {Term: 3, VotedFor: "b"}, {Term: 3}} {
		if err := w.SaveHardState(h); err == nil {
			t.Fatal("accepted invalid vote", h)
		}
	}
	for _, es := range [][]Entry{
		{{Index: 0, Term: 3}}, {{Index: 5, Term: 3}}, {{Index: 4, Term: 4}}, {{Index: 4, Term: 3}, {Index: 6, Term: 3}}, {{Index: 4, Term: 2}}, {{Index: 4, Term: 3, Kind: 255}},
	} {
		if err := w.Append(es); err == nil {
			t.Fatal("accepted invalid log", es)
		}
	}
	must(t, w.SaveHardState(HardState{Term: 4, VotedFor: "b"}))
	must(t, w.Append([]Entry{{Index: 4, Term: 4}}))
}

func TestWALDirectoryOwnershipAndClose(t *testing.T) {
	dir := t.TempDir()
	w := openTestWAL(t, dir)
	if other, err := OpenWAL(dir); !errors.Is(err, ErrLocked) {
		if other != nil {
			other.Close()
		}
		t.Fatalf("second opener: %v", err)
	}
	must(t, w.Close())
	must(t, w.Close())
	if _, err := w.Recover(); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	if err := w.SaveHardState(HardState{Term: 1}); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	next := openTestWAL(t, dir)
	must(t, next.SaveHardState(HardState{Term: 1}))
}

func TestWALRepairsEveryIncompleteFinalRecord(t *testing.T) {
	source := t.TempDir()
	w := openTestWAL(t, source)
	must(t, w.SaveHardState(HardState{Term: 1, VotedFor: "a"}))
	before, err := os.ReadFile(filepath.Join(source, "raft.wal"))
	must(t, err)
	must(t, w.Append([]Entry{{Index: 1, Term: 1, Data: []byte("payload")}}))
	must(t, w.Close())
	full, err := os.ReadFile(filepath.Join(source, "raft.wal"))
	must(t, err)
	for cut := len(before); cut < len(full); cut++ {
		t.Run(fmt.Sprint(cut), func(t *testing.T) {
			dir := t.TempDir()
			must(t, os.WriteFile(filepath.Join(dir, "raft.wal"), full[:cut], 0600))
			repaired := openTestWAL(t, dir)
			got, err := repaired.Recover()
			must(t, err)
			if got.HardState.Term != 1 || len(got.Entries) != 0 {
				t.Fatal(got)
			}
			onDisk, err := os.ReadFile(filepath.Join(dir, "raft.wal"))
			must(t, err)
			if !bytes.Equal(onDisk, before) {
				t.Fatal("tail not trimmed to last complete record")
			}
			must(t, repaired.Append([]Entry{{Index: 1, Term: 1, Data: []byte("after-repair")}}))
			must(t, repaired.Close())
			reopened := openTestWAL(t, dir)
			r, err := reopened.Recover()
			must(t, err)
			if string(r.Entries[0].Data) != "after-repair" {
				t.Fatal(r)
			}
		})
	}
}

func TestWALRejectsCorruptionWithoutRewritingFile(t *testing.T) {
	source := t.TempDir()
	w := openTestWAL(t, source)
	seedWAL(t, w)
	must(t, w.Close())
	original, err := os.ReadFile(filepath.Join(source, "raft.wal"))
	must(t, err)
	for _, position := range []int{0, 8, 12, 16, 20, len(original) - 1} {
		t.Run(fmt.Sprint(position), func(t *testing.T) {
			data := bytes.Clone(original)
			data[position] ^= 0x40
			dir := t.TempDir()
			path := filepath.Join(dir, "raft.wal")
			must(t, os.WriteFile(path, data, 0600))
			bad, err := OpenWAL(dir)
			if !errors.Is(err, ErrCorrupt) {
				if bad != nil {
					bad.Close()
				}
				t.Fatalf("corruption accepted: %v", err)
			}
			after, err := os.ReadFile(path)
			must(t, err)
			if !bytes.Equal(after, data) {
				t.Fatal("corrupt WAL silently rewritten")
			}
		})
	}
}

type faultyFile struct {
	walFile
	syncFailure, writeFailure bool
}

func (f faultyFile) Sync() error {
	if f.syncFailure {
		return errors.New("injected sync failure")
	}
	return f.walFile.Sync()
}
func (f faultyFile) Write(b []byte) (int, error) {
	if f.writeFailure {
		n, err := f.walFile.Write(b[:5])
		if err != nil {
			return n, err
		}
		return n, io.ErrShortWrite
	}
	return f.walFile.Write(b)
}
func TestWALIOFailureStopsFurtherWork(t *testing.T) {
	for _, syncFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(syncFailure), func(t *testing.T) {
			dir := t.TempDir()
			w := openTestWAL(t, dir)
			must(t, w.SaveHardState(HardState{Term: 1}))
			w.file = faultyFile{walFile: w.file, syncFailure: syncFailure, writeFailure: !syncFailure}
			if err := w.Append([]Entry{{Index: 1, Term: 1}}); err == nil {
				t.Fatal("acknowledged failed I/O")
			}
			if _, err := w.Recover(); err == nil {
				t.Fatal("read from poisoned WAL")
			}
			if err := w.SaveHardState(HardState{Term: 2}); err == nil {
				t.Fatal("continued writing after I/O failure")
			}
			must(t, w.Close())
			reopened := openTestWAL(t, dir)
			r, err := reopened.Recover()
			must(t, err)
			if r.HardState.Term != 1 {
				t.Fatal("later write persisted after fatal failure")
			}
			// The failed Sync may have persisted its record; outcome is deliberately unknown.
			if !syncFailure && len(r.Entries) != 0 {
				t.Fatal("partial write survived as complete entry")
			}
		})
	}
}

func TestWALCrashHelper(t *testing.T) {
	dir := os.Getenv("RAFTKV_WAL_CRASH_TEST_DIR")
	if dir == "" {
		return
	}
	w, err := OpenWAL(dir)
	if err != nil {
		t.Fatal(err)
	}
	must(t, w.SaveHardState(HardState{Term: 4, VotedFor: "a"}))
	must(t, w.Append([]Entry{{Index: 1, Term: 4, Data: []byte("acknowledged")}}))
	fmt.Println("synced")
	<-time.After(time.Hour) // parent kills this process without Close
}
func TestWALSurvivesProcessKillAfterAcknowledgement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dir := t.TempDir()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWALCrashHelper$")
	cmd.Env = append(os.Environ(), "RAFTKV_WAL_CRASH_TEST_DIR="+dir)
	pipe, err := cmd.StdoutPipe()
	must(t, err)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	must(t, cmd.Start())
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})
	line, err := bufio.NewReader(pipe).ReadString('\n')
	must(t, err)
	if line != "synced\n" {
		t.Fatalf("helper: %q %s", line, stderr.String())
	}
	must(t, cmd.Process.Kill())
	_ = cmd.Wait()
	w := openTestWAL(t, dir)
	r, err := w.Recover()
	must(t, err)
	if r.HardState != (HardState{Term: 4, VotedFor: "a"}) || len(r.Entries) != 1 || string(r.Entries[0].Data) != "acknowledged" {
		t.Fatalf("lost acknowledged data: %+v", r)
	}
}
