package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"
)

// OpType identifies one of the three planned KV operations.
type OpType string

const (
	OpPut    OpType = "PUT"
	OpGet    OpType = "GET"
	OpDelete OpType = "DELETE"
)

// Command is the JSON command envelope. Keys are nonempty UTF-8 strings;
// values may be empty. An omitted value means empty. GET/DELETE must not carry
// a nonempty value. No domain size limit has been selected.
type Command struct {
	Op    OpType `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

var ErrInvalidCommand = errors.New("invalid command")

func validateCommand(c Command) error {
	if c.Key == "" || !utf8.ValidString(c.Key) || !utf8.ValidString(c.Value) {
		return fmt.Errorf("%w: nonempty UTF-8 key and UTF-8 value required", ErrInvalidCommand)
	}
	switch c.Op {
	case OpPut:
	case OpGet, OpDelete:
		if c.Value != "" {
			return fmt.Errorf("%w: read/delete cannot carry a value", ErrInvalidCommand)
		}
	default:
		return fmt.Errorf("%w: unknown operation", ErrInvalidCommand)
	}
	return nil
}

func EncodeCommand(c Command) ([]byte, error) {
	if err := validateCommand(c); err != nil {
		return nil, err
	}
	return json.Marshal(c)
}

func DecodeCommand(data []byte) (Command, error) {
	var c Command
	if err := decodeJSON(data, &c); err != nil {
		return c, fmt.Errorf("%w: %v", ErrInvalidCommand, err)
	}
	return c, validateCommand(c)
}
