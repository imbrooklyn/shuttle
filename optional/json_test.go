package optional_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/imbrooklyn/shuttle/optional"
)

type appendOnUnmarshal []int

func (value *appendOnUnmarshal) UnmarshalJSON(data []byte) error {
	var decoded []int
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = append(*value, decoded...)
	return nil
}

type customMarshaler int

func (customMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(`"custom"`), nil
}

type failingMarshaler int

func (failingMarshaler) MarshalJSON() ([]byte, error) {
	return nil, errors.New("marshal failure")
}

var nullUnmarshalCalls int

type nullAware int

func (*nullAware) UnmarshalJSON([]byte) error {
	nullUnmarshalCalls++
	return nil
}

func TestJSONRepresentation(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "None", value: optional.None[int](), want: "null"},
		{name: "Some zero", value: optional.Some(0), want: "0"},
		{name: "Some nil pointer", value: optional.Some[*int](nil), want: "null"},
		{name: "Some nil slice", value: optional.Some[[]int](nil), want: "null"},
		{name: "Some nil interface", value: optional.Some[any](nil), want: "null"},
		{name: "custom marshaler", value: optional.Some(customMarshaler(1)), want: `"custom"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(test.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != test.want {
				t.Fatalf("Marshal = %s, want %s", data, test.want)
			}
		})
	}

	if _, err := json.Marshal(optional.Some(failingMarshaler(1))); err == nil {
		t.Fatal("custom marshal error was not propagated")
	}
}

func TestJSONDecodeAndAtomicity(t *testing.T) {
	value := optional.Some(appendOnUnmarshal{9})
	if err := json.Unmarshal([]byte("[1,2]"), &value); err != nil {
		t.Fatal(err)
	}
	if got := value.Must(); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("decode did not use fresh zero temporary: %v", got)
	}

	before := optional.Some([]int{7, 8})
	receiver := before
	if err := json.Unmarshal([]byte("[1,"), &receiver); err == nil {
		t.Fatal("malformed JSON decoded successfully")
	}
	if !optional.EqualFunc(receiver, before, func(a, b []int) bool {
		return bytes.Equal(intsToBytes(a), intsToBytes(b))
	}) {
		t.Fatalf("failed decode changed receiver: %v", receiver)
	}

	if err := json.Unmarshal([]byte(" \n\t null \r"), &receiver); err != nil {
		t.Fatal(err)
	}
	if !receiver.IsNone() {
		t.Fatalf("null decoded as %v", receiver)
	}
	nullUnmarshalCalls = 0
	custom := optional.Some(nullAware(1))
	if err := json.Unmarshal([]byte("null"), &custom); err != nil {
		t.Fatal(err)
	}
	if !custom.IsNone() || nullUnmarshalCalls != 0 {
		t.Fatalf("null result = %v, custom UnmarshalJSON calls = %d", custom, nullUnmarshalCalls)
	}

	var nilReceiver *optional.Optional[int]
	if err := nilReceiver.UnmarshalJSON([]byte("1")); err == nil {
		t.Fatal("nil receiver returned nil error")
	}
}

func intsToBytes(values []int) []byte {
	result := make([]byte, len(values))
	for index, value := range values {
		result[index] = byte(value)
	}
	return result
}

func TestJSONFieldOmission(t *testing.T) {
	type fields struct {
		Plain     optional.Optional[int]  `json:"plain"`
		OmitEmpty optional.Optional[int]  `json:"empty,omitempty"`
		OmitZero  optional.Optional[int]  `json:"zero,omitzero"`
		Present   optional.Optional[int]  `json:"present,omitzero"`
		Nil       optional.Optional[*int] `json:"nil,omitzero"`
	}
	data, err := json.Marshal(fields{
		Present: optional.Some(0),
		Nil:     optional.Some[*int](nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"plain":null,"empty":null,"present":0,"nil":null}`
	if string(data) != want {
		t.Fatalf("Marshal fields = %s, want %s", data, want)
	}
}

func TestJSONPresentNilRoundTripIsLossy(t *testing.T) {
	original := optional.Some[*int](nil)
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded optional.Optional[*int]
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !original.IsSome() || !decoded.IsNone() {
		t.Fatalf("round trip states: original Some=%v decoded None=%v", original.IsSome(), decoded.IsNone())
	}
}

func TestIsZero(t *testing.T) {
	if !optional.None[int]().IsZero() {
		t.Fatal("None.IsZero() is false")
	}
	if optional.Some(0).IsZero() {
		t.Fatal("Some(0).IsZero() is true")
	}
}
