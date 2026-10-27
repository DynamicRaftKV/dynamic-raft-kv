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
)

func TestDurableNodeProcess(t *testing.T) {
	configPath := os.Getenv("RAFTKV_PROCESS_TEST_CONFIG")
	if configPath == "" {
		return
	}
	c := config{id: os.Getenv("RAFTKV_PROCESS_TEST_ID"), dataDir: os.Getenv("RAFTKV_PROCESS_TEST_DATA"), httpAddr: os.Getenv("RAFTKV_PROCESS_TEST_HTTP"), grpcAddr: os.Getenv("RAFTKV_PROCESS_TEST_GRPC"), clusterConfig: configPath}
	if err := serve(context.Background(), c); err != nil {
		t.Fatal(err)
	}
}

func TestDurableClusterLeaderKillAndFullRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	root := t.TempDir()
	peers := make([]raft.Peer, 3)
	httpAddresses := make([]string, 3)
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
	data, _ := json.Marshal(clusterFile{Peers: peers})
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	commands := make([]*exec.Cmd, 3)
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
	for i := range commands {
		start(i)
	}
	first := leader()
	put(first, "before-kill", "retained")
	stop(first)
	second := leader()
	get(second, "before-kill", "retained")
	put(second, "after-kill", "also-retained")
	for i := range commands {
		stop(i)
	}
	for i := range commands {
		start(i)
	}
	recovered := leader()
	get(recovered, "before-kill", "retained")
	get(recovered, "after-kill", "also-retained")
	put(recovered, "after-restart", "healthy")
}
