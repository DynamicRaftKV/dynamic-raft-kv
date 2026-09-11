package api

import (
	"encoding/json"
	"io"
	"net/http"
)

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

func unimplemented(w http.ResponseWriter, _ *http.Request) {
	respondError(w, http.StatusNotImplemented, "not implemented")
}

// decode rejects unknown fields and multiple JSON values. Domain policy beyond
// required fields remains pending; this is schema parsing, not KV validation.
func decode(r *http.Request, out any) bool {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return false
	}
	return d.Decode(new(any)) == io.EOF
}

func put(w http.ResponseWriter, r *http.Request) {
	var body PutRequest
	if !decode(r, &body) || body.Value == nil {
		respondError(w, http.StatusBadRequest, "expected JSON object with string value")
		return
	}
	unimplemented(w, r)
}

func join(w http.ResponseWriter, r *http.Request) {
	var body JoinRequest
	if !decode(r, &body) || body.ID == "" || body.GRPCAddress == "" || body.HTTPAddress == "" {
		respondError(w, http.StatusBadRequest, "id, grpc_address, and http_address are required")
		return
	}
	unimplemented(w, r)
}

func remove(w http.ResponseWriter, r *http.Request) {
	var body RemoveRequest
	if !decode(r, &body) || body.ID == "" {
		respondError(w, http.StatusBadRequest, "id is required")
		return
	}
	unimplemented(w, r)
}
