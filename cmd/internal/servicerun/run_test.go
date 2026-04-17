package servicerun

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestProgramStopWithoutState(t *testing.T) {
	var p program
	if err := p.Stop(nil); err != nil {
		t.Fatalf("Stop() error = %v, want nil", err)
	}
}

func TestProgramStopCancelsRunningServer(t *testing.T) {
	done := make(chan error, 1)
	stopped := make(chan struct{}, 1)
	p := &program{
		cancel: func() {
			stopped <- struct{}{}
			done <- context.Canceled
		},
		done: done,
	}

	if err := p.Stop(nil); err != nil {
		t.Fatalf("Stop() error = %v, want nil", err)
	}

	select {
	case <-stopped:
	default:
		t.Fatal("Stop() did not invoke cancel")
	}
}

func TestProgramStopReturnsServerError(t *testing.T) {
	done := make(chan error, 1)
	done <- errors.New("boom")
	p := &program{done: done}

	err := p.Stop(nil)
	if err == nil || err.Error() != "service: server stopped with error: boom" {
		t.Fatalf("Stop() error = %v, want wrapped server error", err)
	}
}

func TestProgramStopTimesOut(t *testing.T) {
	previous := serviceStopTimeout
	serviceStopTimeout = 10 * time.Millisecond
	t.Cleanup(func() {
		serviceStopTimeout = previous
	})

	p := &program{done: make(chan error)}
	err := p.Stop(nil)
	if err == nil || err.Error() != "service: timeout waiting for server shutdown" {
		t.Fatalf("Stop() error = %v, want timeout", err)
	}
}
