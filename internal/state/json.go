package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"
)

// decodeJSON accepts exactly one JSON value with known fields and valid UTF-8.
func decodeJSON(data []byte, target any) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("invalid UTF-8")
	}
	// None of the command/snapshot schema fields are nullable. encoding/json
	// otherwise silently converts null into zero-valued strings.
	scan := json.NewDecoder(bytes.NewReader(data))
	for {
		token, err := scan.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if token == nil {
			return fmt.Errorf("null is not allowed")
		}
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON")
	}
	return nil
}
