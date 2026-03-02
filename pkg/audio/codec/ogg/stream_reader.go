package ogg

import (
	"errors"
	"fmt"
	"io"
)

// StreamReader 从 io.Reader 顺序读取 OGG page。
type StreamReader struct {
	r io.Reader
}

func NewStreamReader(r io.Reader) *StreamReader {
	return &StreamReader{r: r}
}

// NextPage 读取并解析下一个 page。
func (sr *StreamReader) NextPage() (*Page, error) {
	if sr == nil || sr.r == nil {
		return nil, fmt.Errorf("ogg: read page: reader is nil")
	}

	header := make([]byte, pageHeaderSize)
	if _, err := io.ReadFull(sr.r, header); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("ogg: read page: truncated header: %w", err)
		}
		return nil, fmt.Errorf("ogg: read page: read header failed: %w", err)
	}

	segCount := int(header[26])
	segments := make([]byte, segCount)
	if _, err := io.ReadFull(sr.r, segments); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("ogg: read page: truncated segment table: %w", err)
		}
		return nil, fmt.Errorf("ogg: read page: read segment table failed: %w", err)
	}

	payloadLen := 0
	for _, seg := range segments {
		payloadLen += int(seg)
	}
	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(sr.r, payload); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("ogg: read page: truncated payload: %w", err)
		}
		return nil, fmt.Errorf("ogg: read page: read payload failed: %w", err)
	}

	raw := make([]byte, 0, pageHeaderSize+len(segments)+len(payload))
	raw = append(raw, header...)
	raw = append(raw, segments...)
	raw = append(raw, payload...)

	page, err := ParsePage(raw)
	if err != nil {
		return nil, fmt.Errorf("ogg: read page: %w", err)
	}
	return page, nil
}

// ReadAllPages 读取输入中的全部 page。
func ReadAllPages(r io.Reader) ([]*Page, error) {
	sr := NewStreamReader(r)
	pages := make([]*Page, 0, 8)
	for {
		p, err := sr.NextPage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return pages, nil
			}
			return nil, err
		}
		pages = append(pages, p)
	}
}

// ReadAllPackets 读取并还原输入中的全部逻辑包。
func ReadAllPackets(r io.Reader) ([]Packet, error) {
	pages, err := ReadAllPages(r)
	if err != nil {
		return nil, err
	}
	return ExtractPackets(pages)
}
