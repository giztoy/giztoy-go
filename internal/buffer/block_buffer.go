package buffer

import (
	"fmt"
	"io"
	"slices"
	"sync"
)

// BlockBuffer is a thread-safe fixed-size circular buffer.
type BlockBuffer[T any] struct {
	cond *sync.Cond

	mu         sync.Mutex
	buf        []T
	head, tail int64
	closeWrite bool
	closeErr   error
}

// Block creates a new BlockBuffer using the provided slice as storage.
func Block[T any](buf []T) *BlockBuffer[T] {
	v := &BlockBuffer[T]{
		buf: buf,
	}
	v.cond = sync.NewCond(&v.mu)
	return v
}

// BlockN creates a new BlockBuffer with the specified size.
func BlockN[T any](size int) *BlockBuffer[T] {
	return Block(make([]T, size))
}

func (bb *BlockBuffer[T]) Discard(n int) (err error) {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	if bb.closeErr != nil {
		return fmt.Errorf("buffer: skip from closed buffer: %w", bb.closeErr)
	}
	if n > int(bb.tail-bb.head) {
		bb.head = bb.tail
		return nil
	}
	bb.head += int64(n)
	return nil
}

func (bb *BlockBuffer[T]) Read(p []T) (int, error) {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	if bb.closeErr != nil {
		return 0, fmt.Errorf("buffer: read from closed buffer: %w", bb.closeErr)
	}

	for bb.head == bb.tail {
		if bb.closeWrite {
			return 0, io.EOF
		}
		bb.cond.Wait()
		if bb.closeErr != nil {
			return 0, fmt.Errorf("buffer: read from closed buffer: %w", bb.closeErr)
		}
	}

	avail := int(bb.tail - bb.head)
	head := int(bb.head % int64(len(bb.buf)))

	var n int
	if head+avail <= len(bb.buf) {
		n = copy(p, bb.buf[head:head+avail])
	} else {
		n = copy(p, bb.buf[head:])
		n += copy(p[n:], bb.buf[:avail-n])
	}

	bb.head += int64(n)
	bb.cond.Signal()
	return n, nil
}

func (bb *BlockBuffer[T]) Write(p []T) (int, error) {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	if bb.closeErr != nil {
		return 0, fmt.Errorf("buffer: write to closed buffer: %w", bb.closeErr)
	}
	if bb.closeWrite {
		return 0, fmt.Errorf("buffer: write to closed buffer: %w", io.ErrClosedPipe)
	}

	wn := 0
	bufsz := int64(len(bb.buf))
	for len(p) > 0 {
		for bb.tail-bb.head == bufsz {
			bb.cond.Wait()
			if bb.closeErr != nil {
				return wn, fmt.Errorf("buffer: write to closed buffer: %w", bb.closeErr)
			}
			if bb.closeWrite {
				return wn, fmt.Errorf("buffer: write to closed buffer: %w", io.ErrClosedPipe)
			}
		}
		avail := int(bufsz - (bb.tail - bb.head))
		tail := int(bb.tail % bufsz)

		var n int
		if tail+avail <= len(bb.buf) {
			n = copy(bb.buf[tail:tail+avail], p)
		} else {
			n = copy(bb.buf[tail:], p)
			n += copy(bb.buf[:avail-n], p[n:])
		}

		bb.tail += int64(n)
		p = p[n:]
		wn += n
		bb.cond.Signal()
	}
	return wn, nil
}

func (bb *BlockBuffer[T]) CloseWithError(err error) error {
	if err == nil {
		err = io.ErrClosedPipe
	}
	bb.mu.Lock()
	defer bb.mu.Unlock()
	return bb.closeWithErrorLocked(err)
}

func (bb *BlockBuffer[T]) closeWithErrorLocked(err error) error {
	if bb.closeErr != nil {
		return nil
	}
	bb.closeErr = err
	if !bb.closeWrite {
		bb.closeWrite = true
	}
	bb.cond.Broadcast()
	return nil
}

func (bb *BlockBuffer[T]) Error() error {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	return bb.closeErr
}

func (bb *BlockBuffer[T]) Close() error {
	return bb.CloseWithError(io.ErrClosedPipe)
}

func (bb *BlockBuffer[T]) CloseWrite() error {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	if bb.closeWrite {
		return nil
	}
	bb.closeWrite = true
	bb.cond.Broadcast()
	return nil
}

func (bb *BlockBuffer[T]) Next() (t T, err error) {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	if bb.closeErr != nil {
		err = fmt.Errorf("buffer: read from closed buffer: %w", bb.closeErr)
		return
	}
	for bb.head == bb.tail {
		if bb.closeWrite {
			err = ErrIteratorDone
			return
		}
		bb.cond.Wait()
		if bb.closeErr != nil {
			err = fmt.Errorf("buffer: read from closed buffer: %w", bb.closeErr)
			return
		}
	}
	head := bb.head % int64(len(bb.buf))
	chunk := bb.buf[head]
	bb.head++
	bb.cond.Signal()
	return chunk, nil
}

func (bb *BlockBuffer[T]) Add(t T) error {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	if bb.closeErr != nil {
		return fmt.Errorf("buffer: write to closed buffer: %w", bb.closeErr)
	}
	if bb.closeWrite {
		return fmt.Errorf("buffer: write to closed buffer: %w", io.ErrClosedPipe)
	}
	bufsz := int64(len(bb.buf))
	for bb.tail-bb.head == bufsz {
		bb.cond.Wait()
		if bb.closeErr != nil {
			return fmt.Errorf("buffer: write to closed buffer: %w", bb.closeErr)
		}
		if bb.closeWrite {
			return fmt.Errorf("buffer: write to closed buffer: %w", io.ErrClosedPipe)
		}
	}
	tail := bb.tail % int64(len(bb.buf))
	bb.buf[tail] = t
	bb.tail++
	bb.cond.Signal()
	return nil
}

func (bb *BlockBuffer[T]) Reset() {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	bb.head = 0
	bb.tail = 0
}

func (bb *BlockBuffer[T]) Len() int {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	return int(bb.tail - bb.head)
}

func (bb *BlockBuffer[T]) Bytes() []T {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	h := bb.head % int64(len(bb.buf))
	t := bb.tail % int64(len(bb.buf))
	if h < t {
		return bb.buf[h:t]
	}
	return slices.Concat(bb.buf[h:], bb.buf[:t])
}
