package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
)

func TestDurableNodeProcess(t *testing.T) {
	configPath := os.Getenv("RAFTKV_PROCESS_TEST_CONFIG")
	if configPath == "" {
		return
	}
	c := config{id: os.Getenv("RAFTKV_PROCESS_TEST_ID"), dataDir: os.Getenv("RAFTKV_PROCESS_TEST_DATA"), httpAddr: os.Getenv("RAFTKV_PROCESS_TEST_HTTP"), grpcAddr: os.Getenv("RAFTKV_PROCESS_TEST_GRPC"), clusterConfig: configPath}
	c.snapshotEntries = 2
	c.joining = os.Getenv("RAFTKV_PROCESS_TEST_JOIN") == "true"
	if err := serve(context.Background(), c); err != nil {
		t.Fatal(err)
	}
}

func TestDurableClusterLeaderKillAndFullRestart(t *testing.T) {
	runDurableCluster(t, false)
}

func TestDynamicMembershipProcessJoinPromoteAndRestart(t *testing.T) {
	runDurableCluster(t, true)
}

func runDurableCluster(t *testing.T, dynamic bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	root := t.TempDir()
	count := 3
	if dynamic {
		count = 4
	}
	peers := make([]raft.Peer, count)
	httpAddresses := make([]string, count)
	var reservations []net.Listener
	for i := range peers {
		h, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		g, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			h.Close()
			t.Fatal(err)
		}
		reservations = append(reservations, h, g)
		httpAddresses[i] = h.Addr().String()
		peers[i] = raft.Peer{ID: fmt.Sprintf("node%d", i), GRPCAddress: g.Addr().String(), HTTPAddress: "http://" + h.Addr().String()}
	}
	for _, l := range reservations {
		l.Close()
	}
	file := filepath.Join(root, "cluster.json")
	data, _ := json.Marshal(clusterFile{Peers: peers[:3]})
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	commands := make([]*exec.Cmd, count)
	stop := func(i int) {
		if commands[i] != nil {
			_ = commands[i].Process.Kill()
			_ = commands[i].Wait()
			commands[i] = nil
		}
	}
	defer func() {
		for i := range commands {
			stop(i)
		}
	}()
	start := func(i int) {
		t.Helper()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDurableNodeProcess$")
		cmd.Env = append(os.Environ(), "RAFTKV_PROCESS_TEST_CONFIG="+file, "RAFTKV_PROCESS_TEST_ID="+peers[i].ID, "RAFTKV_PROCESS_TEST_DATA="+filepath.Join(root, peers[i].ID), "RAFTKV_PROCESS_TEST_HTTP="+httpAddresses[i], "RAFTKV_PROCESS_TEST_GRPC="+peers[i].GRPCAddress)
		if i == 3 {
			cmd.Env = append(cmd.Env, "RAFTKV_PROCESS_TEST_JOIN=true")
		}
		logFile, err := os.OpenFile(filepath.Join(root, peers[i].ID+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		defer logFile.Close()
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		commands[i] = cmd
	}
	client := &http.Client{Timeout: 3 * time.Second}
	leader := func() int {
		t.Helper()
		for {
			for i, p := range peers {
				if commands[i] == nil {
					continue
				}
				resp, err := client.Get(p.HTTPAddress + "/cluster/status")
				if err != nil {
					continue
				}
				var s raft.NodeStatus
				err = json.NewDecoder(resp.Body).Decode(&s)
				resp.Body.Close()
				if err == nil && s.Role == raft.Leader {
					return i
				}
			}
			select {
			case <-ctx.Done():
				for _, p := range peers {
					log, _ := os.ReadFile(filepath.Join(root, p.ID+".log"))
					t.Logf("%s: %s", p.ID, log)
				}
				t.Fatal("no leader before deadline")
				return -1
			case <-time.After(30 * time.Millisecond):
			}
		}
	}
	put := func(i int, key, value string) {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"value": value})
		req, _ := http.NewRequest(http.MethodPut, peers[i].HTTPAddress+"/kv/"+key, bytes.NewReader(body))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			data, _ := io.ReadAll(resp.Body)
			t.Fatalf("put %s: %s %s", key, resp.Status, data)
		}
	}
	get := func(i int, key, want string) {
		t.Helper()
		resp, err := client.Get(peers[i].HTTPAddress + "/kv/" + key)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var result state.Result
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 || !result.Found || result.Value != want {
			t.Fatalf("get %s: %s %+v", key, resp.Status, result)
		}
	}
	for i := 0; i < 3; i++ {
		start(i)
	}
	first := leader()
	put(first, "before-kill", "retained")
	if dynamic {
		start(3)
		post := func(path string, body any, allowConflict bool) bool {
			t.Helper()
			data, _ := json.Marshal(body)
			resp, err := client.Post(peers[first].HTTPAddress+path, "application/json", bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if allowConflict && resp.StatusCode == 409 {
				return false
			}
			if resp.StatusCode != 202 {
				data, _ := io.ReadAll(resp.Body)
				t.Fatalf("%s: %s %s", path, resp.Status, data)
			}
			return true
		}
		post("/cluster/join", map[string]string{"id": peers[3].ID, "grpc_address": peers[3].GRPCAddress, "http_address": peers[3].HTTPAddress}, false)
		for !post("/cluster/promote", map[string]string{"id": peers[3].ID}, true) {
			select {
			case <-ctx.Done():
				t.Fatal("promotion timeout")
			case <-time.After(30 * time.Millisecond):
			}
		}
		// Every live node must apply the finalized four-voter configuration
		// before hard killing the leader or restarting the whole cluster.
		for {
			ready := true
			for _, p := range peers {
				resp, err := client.Get(p.HTTPAddress + "/cluster/status")
				if err != nil {
					ready = false
					continue
				}
				var status raft.NodeStatus
				err = json.NewDecoder(resp.Body).Decode(&status)
				resp.Body.Close()
				if err != nil || status.MembershipPending || len(status.Peers) != 4 || status.LastApplied < status.ConfigurationIndex {
					ready = false
					continue
				}
				for _, p := range status.Peers {
					if p.Learner {
						ready = false
					}
				}
			}
			if ready {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("membership finalization timeout")
			case <-time.After(30 * time.Millisecond):
			}
		}
		get(3, "before-kill", "retained")
	}
	stop(first)
	second := leader()
	get(second, "before-kill", "retained")
	put(second, "after-kill", "also-retained")
	for i := range commands {
		stop(i)
	}
	// The committed checkpoint must survive a hard process kill without Close.
	disk, err := storage.OpenWAL(filepath.Join(root, peers[second].ID))
	if err != nil {
		t.Fatal(err)
	}
	recovery, err := disk.Recover()
	disk.Close()
	if err != nil || recovery.Snapshot == nil {
		t.Fatal("missing durable process snapshot", err)
	}
	for i := range commands {
		start(i)
	}
	recovered := leader()
	if dynamic {
		resp, err := client.Get(peers[recovered].HTTPAddress + "/cluster/status")
		if err != nil {
			t.Fatal(err)
		}
		var status raft.NodeStatus
		err = json.NewDecoder(resp.Body).Decode(&status)
		resp.Body.Close()
		if err != nil || len(status.Peers) != 4 || status.MembershipPending {
			t.Fatal("recovered wrong membership", status, err)
		}
	}
	get(recovered, "before-kill", "retained")
	get(recovered, "after-kill", "also-retained")
	put(recovered, "after-restart", "healthy")
}
