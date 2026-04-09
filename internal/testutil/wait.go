package testutil

import (
	"fmt"
	"net"
	"testing"
	"time"
)

const (
	ReadyTimeout = 10 * time.Second
	ProbeTimeout = time.Second
	PollInterval = 20 * time.Millisecond
)

func AllocateUDPAddr(t testing.TB) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocateUDPAddr: %v", err)
	}
	addr := pc.LocalAddr().(*net.UDPAddr)
	_ = pc.Close()
	return fmt.Sprintf("127.0.0.1:%d", addr.Port)
}

func WaitUntil(timeout time.Duration, check func() error) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := check(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(PollInterval)
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("condition not satisfied before timeout")
}
