package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"
)

const snapshotVersion = 1

var ErrInvalidSnapshot = errors.New("invalid snapshot")

// Snapshot metadata here is only a format version, never Raft metadata.
type snapshotPayload struct {
	Version int               `json:"version"`
	Data    map[string]string `json:"data"`
}

// Snapshot serializes one consistent map view. JSON sorts map keys, giving a
// stable representation. The read lock prevents writes throughout encoding.
func (s *Store) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data := s.data
	if data == nil {
		data = map[string]string{}
	}
	return json.Marshal(snapshotPayload{Version: snapshotVersion, Data: data})
}

// Restore validates into a fresh map before atomically publishing it.
func (s *Store) Restore(data []byte) error {
	var payload snapshotPayload
	if err := decodeJSON(data, &payload); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSnapshot, err)
	}
	if payload.Version != snapshotVersion || payload.Data == nil {
		return fmt.Errorf("%w: version 1 and a data object required", ErrInvalidSnapshot)
	}
	for key, value := range payload.Data {
		if key == "" || !utf8.ValidString(key) || !utf8.ValidString(value) {
			return fmt.Errorf("%w: invalid key/value", ErrInvalidSnapshot)
		}
	}
	s.mu.Lock()
	s.data = payload.Data
	s.mu.Unlock()
	return nil
}
