package optional

import (
	"bytes"
	"encoding/json"
	"errors"
)

// MarshalJSON encodes None as null and Some exactly as its stored value.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if !o.some {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}

// UnmarshalJSON decodes null as None and any successfully decoded non-null
// value as Some. A failed decode leaves the receiver unchanged.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	if o == nil {
		return errors.New("optional: UnmarshalJSON called on nil receiver")
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*o = None[T]()
		return nil
	}

	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*o = Some(value)
	return nil
}
