package buffer

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
)

func TestRingBuffer(t *testing.T) {
	t.Run("size=1", func(t *testing.T) {
		rb := RingN[byte](1)
		rb.Write([]byte{1, 2, 3})
		rb.CloseWrite()

		if rb.Len() != 1 {
			t.Errorf("len=%d", rb.Len())
		}

		got, err := io.ReadAll(rb)
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if !bytes.Equal(got, []byte{3}) {
			t.Errorf("got=%v", got)
		}
	})

	t.Run("size=2", func(t *testing.T) {
		rb := RingN[byte](2)
		rb.Write([]byte{1, 2, 3})
		rb.CloseWrite()

		if rb.Len() != 2 {
			t.Errorf("len=%d", rb.Len())
		}

		got, err := io.ReadAll(rb)
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if !bytes.Equal(got, []byte{2, 3}) {
			t.Errorf("got=%v", got)
		}
	})

	t.Run("size=3", func(t *testing.T) {
		rb := RingN[byte](3)
		rb.Write([]byte{1, 2, 3})
		rb.CloseWrite()

		if rb.Len() != 3 {
			t.Errorf("len=%d", rb.Len())
		}

		got, err := io.ReadAll(rb)
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if !bytes.Equal(got, []byte{1, 2, 3}) {
			t.Errorf("got=%v", got)
		}
	})

	t.Run("size=4", func(t *testing.T) {
		rb := RingN[byte](4)
		rb.Write([]byte{1, 2, 3})
		rb.CloseWrite()

		if rb.Len() != 3 {
			t.Errorf("len=%d", rb.Len())
		}

		got, err := io.ReadAll(rb)
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if !bytes.Equal(got, []byte{1, 2, 3}) {
			t.Errorf("got=%v", got)
		}
	})

	t.Run("size=100,7,1", func(t *testing.T) {
		rb := RingN[byte](7)
		for i := range 100 {
			rb.Write([]byte{byte(i)})
		}
		rb.CloseWrite()

		if rb.Len() != 7 {
			t.Errorf("len=%d", rb.Len())
		}

		got, err := io.ReadAll(rb)
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if !bytes.Equal(got, []byte{93, 94, 95, 96, 97, 98, 99}) {
			t.Errorf("got=%v", got)
		}
	})

	t.Run("size=100,7,3", func(t *testing.T) {
		rb := RingN[byte](7)
		for i := range 100 {
			rb.Write([]byte{byte(i), byte(i + 1), byte(i + 2)})
		}
		rb.CloseWrite()

		if rb.Len() != 7 {
			t.Errorf("len=%d", rb.Len())
		}

		got, err := io.ReadAll(rb)
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if !bytes.Equal(got, []byte{99, 98, 99, 100, 99, 100, 101}) {
			t.Errorf("got=%v", got)
		}
	})

	t.Run("size=100,7,7", func(t *testing.T) {
		rb := RingN[byte](7)
		for i := range 100 {
			rb.Write([]byte{byte(i), byte(i + 1), byte(i + 2), byte(i + 3), byte(i + 4), byte(i + 5), byte(i + 6)})
		}
		rb.CloseWrite()

		if rb.Len() != 7 {
			t.Errorf("len=%d", rb.Len())
		}

		got, err := io.ReadAll(rb)
		if err != nil {
			t.Errorf("read with error: %v", err)
		}
		if !bytes.Equal(got, []byte{99, 100, 101, 102, 103, 104, 105}) {
			t.Errorf("got=%v", got)
		}
	})
}

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
