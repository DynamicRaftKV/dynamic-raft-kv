package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrCorrupt = errors.New("storage: corrupt WAL")
	ErrLocked  = errors.New("storage: data directory already in use")
	ErrClosed  = errors.New("storage: WAL closed")
)

const walMagic = "RAFTKV01"
const maxRecordBytes = 64 << 20

var crcTable = crc32.MakeTable(crc32.Castagnoli)

type walRecord struct {
	Type     string     `json:"type"`
	Hard     *HardState `json:"hard_state,omitempty"`
	Entries  []Entry    `json:"entries,omitempty"`
	Snapshot *Snapshot  `json:"snapshot,omitempty"`
	Retain   bool       `json:"retain_suffix,omitempty"`
}

type walFile interface {
	io.Reader
	io.Writer
	io.Seeker
	Sync() error
	Truncate(int64) error
	Close() error
}

// WAL synchronously syncs every accepted operation before publishing its memory
// view. An I/O error poisons the handle: reopen/recover before any further work.
// One process owns a directory. Snapshots publish as a single WAL record;
// compaction atomically replaces the WAL with a checkpoint and retained suffix.
type WAL struct {
	mu             sync.Mutex
	file           walFile
	lock           *os.File
	state          Recovered
	failed         error
	closed         bool
	dir            string
	checkpointHook func(string)  // package tests observe crash boundaries
	checkpointIO   *checkpointIO // nil uses OS operations; package tests inject faults
}

var _ Storage = (*WAL)(nil)

// OpenWAL acquires exclusive ownership and recovers checksummed records. Only
// an incomplete final record is discarded; a checksum/semantic error is fatal.
func OpenWAL(dir string) (*WAL, error) {
	if dir == "" {
		return nil, errors.New("storage: data directory required")
	}
	if err := durableMkdir(dir); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "LOCK"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := lockFile(lock); err != nil {
		lock.Close()
		return nil, err
	}
	w := &WAL{lock: lock, dir: dir}
	ok := false
	defer func() {
		if !ok {
			w.Close()
		}
	}()
	name := filepath.Join(dir, "raft.wal")
	f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	fresh := err == nil
	if errors.Is(err, os.ErrExist) {
		f, err = os.OpenFile(name, os.O_RDWR, 0600)
	}
	if err != nil {
		return nil, err
	}
	w.file = f
	if fresh {
		if err := writeAll(f, []byte(walMagic)); err != nil {
			return nil, err
		}
		if err := f.Sync(); err != nil {
			return nil, err
		}
		if err := syncDirectory(dir); err != nil {
			return nil, err
		}
	} else if err := w.replay(); err != nil {
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return nil, err
	}
	ok = true
	return w, nil
}

// Sync newly created directory entries from the deepest directory to the first
// existing ancestor, so an acknowledged WAL cannot lose its parent on restart.
func durableMkdir(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	var missing []string
	for p := absolute; ; p = filepath.Dir(p) {
		info, err := os.Stat(p)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("storage: %s is not a directory", p)
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		missing = append(missing, p)
	}
	if err := os.MkdirAll(absolute, 0700); err != nil {
		return err
	}
	for _, p := range missing {
		if err := syncDirectory(filepath.Dir(p)); err != nil {
			return err
		}
	}
	return nil
}
func syncDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
func cloneEntries(entries []Entry) []Entry {
	out := append([]Entry(nil), entries...)
	for i := range out {
		out[i].Data = bytes.Clone(out[i].Data)
	}
	return out
}
func (w *WAL) usable() error {
	if w.closed {
		return ErrClosed
	}
	return w.failed
}
func (w *WAL) poison(err error) error {
	w.failed = fmt.Errorf("storage: WAL I/O failed: %w", err)
	return w.failed
}

func (w *WAL) replay() error {
	magic := make([]byte, len(walMagic))
	if _, err := io.ReadFull(w.file, magic); err != nil || string(magic) != walMagic {
		return fmt.Errorf("%w: invalid file header", ErrCorrupt)
	}
	offset := int64(len(walMagic))
	for {
		header := make([]byte, 12)
		n, err := io.ReadFull(w.file, header)
		if err == io.EOF && n == 0 {
			return nil
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return w.trimTail(offset)
		}
		if err != nil {
			return err
		}
		if crc32.Checksum(header[:8], crcTable) != binary.LittleEndian.Uint32(header[8:]) {
			return fmt.Errorf("%w: record header at %d", ErrCorrupt, offset)
		}
		length := binary.LittleEndian.Uint32(header[:4])
		if length == 0 || length > maxRecordBytes {
			return fmt.Errorf("%w: record length at %d", ErrCorrupt, offset)
		}
		data := make([]byte, int(length))
		_, err = io.ReadFull(w.file, data)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return w.trimTail(offset)
		}
		if err != nil {
			return err
		}
		if crc32.Checksum(data, crcTable) != binary.LittleEndian.Uint32(header[4:8]) {
			return fmt.Errorf("%w: record payload at %d", ErrCorrupt, offset)
		}
		var record walRecord
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&record); err != nil {
			return fmt.Errorf("%w: record JSON at %d: %v", ErrCorrupt, offset, err)
		}
		if dec.Decode(new(any)) != io.EOF {
			return fmt.Errorf("%w: trailing record data", ErrCorrupt)
		}
		if err := w.validate(record); err != nil {
			return fmt.Errorf("%w at %d: %v", ErrCorrupt, offset, err)
		}
		w.apply(record)
		offset += 12 + int64(length)
	}
}
func (w *WAL) trimTail(offset int64) error {
	if err := w.file.Truncate(offset); err != nil {
		return err
	}
	return w.file.Sync()
}
func (w *WAL) validate(r walRecord) error {
	if r.Type != "snapshot" && (r.Snapshot != nil || r.Retain) {
		return errors.New("unexpected snapshot fields")
	}
	switch r.Type {
	case "snapshot":
		if r.Snapshot == nil || r.Hard != nil || len(r.Entries) != 0 {
			return errors.New("invalid snapshot record")
		}
		s := r.Snapshot
		if s.LastIncludedIndex == 0 || s.LastIncludedIndex == ^uint64(0) || s.LastIncludedTerm == 0 || s.LastIncludedTerm > w.state.HardState.Term || len(s.Data) > MaxSnapshotBytes || len(s.Configuration) > 1<<20 {
			return errors.New("invalid snapshot boundary or size")
		}
		base, term := w.boundary()
		if s.LastIncludedIndex <= base || s.LastIncludedTerm < term {
			return errors.New("snapshot regression")
		}
		if r.Retain {
			i := s.LastIncludedIndex - base
			if i > uint64(len(w.state.Entries)) || w.state.Entries[i-1].Term != s.LastIncludedTerm {
				return errors.New("snapshot retention boundary mismatch")
			}
		}
	case "hard_state":
		if r.Hard == nil || len(r.Entries) != 0 {
			return errors.New("invalid hard-state record")
		}
		h, old := *r.Hard, w.state.HardState
		if h.Term < old.Term || (h.Term == 0 && h.VotedFor != "") || (h.Term == old.Term && old.VotedFor != "" && h.VotedFor != old.VotedFor) {
			return errors.New("term/vote regression")
		}
	case "append":
		if r.Hard != nil || len(r.Entries) == 0 {
			return errors.New("invalid append record")
		}
		start := r.Entries[0].Index
		base, previousTerm := w.boundary()
		if start <= base || start > base+uint64(len(w.state.Entries))+1 {
			return errors.New("noncontiguous append")
		}
		if start > base+1 {
			previousTerm = w.state.Entries[start-base-2].Term
		}
		for i, e := range r.Entries {
			if e.Index != start+uint64(i) || e.Term == 0 || e.Term < previousTerm || e.Term > w.state.HardState.Term || e.Kind > EntryConfiguration {
				return errors.New("invalid log entry")
			}
			previousTerm = e.Term
		}
	default:
		return errors.New("unknown record type")
	}
	return nil
}
func (w *WAL) apply(r walRecord) {
	if r.Hard != nil {
		w.state.HardState = *r.Hard
	} else if r.Snapshot != nil {
		base, _ := w.boundary()
		if r.Retain {
			w.state.Entries = cloneEntries(w.state.Entries[r.Snapshot.LastIncludedIndex-base:])
		} else {
			w.state.Entries = nil
		}
		w.state.Snapshot = cloneSnapshot(r.Snapshot)
	} else {
		base, _ := w.boundary()
		w.state.Entries = append(w.state.Entries[:r.Entries[0].Index-base-1], cloneEntries(r.Entries)...)
	}
}
func (w *WAL) persist(r walRecord) error {
	if err := w.usable(); err != nil {
		return err
	}
	if err := w.validate(r); err != nil {
		return err
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if len(data) > maxRecordBytes {
		return errors.New("storage: WAL record exceeds 64 MiB")
	}
	frame := make([]byte, 12+len(data))
	binary.LittleEndian.PutUint32(frame[:4], uint32(len(data)))
	binary.LittleEndian.PutUint32(frame[4:8], crc32.Checksum(data, crcTable))
	binary.LittleEndian.PutUint32(frame[8:12], crc32.Checksum(frame[:8], crcTable))
	copy(frame[12:], data)
	if err := writeAll(w.file, frame); err != nil {
		return w.poison(err)
	}
	if err := w.file.Sync(); err != nil {
		return w.poison(err)
	}
	w.apply(r)
	return nil
}
func (w *WAL) Recover() (Recovered, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.usable(); err != nil {
		return Recovered{}, err
	}
	return Recovered{HardState: w.state.HardState, Snapshot: cloneSnapshot(w.state.Snapshot), Entries: cloneEntries(w.state.Entries)}, nil
}
func (w *WAL) SaveHardState(h HardState) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.persist(walRecord{Type: "hard_state", Hard: &h})
}
func (w *WAL) Append(entries []Entry) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.usable(); err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	return w.persist(walRecord{Type: "append", Entries: entries})
}
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	var errs []error
	if w.file != nil {
		errs = append(errs, w.file.Close())
	}
	if w.lock != nil {
		errs = append(errs, w.lock.Close())
	}
	return errors.Join(errs...)
}
