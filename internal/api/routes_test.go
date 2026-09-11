package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSkeletonRoutes(t *testing.T) {
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"PUT", "/kv/a", `{"value":""}`, 501},
		{"GET", "/kv/a", "", 501},
		{"DELETE", "/kv/a", "", 501},
		{"GET", "/cluster/status", "", 501},
		{"POST", "/cluster/join", `{"id":"n4","grpc_address":"n4:9004","http_address":"http://localhost:8004"}`, 501},
		{"POST", "/cluster/remove", `{"id":"n4"}`, 501},
		{"PUT", "/kv/a", `{}`, 400},
		{"PUT", "/kv/a", `{"value":null}`, 400},
		{"PUT", "/kv/a", `{"value":1}`, 400},
		{"PUT", "/kv/a", `{"value":"x","ttl":3}`, 400},
		{"PUT", "/kv/a", `{"value":"x"} {}`, 400},
		{"POST", "/cluster/join", `{}`, 400},
		{"POST", "/cluster/remove", `null`, 400},
		{"POST", "/kv/a", `{}`, 405},
		{"GET", "/unknown", "", 404},
	} {
		t.Run(tc.method+tc.path+tc.body, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			w := httptest.NewRecorder()
			NewHandler().ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
			if tc.status == 501 || tc.status == 400 {
				var body ErrorResponse
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Error == "" {
					t.Fatalf("invalid error envelope: %s", w.Body.String())
				}
			}
		})
	}
}
