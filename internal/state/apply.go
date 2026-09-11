package state

import "encoding/json"

// Apply validates the entire command before changing state. Results are JSON
// Result values; command errors wrap ErrInvalidCommand and leave state intact.
func (s *Store) Apply(data []byte) ([]byte, error) {
	command, err := DecodeCommand(data)
	if err != nil {
		return nil, err
	}
	result := Result{Op: command.Op}
	switch command.Op {
	case OpPut:
		s.put(command.Key, command.Value)
	case OpGet:
		result.Value, result.Found = s.get(command.Key)
	case OpDelete:
		s.delete(command.Key)
	}
	return json.Marshal(result)
}
