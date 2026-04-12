package gizclaw

import (
	"encoding/json"
	"errors"
	"strings"
)

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config=event_codegen_config.yaml -o event_generated.go ../../api/event_types.json

const Version = 1

type RawMessage = json.RawMessage

var (
	ErrInvalidV    = errors.New("event: invalid version")
	ErrMissingName = errors.New("event: missing name")
)

func (e Event) Validate() error {
	if e.V != Version {
		return ErrInvalidV
	}
	if strings.TrimSpace(e.Name) == "" {
		return ErrMissingName
	}
	return nil
}
