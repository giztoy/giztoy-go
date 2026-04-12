package gizclaw

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestEventValidate(t *testing.T) {
	err := (Event{V: Version, Name: "joined"}).Validate()
	if err != nil {
		t.Fatalf("Validate(valid) error = %v", err)
	}

	if err := (Event{V: 2, Name: "joined"}).Validate(); !errors.Is(err, ErrInvalidV) {
		t.Fatalf("Validate(invalid version) err = %v", err)
	}

	if err := (Event{V: Version, Name: "   "}).Validate(); !errors.Is(err, ErrMissingName) {
		t.Fatalf("Validate(blank name) err = %v", err)
	}
}

func TestEventJSONRoundTrip(t *testing.T) {
	raw := RawMessage(`{"room":"alpha"}`)
	want := Event{
		V:    Version,
		Name: "joined",
		Data: &raw,
	}

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	var got Event
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	if got.V != want.V || got.Name != want.Name {
		t.Fatalf("round trip mismatch: got=%+v want=%+v", got, want)
	}
	if got.Data == nil || want.Data == nil || string(*got.Data) != string(*want.Data) {
		t.Fatalf("round trip mismatch: got=%+v want=%+v", got, want)
	}
}
