package state

import "sync"

// Store is an in-memory KV engine. Its zero value is ready to use.
// Do not copy a Store after first use. Raft callers must serialize Apply in
// committed log order even though individual operations are concurrency-safe.
type Store struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewStore() *Store { return &Store{} }

func (s *Store) put(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = make(map[string]string)
	}
	s.data[key] = value
}

func (s *Store) get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, found := s.data[key]
	return value, found
}

func (s *Store) delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

var _ StateMachine = (*Store)(nil)
