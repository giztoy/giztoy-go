package buffer

import (
	"fmt"
	"io"
	"slices"
	"sync"
)

// RingBuffer is a thread-safe ring buffer.
type RingBuffer[T any] struct {
	writeNotify chan struct{}

	mu         sync.Mutex
	buf        []T
	head, tail int64
	closeWrite bool
	closeErr   error
}

// RingN creates a new RingBuffer with the specified size.
func RingN[T any](size int) *RingBuffer[T] {
	return &RingBuffer[T]{
		writeNotify: make(chan struct{}, 1),
		buf:         make([]T, size),
	}
}

func (rb *RingBuffer[T]) Discard(n int) (err error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.closeErr != nil {
		return fmt.Errorf("buffer: skip from closed buffer: %w", rb.closeErr)
	}
	if n > int(rb.tail-rb.head) {
		rb.head = rb.tail
		return nil
	}
	rb.head += int64(n)
	return nil
}

func (rb *RingBuffer[T]) Read(p []T) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closeErr != nil {
		return 0, fmt.Errorf("buffer: read from closed buffer: %w", rb.closeErr)
	}

	for rb.head == rb.tail {
		if rb.closeWrite {
			return 0, io.EOF
		}
		rb.mu.Unlock()
		<-rb.writeNotify
		rb.mu.Lock()
		if rb.closeErr != nil {
			return 0, fmt.Errorf("buffer: read from closed buffer: %w", rb.closeErr)
		}
	}

	avail := int(rb.tail - rb.head)
	head := int(rb.head % int64(len(rb.buf)))

	var n int
	if head+avail <= len(rb.buf) {
		n = copy(p, rb.buf[head:head+avail])
	} else {
		n = copy(p, rb.buf[head:])
		n += copy(p[n:], rb.buf[:avail-n])
	}

	rb.head += int64(n)
	return n, nil
}

func (rb *RingBuffer[T]) CloseWithError(err error) error {
	if err == nil {
		err = io.ErrClosedPipe
	}
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.closeWithErrorLocked(err)
}

func (rb *RingBuffer[T]) closeWithErrorLocked(err error) error {
	if rb.closeErr != nil {
		return nil
	}
	rb.closeErr = err

	if !rb.closeWrite {
		rb.closeWrite = true
		close(rb.writeNotify)
	}
	return nil
}

func (rb *RingBuffer[T]) Error() error {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.closeErr
}

func (rb *RingBuffer[T]) Close() error {
	return rb.CloseWithError(io.ErrClosedPipe)
}

func (rb *RingBuffer[T]) CloseWrite() error {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.closeWrite {
		return nil
	}
	rb.closeWrite = true
	close(rb.writeNotify)
	return nil
}

func (rb *RingBuffer[T]) Write(p []T) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.closeErr != nil {
		return 0, fmt.Errorf("buffer: write to closed buffer: %w", rb.closeErr)
	}
	if rb.closeWrite {
		return 0, fmt.Errorf("buffer: write to closed buffer: %w", io.ErrClosedPipe)
	}

	bufsz := int64(len(rb.buf))
	avail := int(bufsz - (rb.tail - rb.head))
	tail := int(rb.tail % bufsz)

	var wn int
	if avail > 0 {
		if tail+avail <= len(rb.buf) {
			wn = copy(rb.buf[tail:tail+avail], p)
		} else {
			wn = copy(rb.buf[tail:], p)
			wn += copy(rb.buf[:avail-wn], p[wn:])
		}
		rb.tail += int64(wn)
	}

	leftn := len(p) - wn
	if leftn == 0 {
		return wn, nil
	}

	var cbuf, bbuf []T

	if leftn <= len(rb.buf) {
		cbuf = p[len(p)-leftn:]
	} else {
		cn := leftn % len(rb.buf)
		cbuf = p[len(p)-cn:]
		bbuf = p[len(p)-len(rb.buf) : len(p)-cn]
	}

	head := int(rb.head % bufsz)
	if cp1 := copy(rb.buf[head:], cbuf); cp1 < len(cbuf) {
		cp2 := copy(rb.buf, cbuf[cp1:])
		copy(rb.buf[cp2:], bbuf)
	} else {
		bp1 := copy(rb.buf[head+cp1:], bbuf)
		copy(rb.buf, bbuf[bp1:])
	}

	rb.head += int64(len(cbuf))
	rb.tail += int64(len(cbuf))

	return len(p), nil
}

func (rb *RingBuffer[T]) Next() (t T, err error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.closeErr != nil {
		err = fmt.Errorf("buffer: read from closed buffer: %w", rb.closeErr)
		return
	}
	for rb.head == rb.tail {
		if rb.closeWrite {
			err = ErrIteratorDone
			return
		}
		rb.mu.Unlock()
		<-rb.writeNotify
		rb.mu.Lock()
		if rb.closeErr != nil {
			err = fmt.Errorf("buffer: read from closed buffer: %w", rb.closeErr)
			return
		}
	}
	head := rb.head % int64(len(rb.buf))
	chunk := rb.buf[head]
	rb.head++
	return chunk, nil
}

func (rb *RingBuffer[T]) Add(t T) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.closeErr != nil {
		return fmt.Errorf("buffer: write to closed buffer: %w", rb.closeErr)
	}
	if rb.closeWrite {
		return fmt.Errorf("buffer: write to closed buffer: %w", io.ErrClosedPipe)
	}
	tail := rb.tail % int64(len(rb.buf))
	rb.buf[tail] = t
	rb.tail++
	if rb.tail-rb.head > int64(len(rb.buf)) {
		rb.head++
	}
	select {
	case rb.writeNotify <- struct{}{}:
	default:
	}
	return nil
}

func (rb *RingBuffer[T]) Reset() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.head = 0
	rb.tail = 0
}

func (rb *RingBuffer[T]) Len() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return int(rb.tail - rb.head)
}

func (rb *RingBuffer[T]) Bytes() []T {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	h := rb.head % int64(len(rb.buf))
	t := rb.tail % int64(len(rb.buf))
	if h < t {
		return rb.buf[h:t]
	}
	return slices.Concat(rb.buf[h:], rb.buf[:t])
}
