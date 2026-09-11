package state

// Result is the JSON result envelope. PUT and DELETE acknowledge the
// operation; GET returns Found plus Value, including an empty existing value.
// DELETE of a missing key succeeds. Previous values are not returned.
type Result struct {
	Op    OpType `json:"op"`
	Found bool   `json:"found,omitempty"`
	Value string `json:"value,omitempty"`
}
