package buffer

import (
	"errors"
	"io"
	"testing"
	"time"
)

func TestRingBufferAddNextDiscardAndCloseWithError(t *testing.T) {
	rb := RingN[int](2)

	if err := rb.Add(1); err != nil {
		t.Fatalf("add 1 failed: %v", err)
	}
	if err := rb.Add(2); err != nil {
		t.Fatalf("add 2 failed: %v", err)
	}
	if err := rb.Add(3); err != nil {
		t.Fatalf("add 3 failed: %v", err)
	}

	if rb.Len() != 2 {
		t.Fatalf("len=%d, want=2", rb.Len())
	}

	v, err := rb.Next()
	if err != nil {
		t.Fatalf("next failed: %v", err)
	}
	if v != 2 {
		t.Fatalf("first next=%d, want=2", v)
	}

	if err := rb.Discard(1); err != nil {
		t.Fatalf("discard failed: %v", err)
	}
	if rb.Len() != 0 {
		t.Fatalf("len after discard=%d, want=0", rb.Len())
	}

	if err := rb.CloseWithError(nil); err != nil {
		t.Fatalf("close with nil error failed: %v", err)
	}
	if !errors.Is(rb.Error(), io.ErrClosedPipe) {
		t.Fatalf("expected io.ErrClosedPipe, got: %v", rb.Error())
	}

	if _, err := rb.Write([]int{9}); err == nil {
		t.Fatal("write should fail after CloseWithError")
	}
	if err := rb.Add(9); err == nil {
		t.Fatal("add should fail after CloseWithError")
	}
	if _, err := rb.Read(make([]int, 1)); err == nil {
		t.Fatal("read should fail after CloseWithError")
	}
	if _, err := rb.Next(); err == nil {
		t.Fatal("next should fail after CloseWithError")
	}
	if err := rb.Discard(1); err == nil {
		t.Fatal("discard should fail after CloseWithError")
	}
}

func TestRingBufferBlockingNextUnblockedByAdd(t *testing.T) {
	rb := RingN[int](2)

	done := make(chan int, 1)
	go func() {
		v, err := rb.Next()
		if err != nil {
			done <- -1
			return
		}
		done <- v
	}()

	time.Sleep(20 * time.Millisecond)
	if err := rb.Add(42); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	select {
	case v := <-done:
		if v != 42 {
			t.Fatalf("unexpected next result: %d", v)
		}
	case <-time.After(time.Second):
		t.Fatal("next was not unblocked by add")
	}

	if err := rb.CloseWrite(); err != nil {
		t.Fatalf("close write failed: %v", err)
	}
	if _, err := rb.Next(); !errors.Is(err, ErrIteratorDone) {
		t.Fatalf("expected ErrIteratorDone, got: %v", err)
	}
}

func TestBlockBufferDiscardCloseAndBlockedAdd(t *testing.T) {
	bb := BlockN[int](1)
	if err := bb.Add(1); err != nil {
		t.Fatalf("add 1 failed: %v", err)
	}

	addDone := make(chan error, 1)
	go func() {
		addDone <- bb.Add(2)
	}()

	time.Sleep(20 * time.Millisecond)
	v, err := bb.Next()
	if err != nil {
		t.Fatalf("next failed: %v", err)
	}
	if v != 1 {
		t.Fatalf("next=%d, want=1", v)
	}

	select {
	case err := <-addDone:
		if err != nil {
			t.Fatalf("blocked add failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked add did not resume")
	}

	if err := bb.Discard(1); err != nil {
		t.Fatalf("discard failed: %v", err)
	}
	if bb.Len() != 0 {
		t.Fatalf("len after discard=%d, want=0", bb.Len())
	}

	if err := bb.CloseWithError(nil); err != nil {
		t.Fatalf("close with nil error failed: %v", err)
	}
	if !errors.Is(bb.Error(), io.ErrClosedPipe) {
		t.Fatalf("expected io.ErrClosedPipe, got: %v", bb.Error())
	}

	if _, err := bb.Read(make([]int, 1)); err == nil {
		t.Fatal("read should fail after CloseWithError")
	}
	if _, err := bb.Write([]int{3}); err == nil {
		t.Fatal("write should fail after CloseWithError")
	}
	if err := bb.Add(3); err == nil {
		t.Fatal("add should fail after CloseWithError")
	}
	if _, err := bb.Next(); err == nil {
		t.Fatal("next should fail after CloseWithError")
	}
	if err := bb.Discard(1); err == nil {
		t.Fatal("discard should fail after CloseWithError")
	}
}
