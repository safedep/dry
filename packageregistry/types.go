package packageregistry

import (
	"encoding/json"
)

// OptionalInt represents an optional integer value
type OptionalInt struct {
	Value uint64
	Valid bool
}

// MarshalJSON implements custom JSON marshaling
func (o OptionalInt) MarshalJSON() ([]byte, error) {
	if !o.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(o.Value)
}

// UnmarshalJSON implements custom JSON unmarshaling
func (o *OptionalInt) UnmarshalJSON(data []byte) error {
	// Handle null case
	if string(data) == "null" {
		o.Valid = false
		return nil
	}

	// Try to unmarshal as int
	var value uint64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	o.Value = value
	o.Valid = true
	return nil
}
