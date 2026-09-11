package state

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

func TestReplayAgainstModel(t *testing.T) {
	for seed := int64(0); seed < 100; seed++ {
		r := rand.New(rand.NewSource(seed))
		a, b := NewStore(), NewStore()
		model := map[string]string{}
		for step := 0; step < 200; step++ {
			c := Command{Op: []OpType{OpPut, OpGet, OpDelete}[r.Intn(3)], Key: fmt.Sprint(r.Intn(20))}
			want := Result{Op: c.Op}
			switch c.Op {
			case OpPut:
				c.Value = []string{"", "text", "雪\x00"}[r.Intn(3)]
				model[c.Key] = c.Value
			case OpGet:
				want.Value, want.Found = model[c.Key]
			case OpDelete:
				delete(model, c.Key)
			}
			for _, s := range []*Store{a, b} {
				if got := applyForTest(t, s, c); got != want {
					t.Fatalf("seed %d step %d: %+v != %+v", seed, step, got, want)
				}
			}
			if step%25 == 0 {
				data, _ := b.Snapshot()
				b = NewStore()
				if err := b.Restore(data); err != nil {
					t.Fatal(err)
				}
			}
		}
		x, _ := a.Snapshot()
		y, _ := b.Snapshot()
		if !bytes.Equal(x, y) {
			t.Fatalf("seed %d: replay differs", seed)
		}
		if len(a.data) != len(model) {
			t.Fatal("model state size differs")
		}
		for k, v := range model {
			if got, found := a.get(k); !found || got != v {
				t.Fatal("model state differs")
			}
		}
	}
}
