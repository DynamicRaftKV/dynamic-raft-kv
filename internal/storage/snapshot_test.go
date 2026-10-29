package storage

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSnapshotCompactionCrashHelper(t *testing.T) {
	dir := os.Getenv("RAFTKV_SNAPSHOT_CRASH_DIR")
	if dir == "" {
		return
	}
	w := openTestWAL(t, dir)
	seedWAL(t, w)
	must(t, w.InstallSnapshot(Snapshot{LastIncludedIndex: 2, LastIncludedTerm: 2, Data: []byte("durable snapshot"), Configuration: []byte("peers")}, true))
	w.checkpointHook = func(stage string) {
		if stage == os.Getenv("RAFTKV_SNAPSHOT_CRASH_STAGE") {
			fmt.Println("ready")
			<-time.After(time.Hour)
		}
	}
	must(t, w.Compact(2))
}

func TestSnapshotCompactionProcessCrashBoundaries(t *testing.T) {
	for _, stage := range []string{"before_rename", "after_rename", "after_directory_sync"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			dir := t.TempDir()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSnapshotCompactionCrashHelper$")
			cmd.Env = append(os.Environ(), "RAFTKV_SNAPSHOT_CRASH_DIR="+dir, "RAFTKV_SNAPSHOT_CRASH_STAGE="+stage)
			pipe, err := cmd.StdoutPipe()
			must(t, err)
			must(t, cmd.Start())
			defer func() {
				if cmd.ProcessState == nil {
					cmd.Process.Kill()
					cmd.Wait()
				}
			}()
			line, err := bufio.NewReader(pipe).ReadString('\n')
			must(t, err)
			if line != "ready\n" {
				t.Fatal(line)
			}
			must(t, cmd.Process.Kill())
			cmd.Wait()
			w := openTestWAL(t, dir)
			r, err := w.Recover()
			must(t, err)
			if r.Snapshot == nil || string(r.Snapshot.Data) != "durable snapshot" || r.Snapshot.LastIncludedIndex != 2 || r.HardState.Term != 3 || len(r.Entries) != 1 || r.Entries[0].Index != 3 {
				t.Fatalf("lost checkpoint at %s: %+v", stage, r)
			}
			must(t, w.Append([]Entry{{Index: 4, Term: 3}}))
			must(t, w.Compact(2))
			must(t, w.Close())
			w = openTestWAL(t, dir)
			r, err = w.Recover()
			must(t, err)
			if len(r.Entries) != 2 {
				t.Fatal("post-crash append lost")
			}
		})
	}
}

func TestSnapshotRecoveryCompactionAndAppend(t *testing.T) {
	for _, retain := range []bool{false, true} {
		dir := t.TempDir()
		w := openTestWAL(t, dir)
		seedWAL(t, w)
		s := Snapshot{LastIncludedIndex: 2, LastIncludedTerm: 2, Configuration: []byte("peers"), Data: []byte("state")}
		must(t, w.InstallSnapshot(s, retain))
		s.Data[0] = '!'
		want, err := w.Recover()
		must(t, err)
		if string(want.Snapshot.Data) != "state" || (retain && len(want.Entries) != 1) || (!retain && len(want.Entries) != 0) {
			t.Fatal(want)
		}
		must(t, w.Close())
		w = openTestWAL(t, dir)
		got, err := w.Recover()
		must(t, err)
		if !reflect.DeepEqual(got, want) {
			t.Fatal("installation recovery differs")
		}
		before, err := os.Stat(filepath.Join(dir, "raft.wal"))
		must(t, err)
		must(t, w.Compact(2))
		after, err := os.Stat(filepath.Join(dir, "raft.wal"))
		must(t, err)
		if after.Size() >= before.Size() {
			t.Fatal("WAL did not shrink")
		}
		must(t, w.Close())
		w = openTestWAL(t, dir)
		got, err = w.Recover()
		must(t, err)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("compaction lost state: %+v", got)
		}
		got.Snapshot.Data[0] = '!'
		got.Snapshot.Configuration[0] = '!'
		got, err = w.Recover()
		must(t, err)
		if !reflect.DeepEqual(got, want) {
			t.Fatal("snapshot aliases recovery buffers")
		}
		must(t, w.Append([]Entry{{Index: 3, Term: 3, Data: []byte("new suffix")}}))
		if w.Append([]Entry{{Index: 2, Term: 3}}) == nil {
			t.Fatal("accepted entry covered by snapshot")
		}
		must(t, w.Close())
		w = openTestWAL(t, dir)
		got, err = w.Recover()
		must(t, err)
		if len(got.Entries) != 1 || string(got.Entries[0].Data) != "new suffix" {
			t.Fatal(got)
		}
	}
}

func TestSnapshotTornRecordEveryByteAndCorruption(t *testing.T) {
	dir := t.TempDir()
	w := openTestWAL(t, dir)
	seedWAL(t, w)
	old, err := os.ReadFile(filepath.Join(dir, "raft.wal"))
	must(t, err)
	must(t, w.InstallSnapshot(Snapshot{LastIncludedIndex: 2, LastIncludedTerm: 2, Data: []byte("snapshot")}, true))
	complete, err := os.ReadFile(filepath.Join(dir, "raft.wal"))
	must(t, err)
	for cut := len(old); cut < len(complete); cut++ {
		d := t.TempDir()
		must(t, os.WriteFile(filepath.Join(d, "raft.wal"), complete[:cut], 0600))
		rw := openTestWAL(t, d)
		r, err := rw.Recover()
		must(t, err)
		if r.Snapshot != nil || len(r.Entries) != 3 {
			t.Fatalf("mixed generation at cut %d", cut)
		}
		must(t, rw.Close())
	}
	bad := bytes.Clone(complete)
	bad[len(bad)-1] ^= 1
	d := t.TempDir()
	must(t, os.WriteFile(filepath.Join(d, "raft.wal"), bad, 0600))
	if rw, err := OpenWAL(d); err == nil {
		rw.Close()
		t.Fatal("accepted corrupt snapshot")
	}
}

func TestSnapshotInvalidBoundaryAndFailure(t *testing.T) {
	w := openTestWAL(t, t.TempDir())
	seedWAL(t, w)
	if w.InstallSnapshot(Snapshot{LastIncludedIndex: 2, LastIncludedTerm: 3}, true) == nil {
		t.Fatal("retained conflicting suffix")
	}
	must(t, w.InstallSnapshot(Snapshot{LastIncludedIndex: 2, LastIncludedTerm: 2}, true))
	if w.InstallSnapshot(Snapshot{LastIncludedIndex: 1, LastIncludedTerm: 1}, false) == nil {
		t.Fatal("snapshot regressed")
	}
	if w.Compact(3) == nil {
		t.Fatal("compacted above durable boundary")
	}
	w.file = faultyFile{walFile: w.file, syncFailure: true}
	if w.InstallSnapshot(Snapshot{LastIncludedIndex: 3, LastIncludedTerm: 3}, true) == nil {
		t.Fatal("acknowledged failed sync")
	}
	if w.Compact(2) == nil {
		t.Fatal("continued after uncertain write")
	}
}
