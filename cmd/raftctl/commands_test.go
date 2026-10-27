package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestKVCommandsFollowLeaderPreservingRequest(t *testing.T) {
	var method, path, body string
	leader := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.EscapedPath()
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer leader.Close()
	follower := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", leader.URL+r.URL.EscapedPath())
		w.WriteHeader(307)
	}))
	defer follower.Close()
	for _, tc := range []struct {
		args         []string
		method, body string
	}{
		{[]string{"put", "a/b", "hello"}, "PUT", `{"value":"hello"}`}, {[]string{"get", "a/b"}, "GET", ""}, {[]string{"delete", "a/b"}, "DELETE", ""},
	} {
		var out bytes.Buffer
		if err := run(append([]string{"--endpoint", follower.URL}, tc.args...), &out); err != nil {
			t.Fatal(err)
		}
		if method != tc.method || path != "/kv/a%2Fb" || body != tc.body {
			t.Fatalf("%s %s %s", method, path, body)
		}
	}
}
func TestCLIRedirectLoopIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/kv/key")
		w.WriteHeader(307)
	}))
	defer server.Close()
	if err := run([]string{"--endpoint", server.URL, "get", "key"}, io.Discard); err == nil {
		t.Fatal("redirect loop accepted")
	}
}
