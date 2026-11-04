package demo

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
)

func TestClusterBridge(t *testing.T) {
	var path, method, body string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.EscapedPath()
		method = r.Method
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()
	h := NewClusterHandler(state.NewStore(), []raft.Peer{{ID: "one", HTTPAddress: target.URL}, {ID: "two", HTTPAddress: "http://127.0.0.1:9999", GRPCAddress: "127.0.0.1:9998"}})
	for _, tc := range []struct {
		data string
		code int
	}{
		{`{"node":"one","operation":"PUT","key":"a/b","value":"value"}`, 200},
		{`{"node":"http://other","operation":"GET","key":"a"}`, 400},
		{`{"node":"one","operation":"GET","key":".."}`, 400},
		{`{"node":"one","operation":"join","member":"unknown"}`, 400},
		{`{"node":"one","operation":"shell"}`, 400},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/demo/api/cluster/command", strings.NewReader(tc.data)))
		if w.Code != tc.code {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if path != "/kv/a%2Fb" || method != "PUT" || body != `{"value":"value"}` {
		t.Fatal(path, method, body)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/demo/api/cluster/command", strings.NewReader(`{"node":"one","operation":"remove","member":"two"}`))
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-site mutation accepted", w.Code)
	}
}
