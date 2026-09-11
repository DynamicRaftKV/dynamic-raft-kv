package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func applyForTest(t *testing.T, s StateMachine, c Command) Result {
	t.Helper()
	data, err := EncodeCommand(c)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.Apply(data)
	if err != nil {
		t.Fatal(err)
	}
	var r Result
	if err := json.Unmarshal(result, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestApplyResults(t *testing.T) {
	s := NewStore()
	for _, tc := range []struct {
		c    Command
		want Result
	}{
		{Command{Op: OpGet, Key: "a"}, Result{Op: OpGet}},
		{Command{Op: OpPut, Key: "a", Value: "one"}, Result{Op: OpPut}},
		{Command{Op: OpGet, Key: "a"}, Result{Op: OpGet, Found: true, Value: "one"}},
		{Command{Op: OpPut, Key: "a"}, Result{Op: OpPut}},
		{Command{Op: OpGet, Key: "a"}, Result{Op: OpGet, Found: true}},
		{Command{Op: OpDelete, Key: "a"}, Result{Op: OpDelete}},
		{Command{Op: OpDelete, Key: "a"}, Result{Op: OpDelete}},
		{Command{Op: OpGet, Key: "a"}, Result{Op: OpGet}},
	} {
		if got := applyForTest(t, s, tc.c); got != tc.want {
			t.Fatalf("%+v: got %+v want %+v", tc.c, got, tc.want)
		}
	}
}

func TestInvalidCommandsDoNotMutate(t *testing.T) {
	s := NewStore()
	applyForTest(t, s, Command{Op: OpPut, Key: "keep", Value: "value"})
	before, _ := s.Snapshot()
	for _, input := range []string{"", "null", "[]", "{}", `{"op":"CAS","key":"keep"}`, `{"op":"PUT","key":""}`, `{"op":"PUT","key":"keep","value":null}`, `{"op":"PUT","key":"keep","value":3}`, `{"op":"PUT","key":"keep","ttl":1}`, `{"op":"GET","key":"keep","value":"x"}`, `{"op":"PUT","key":"keep"} {}`, "\xff"} {
		result, err := s.Apply([]byte(input))
		if !errors.Is(err, ErrInvalidCommand) || result != nil {
			t.Fatalf("%q: result=%s err=%v", input, result, err)
		}
		after, _ := s.Snapshot()
		if !bytes.Equal(before, after) {
			t.Fatalf("%q changed state", input)
		}
	}
	if _, err := EncodeCommand(Command{Op: OpPut, Key: "\xff"}); !errors.Is(err, ErrInvalidCommand) {
		t.Fatal("invalid UTF-8 key accepted")
	}
}
