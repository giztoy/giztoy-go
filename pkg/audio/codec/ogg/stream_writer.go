package ogg

import (
	"fmt"
	"io"
)

// StreamWriter 把逻辑包写入 OGG page 流。
type StreamWriter struct {
	w            io.Writer
	serial       uint32
	nextSequence uint32
	started      bool
	ended        bool
	broken       bool
}

func NewStreamWriter(w io.Writer, serial uint32) (*StreamWriter, error) {
	if w == nil {
		return nil, fmt.Errorf("ogg: new stream writer: writer is nil")
	}
	return &StreamWriter{w: w, serial: serial}, nil
}

// NextSequence 返回下一页将使用的 page sequence。
func (sw *StreamWriter) NextSequence() uint32 {
	if sw == nil {
		return 0
	}
	return sw.nextSequence
}

// WritePacket 写入一个逻辑包。
//
// - 首包会自动打 BOS 标记。
// - 当 eos=true 时，最后一页会打 EOS 标记，后续再写会报错。
func (sw *StreamWriter) WritePacket(packet []byte, granulePos uint64, eos bool) (int, error) {
	if sw == nil || sw.w == nil {
		return 0, fmt.Errorf("ogg: write packet: writer is nil")
	}
	if sw.broken {
		return 0, fmt.Errorf("ogg: write packet: stream is broken after previous partial write failure")
	}
	if sw.ended {
		return 0, fmt.Errorf("ogg: write packet: stream already ended")
	}

	bos := !sw.started
	pages, err := BuildPacketPages(sw.serial, sw.nextSequence, packet, granulePos, bos, eos)
	if err != nil {
		return 0, fmt.Errorf("ogg: write packet: %w", err)
	}

	written := 0
	for idx, p := range pages {
		raw, err := p.MarshalBinary()
		if err != nil {
			return written, fmt.Errorf("ogg: write packet: marshal page %d failed: %w", idx, err)
		}
		n, werr := sw.w.Write(raw)
		written += n

		if n == len(raw) {
			sw.started = true
			sw.nextSequence++
		}

		if werr != nil {
			if n > 0 || idx > 0 {
				sw.broken = true
			}
			return written, fmt.Errorf("ogg: write packet: write page %d failed: %w", idx, werr)
		}
		if n != len(raw) {
			sw.broken = true
			return written, fmt.Errorf("ogg: write packet: short write on page %d: wrote %d, want %d", idx, n, len(raw))
		}
	}

	if eos {
		sw.ended = true
	}

	return written, nil
}
