package state

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

func TestSnapshotRoundTripAndReplacement(t *testing.T) {
	for _, size := range []int{0, 3, 5000} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			s := NewStore()
			for i := 0; i < size; i++ {
				s.put(fmt.Sprint(i), fmt.Sprintf("value\x00雪%d", i))
			}
			data, err := s.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			dest := NewStore()
			dest.put("stale", "remove")
			if err := dest.Restore(data); err != nil {
				t.Fatal(err)
			}
			if _, found := dest.get("stale"); found {
				t.Fatal("restore merged state")
			}
			after, _ := dest.Snapshot()
			if !bytes.Equal(data, after) {
				t.Fatal("round trip differs")
			}
			if size == 0 && string(data) != `{"version":1,"data":{}}` {
				t.Fatalf("empty format: %s", data)
			}
			data[0] = 'x'
			again, _ := dest.Snapshot()
			if !bytes.Equal(after, again) {
				t.Fatal("restore retained caller bytes")
			}
		})
	}
}

func TestInvalidRestoreLeavesStateIntact(t *testing.T) {
	s := NewStore()
	s.put("keep", "value")
	before, _ := s.Snapshot()
	for _, data := range []string{"", "null", `{}`, `{"version":2,"data":{}}`, `{"version":1,"data":null}`, `{"version":1,"data":{"":"bad"}}`, `{"version":1,"data":{"a":null}}`, `{"version":1,"data":{"a":1}}`, `{"version":1,"data":{},"term":1}`, `{"version":1,"data":{}} []`, "\xff"} {
		if err := s.Restore([]byte(data)); !errors.Is(err, ErrInvalidSnapshot) {
			t.Fatalf("%q: %v", data, err)
		}
		after, _ := s.Snapshot()
		if !bytes.Equal(before, after) {
			t.Fatalf("%q changed state", data)
		}
	}
}
