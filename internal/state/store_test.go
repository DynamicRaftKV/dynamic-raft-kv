package state

import (
	"fmt"
	"sync"
	"testing"
)

func TestCRUD(t *testing.T) {
	var s Store
	if _, found := s.get("a"); found {
		t.Fatal("missing key found")
	}
	s.put("a", "one")
	s.put("a", "")
	if v, found := s.get("a"); !found || v != "" {
		t.Fatalf("empty overwrite: %q %v", v, found)
	}
	s.delete("a")
	s.delete("a")
	if _, found := s.get("a"); found {
		t.Fatal("deleted key found")
	}
}

func TestConcurrentOperationsAndSnapshots(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprint(id)
			for i := 0; i < 100; i++ {
				s.put(key, "value")
				s.get(key)
				command, _ := EncodeCommand(Command{Op: OpPut, Key: key, Value: "applied"})
				if _, err := s.Apply(command); err != nil {
					t.Error(err)
				}
				data, err := s.Snapshot()
				if err != nil {
					t.Error(err)
					return
				}
				if err := NewStore().Restore(data); err != nil {
					t.Error(err)
				}
				if i%10 == 0 {
					if err := s.Restore(data); err != nil {
						t.Error(err)
					}
				}
				s.delete(key)
			}
		}(worker)
	}
	wg.Wait()
}
