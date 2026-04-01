package netbench

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/benchmark/net/internal/adapters"
)

func TestSmoke_KCPPair_DataPath(t *testing.T) {
	pair, err := adapters.NewKCPPair(0)
	if err != nil {
		t.Fatalf("new kcp pair failed: %v", err)
	}
	defer func() { _ = pair.Close() }()

	msgAB := []byte("smoke-kcp-a2b")
	if _, err := pair.A.Write(msgAB); err != nil {
		t.Fatalf("A write failed: %v", err)
	}
	if got := readExactWithTimeout(t, pair.B, len(msgAB), 2*time.Second); !bytes.Equal(got, msgAB) {
		t.Fatalf("B recv mismatch: got=%q want=%q", got, msgAB)
	}

	msgBA := []byte("smoke-kcp-b2a")
	if _, err := pair.B.Write(msgBA); err != nil {
		t.Fatalf("B write failed: %v", err)
	}
	if got := readExactWithTimeout(t, pair.A, len(msgBA), 2*time.Second); !bytes.Equal(got, msgBA) {
		t.Fatalf("A recv mismatch: got=%q want=%q", got, msgBA)
	}
}

func TestSmoke_NoisePair_DataPath(t *testing.T) {
	pair, err := adapters.NewNoisePair(0)
	if err != nil {
		t.Fatalf("new noise pair failed: %v", err)
	}
	defer func() { _ = pair.Close() }()

	msgAB := []byte("smoke-noise-a2b")
	if err := pair.SendAToB(msgAB); err != nil {
		t.Fatalf("A send failed: %v", err)
	}
	gotB, err := pair.RecvOnB(2 * time.Second)
	if err != nil {
		t.Fatalf("B recv failed: %v", err)
	}
	if !bytes.Equal(gotB, msgAB) {
		t.Fatalf("B recv mismatch: got=%q want=%q", gotB, msgAB)
	}

	msgBA := []byte("smoke-noise-b2a")
	if err := pair.SendBToA(msgBA); err != nil {
		t.Fatalf("B send failed: %v", err)
	}
	gotA, err := pair.RecvOnA(2 * time.Second)
	if err != nil {
		t.Fatalf("A recv failed: %v", err)
	}
	if !bytes.Equal(gotA, msgBA) {
		t.Fatalf("A recv mismatch: got=%q want=%q", gotA, msgBA)
	}
}

func TestSmoke_KCPNoisePair_DataPath(t *testing.T) {
	pair, err := adapters.NewKCPNoisePair(0)
	if err != nil {
		t.Fatalf("new kcp-noise pair failed: %v", err)
	}
	defer func() { _ = pair.Close() }()

	msg := []byte("smoke-kcp-noise")
	if _, err := pair.A.Write(msg); err != nil {
		t.Fatalf("A write failed: %v", err)
	}
	if got := readExactWithTimeout(t, pair.B, len(msg), 2*time.Second); !bytes.Equal(got, msg) {
		t.Fatalf("B recv mismatch: got=%q want=%q", got, msg)
	}
}

func readExactWithTimeout(t *testing.T, r io.Reader, n int, timeout time.Duration) []byte {
	t.Helper()

	buf := make([]byte, n)
	errCh := make(chan error, 1)
	go func() {
		_, err := io.ReadFull(r, buf)
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("read full failed: %v", err)
		}
		return buf
	case <-time.After(timeout):
		t.Fatalf("read timeout after %s", timeout)
		return nil
	}
}
