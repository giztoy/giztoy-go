package genx

import (
	"io"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/buffer"
)

// Matcher is a function that determines if a MessageChunk matches a criteria.
type Matcher func(*MessageChunk) bool

// MIMETypeMatcher returns a Matcher that matches chunks with the given MIME type prefix.
func MIMETypeMatcher(mimePrefix string) Matcher {
	return func(chunk *MessageChunk) bool {
		if chunk == nil || chunk.Part == nil {
			return false
		}
		blob, ok := chunk.Part.(*Blob)
		if !ok {
			return false
		}
		return len(blob.MIMEType) >= len(mimePrefix) && blob.MIMEType[:len(mimePrefix)] == mimePrefix
	}
}

// Split splits a stream into two streams based on a matcher function.
func Split(input Stream, match Matcher) (matched, rest Stream) {
	matchedBuf := buffer.N[*MessageChunk](100)
	restBuf := buffer.N[*MessageChunk](100)

	matchedStream := &bufferStream{buf: matchedBuf}
	restStream := &bufferStream{buf: restBuf}

	go func() {
		defer matchedBuf.CloseWrite()
		defer restBuf.CloseWrite()

		for {
			chunk, err := input.Next()
			if err != nil {
				if !isDoneErr(err) {
					matchedBuf.CloseWithError(err)
					restBuf.CloseWithError(err)
				}
				return
			}

			if match(chunk) {
				if err := matchedBuf.Add(chunk); err != nil {
					return
				}
			} else {
				if err := restBuf.Add(chunk); err != nil {
					return
				}
			}
		}
	}()

	return matchedStream, restStream
}

// CompositeSeq combines multiple streams sequentially with EoS markers.
func CompositeSeq(streams ...Stream) Stream {
	if len(streams) == 0 {
		return &emptyStream{}
	}
	if len(streams) == 1 {
		return streams[0]
	}

	outBuf := buffer.N[*MessageChunk](100)
	out := &bufferStream{buf: outBuf}

	go func() {
		defer outBuf.CloseWrite()

		for i, stream := range streams {
			var lastMIMEType string

			for {
				chunk, err := stream.Next()
				if err != nil {
					if isDoneErr(err) {
						break
					}
					outBuf.CloseWithError(err)
					return
				}

				if chunk != nil && chunk.Part != nil {
					if blob, ok := chunk.Part.(*Blob); ok {
						lastMIMEType = blob.MIMEType
					} else if _, ok := chunk.Part.(Text); ok {
						lastMIMEType = "text/plain"
					}
				}

				if err := outBuf.Add(chunk); err != nil {
					return
				}
			}

			if i < len(streams)-1 && lastMIMEType != "" {
				var eosChunk *MessageChunk
				if lastMIMEType == "text/plain" {
					eosChunk = NewTextEndOfStream()
				} else {
					eosChunk = NewEndOfStream(lastMIMEType)
				}
				if err := outBuf.Add(eosChunk); err != nil {
					return
				}
			}
		}
	}()

	return out
}

// Merge merges multiple streams into a single stream.
func Merge(streams ...Stream) Stream {
	if len(streams) == 0 {
		return &emptyStream{}
	}
	if len(streams) == 1 {
		return streams[0]
	}

	return &mergeStream{streams: streams, idx: 0}
}

// MergeInterleaved merges multiple streams by interleaving chunks.
func MergeInterleaved(streams ...Stream) Stream {
	if len(streams) == 0 {
		return &emptyStream{}
	}
	if len(streams) == 1 {
		return streams[0]
	}

	outBuf := buffer.N[*MessageChunk](100)
	out := &bufferStream{buf: outBuf}

	go func() {
		defer outBuf.CloseWrite()

		active := make([]bool, len(streams))
		for i := range active {
			active[i] = true
		}
		activeCount := len(streams)

		for activeCount > 0 {
			for i, stream := range streams {
				if !active[i] {
					continue
				}

				chunk, err := stream.Next()
				if err != nil {
					if isDoneErr(err) {
						active[i] = false
						activeCount--
						continue
					}
					outBuf.CloseWithError(err)
					return
				}

				if err := outBuf.Add(chunk); err != nil {
					return
				}
			}
		}
	}()

	return out
}

type bufferStream struct {
	buf    *buffer.Buffer[*MessageChunk]
	closed bool
	mu     sync.Mutex
}

func (s *bufferStream) Next() (*MessageChunk, error) {
	chunk, err := s.buf.Next()
	if err == buffer.ErrIteratorDone {
		return nil, io.EOF
	}
	if err != nil {
		return nil, err
	}
	return chunk, nil
}

func (s *bufferStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		s.buf.CloseWrite()
	}
	return nil
}

func (s *bufferStream) CloseWithError(err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		s.buf.CloseWithError(err)
	}
	return nil
}

type emptyStream struct{}

func (s *emptyStream) Next() (*MessageChunk, error) {
	return nil, io.EOF
}

func (s *emptyStream) Close() error {
	return nil
}

func (s *emptyStream) CloseWithError(err error) error {
	return nil
}

type mergeStream struct {
	streams []Stream
	idx     int
	mu      sync.Mutex
}

func (s *mergeStream) Next() (*MessageChunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for s.idx < len(s.streams) {
		chunk, err := s.streams[s.idx].Next()
		if isDoneErr(err) {
			s.idx++
			continue
		}
		if err != nil {
			return nil, err
		}
		return chunk, nil
	}

	return nil, io.EOF
}

func (s *mergeStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, stream := range s.streams {
		stream.Close()
	}
	return nil
}

func (s *mergeStream) CloseWithError(err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, stream := range s.streams {
		stream.CloseWithError(err)
	}
	return nil
}
