package gizclaw

import (
	"errors"
	"strings"
)

//go:generate go tool oapi-codegen -config=event_codegen_config.yaml -o event_generated.go ../../api/event_types.json

const EventVersion = 1

var (
	ErrInvalidEventVersion = errors.New("event: invalid version")
	ErrEventMissingName    = errors.New("event: missing name")
)

func (e Event) Validate() error {
	if e.V != EventVersion {
		return ErrInvalidEventVersion
	}
	if strings.TrimSpace(e.Name) == "" {
		return ErrEventMissingName
	}
	return nil
}
