package demo

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
)

// LocalNodes is an opt-in local process supervisor, not part of Raft. Only
// children started by this supervisor can be killed. Membership uses normal APIs.
type LocalNodes struct {
	mu              sync.Mutex
	exe, root, seed string
	lock            *os.File
	records         []localRecord
	children        map[string]*localChild
}
type localRecord struct {
	Peer    raft.Peer `json:"peer"`
	Enabled bool      `json:"enabled"`
}
type localChild struct {
	cmd  *exec.Cmd
	done chan struct{}
}

func OpenLocalNodes(exe, seed, root string) (*LocalNodes, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(root, "manager.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("local node directory already in use: %w", err)
	}
	m := &LocalNodes{exe: exe, root: root, seed: filepath.Join(root, "seed.json"), lock: f, children: map[string]*localChild{}}
	ok := false
	defer func() {
		if !ok {
			m.Close()
		}
	}()
	data, err := os.ReadFile(seed)
	if err != nil {
		return nil, err
	}
	old, err := os.ReadFile(m.seed)
	if os.IsNotExist(err) {
		err = os.WriteFile(m.seed, data, 0600)
	} else if err == nil && string(old) != string(data) {
		err = fmt.Errorf("managed seed configuration changed")
	}
	if err != nil {
		return nil, err
	}
	data, err = os.ReadFile(filepath.Join(root, "nodes.json"))
	if err == nil {
		err = json.Unmarshal(data, &m.records)
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	seen := map[string]bool{}
	for _, r := range m.records {
		id := strings.TrimPrefix(r.Peer.ID, "local-")
		if b, e := hex.DecodeString(id); e != nil || len(b) != 8 || r.Peer.ID != "local-"+id || seen[id] {
			return nil, fmt.Errorf("invalid managed node identity")
		}
		seen[id] = true
		for _, address := range []string{strings.TrimPrefix(r.Peer.HTTPAddress, "http://"), r.Peer.GRPCAddress} {
			host, port, e := net.SplitHostPort(address)
			n, pe := strconv.Atoi(port)
			if e != nil || pe != nil || n < 1 || n > 65535 || host != "127.0.0.1" || !strings.HasPrefix(r.Peer.HTTPAddress, "http://") {
				return nil, fmt.Errorf("invalid managed loopback endpoint")
			}
		}
		if r.Enabled {
			if err = m.startLocked(r.Peer); err != nil {
				return nil, err
			}
		}
	}
	ok = true
	return m, nil
}
func (m *LocalNodes) saveLocked() error {
	data, err := json.MarshalIndent(m.records, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(m.root, "nodes.tmp"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(filepath.Join(m.root, "nodes.tmp"), filepath.Join(m.root, "nodes.json")); err != nil {
		return err
	}
	dir, err := os.Open(m.root)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func (m *LocalNodes) Peers() []raft.Peer {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]raft.Peer, 0, len(m.records))
	for _, r := range m.records {
		out = append(out, r.Peer)
	}
	return out
}
func (m *LocalNodes) startLocked(p raft.Peer) error {
	if c := m.children[p.ID]; c != nil {
		select {
		case <-c.done:
		default:
			return fmt.Errorf("node already running")
		}
	}
	log, err := os.OpenFile(filepath.Join(m.root, p.ID+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.Command(m.exe, "--join", "--node-id", p.ID, "--cluster-config", m.seed, "--data-dir", filepath.Join(m.root, p.ID), "--http-addr", strings.TrimPrefix(p.HTTPAddress, "http://"), "--grpc-addr", p.GRPCAddress)
	cmd.Stdout = log
	cmd.Stderr = log
	if err = cmd.Start(); err != nil {
		return err
	}
	c := &localChild{cmd: cmd, done: make(chan struct{})}
	m.children[p.ID] = c
	go func() { _ = cmd.Wait(); close(c.done) }()
	client := http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-c.done:
			return fmt.Errorf("node exited; check %s.log (port may be occupied)", p.ID)
		default:
		}
		resp, e := client.Get(p.HTTPAddress + "/cluster/status")
		if e == nil {
			var s raft.NodeStatus
			e = json.NewDecoder(resp.Body).Decode(&s)
			resp.Body.Close()
			if e == nil && s.ID == p.ID {
				return nil
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	<-c.done
	return fmt.Errorf("node startup timed out; inspect its log and restart")
}
func (m *LocalNodes) Create() (raft.Peer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var id [8]byte
	if _, err := rand.Read(id[:]); err != nil {
		return raft.Peer{}, err
	}
	h, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return raft.Peer{}, err
	}
	defer h.Close()
	g, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return raft.Peer{}, err
	}
	defer g.Close()
	p := raft.Peer{ID: "local-" + hex.EncodeToString(id[:]), HTTPAddress: "http://" + h.Addr().String(), GRPCAddress: g.Addr().String(), Learner: true}
	m.records = append(m.records, localRecord{Peer: p, Enabled: true})
	if err = m.saveLocked(); err != nil {
		m.records = m.records[:len(m.records)-1]
		return raft.Peer{}, err
	}
	// The child owns its sockets. A collision in this hand-off fails visibly;
	// never substitute different ports for an identity already persisted.
	h.Close()
	g.Close()
	err = m.startLocked(p)
	return p, err
}
func (m *LocalNodes) Action(id, action string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, r := range m.records {
		if r.Peer.ID == id {
			switch action {
			case "crash":
				c := m.children[id]
				if c == nil {
					return fmt.Errorf("process not owned by this console")
				}
				m.records[i].Enabled = false
				if err := m.saveLocked(); err != nil {
					m.records[i].Enabled = r.Enabled
					return err
				}
				select {
				case <-c.done:
					return nil
				default:
				}
				if err := c.cmd.Process.Kill(); err != nil {
					return err
				}
				<-c.done
				return nil
			case "restart":
				m.records[i].Enabled = true
				if err := m.saveLocked(); err != nil {
					m.records[i].Enabled = r.Enabled
					return err
				}
				return m.startLocked(r.Peer)
			default:
				return fmt.Errorf("unknown process action")
			}
		}
	}
	return fmt.Errorf("only console-created nodes can be controlled")
}
func (m *LocalNodes) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.children {
		select {
		case <-c.done:
			continue
		default:
		}
		_ = c.cmd.Process.Kill()
		<-c.done
	}
	if m.lock != nil {
		_ = m.lock.Close()
		m.lock = nil
	}
}

func (m *LocalNodes) routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /demo/api/cluster/process", func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil || (host != "127.0.0.1" && host != "localhost") {
			fail(w, 403, fmt.Errorf("local console host required"))
			return
		}
		var in struct {
			Action string `json:"action"`
			ID     string `json:"id"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			fail(w, 400, err)
			return
		}
		if dec.Decode(new(any)) != io.EOF {
			fail(w, 400, fmt.Errorf("expected one JSON object"))
			return
		}
		if in.Action == "create" {
			p, err := m.Create()
			if err != nil {
				fail(w, 409, err)
				return
			}
			data, _ := json.Marshal(p)
			reply(w, 201, data)
			return
		}
		if err := m.Action(in.ID, in.Action); err != nil {
			fail(w, 409, err)
			return
		}
		reply(w, 200, []byte(`{"ok":true}`))
	})
}
