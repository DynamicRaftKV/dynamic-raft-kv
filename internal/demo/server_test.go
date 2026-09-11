package demo

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
)

func TestDemoUsesRealEngineAndRestores(t *testing.T) {
	h := NewHandler(state.NewStore())
	call := func(method, path, body string, want int) []byte {
		t.Helper()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, bytes.NewBufferString(body)))
		if w.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body)
		}
		return w.Body.Bytes()
	}
	call("GET", "/demo/", "", 200)
	call("GET", "/demo/style.css", "", 200)
	call("GET", "/demo/app.js", "", 200)
	call("POST", "/demo/api/command", `{"op":"PUT","key":"demo","value":"before"}`, 200)
	snapshot := call("GET", "/demo/api/snapshot", "", 200)
	call("POST", "/demo/api/command", `{"op":"PUT","key":"demo","value":"after"}`, 200)
	call("POST", "/demo/api/restore", string(snapshot), 200)
	result := call("POST", "/demo/api/command", `{"op":"GET","key":"demo"}`, 200)
	var got state.Result
	if err := json.Unmarshal(result, &got); err != nil || got.Value != "before" || !got.Found {
		t.Fatalf("restore result %s: %v", result, err)
	}
	call("POST", "/demo/api/command", `{"op":"CAS","key":"demo"}`, 400)
	call("POST", "/demo/api/restore", `{"version":2,"data":{}}`, 400)
	view := call("GET", "/demo/api/state", "", 200)
	var status struct {
		Commands int `json:"commands"`
		Snapshot struct {
			Data map[string]string `json:"data"`
		} `json:"snapshot"`
	}
	if err := json.Unmarshal(view, &status); err != nil {
		t.Fatal(err)
	}
	if status.Commands != 3 || status.Snapshot.Data["demo"] != "before" {
		t.Fatalf("unexpected state: %s", view)
	}
}

func TestDemoRejectsCrossOriginMutation(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "http://localhost/demo/api/command", bytes.NewBufferString(`{"op":"PUT","key":"a"}`))
	r.Header.Set("Origin", "https://unrelated.example")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	NewHandler(state.NewStore()).ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin mutation: %d", w.Code)
	}
}
