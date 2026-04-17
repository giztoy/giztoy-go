package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRunPrintsRuntimeArgParseError(t *testing.T) {
	var stderr bytes.Buffer
	err := run([]string{"--service-run-workspace"}, &stderr)
	if err == nil {
		t.Fatal("run() error = nil, want parse error")
	}
	if !strings.Contains(stderr.String(), err.Error()) {
		t.Fatalf("stderr = %q, want %q", stderr.String(), err.Error())
	}
}

func TestRunPrintsServiceRunError(t *testing.T) {
	previous := runWorkspaceService
	runWorkspaceService = func(string) error {
		return errors.New("boom")
	}
	t.Cleanup(func() {
		runWorkspaceService = previous
	})

	var stderr bytes.Buffer
	err := run([]string{"--service-run-workspace", "/tmp/workspace"}, &stderr)
	if err == nil {
		t.Fatal("run() error = nil, want service error")
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("stderr = %q, want service error", stderr.String())
	}
}
