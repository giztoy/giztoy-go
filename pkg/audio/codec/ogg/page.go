package ogg

import (
	"encoding/binary"
	"fmt"
)

// Page 表示一个 OGG 物理页。
type Page struct {
	Version         byte
	HeaderType      byte
	GranulePosition uint64
	BitstreamSerial uint32
	PageSequence    uint32
	Checksum        uint32
	Segments        []byte
	Payload         []byte
}

func (p *Page) HasContinuation() bool {
	return p != nil && (p.HeaderType&HeaderTypeContinued) != 0
}

func (p *Page) HasBOS() bool {
	return p != nil && (p.HeaderType&HeaderTypeBOS) != 0
}

func (p *Page) HasEOS() bool {
	return p != nil && (p.HeaderType&HeaderTypeEOS) != 0
}

// Validate 校验页结构完整性。
func (p *Page) Validate() error {
	if p == nil {
		return fmt.Errorf("ogg: validate page: page is nil")
	}
	if p.Version != 0 {
		return fmt.Errorf("ogg: validate page: unsupported version %d", p.Version)
	}
	if p.HeaderType&^byte(HeaderTypeContinued|HeaderTypeBOS|HeaderTypeEOS) != 0 {
		return fmt.Errorf("ogg: validate page: unsupported header type bits 0x%02x", p.HeaderType)
	}
	if len(p.Segments) > maxPageSegments {
		return fmt.Errorf("ogg: validate page: segment count %d exceeds %d", len(p.Segments), maxPageSegments)
	}
	payloadSize := 0
	for _, seg := range p.Segments {
		payloadSize += int(seg)
	}
	if payloadSize != len(p.Payload) {
		return fmt.Errorf("ogg: validate page: payload length mismatch: got %d, want %d", len(p.Payload), payloadSize)
	}
	return nil
}

// MarshalBinary 序列化并自动计算 CRC。
func (p *Page) MarshalBinary() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("ogg: marshal page: %w", err)
	}

	headerLen := pageHeaderSize + len(p.Segments)
	raw := make([]byte, headerLen+len(p.Payload))

	copy(raw[0:4], CapturePattern)
	raw[4] = p.Version
	raw[5] = p.HeaderType
	binary.LittleEndian.PutUint64(raw[6:14], p.GranulePosition)
	binary.LittleEndian.PutUint32(raw[14:18], p.BitstreamSerial)
	binary.LittleEndian.PutUint32(raw[18:22], p.PageSequence)
	// raw[22:26] checksum placeholder, keep zeros before CRC.
	raw[26] = byte(len(p.Segments))
	copy(raw[27:headerLen], p.Segments)
	copy(raw[headerLen:], p.Payload)

	checksum := oggCRC(raw)
	binary.LittleEndian.PutUint32(raw[22:26], checksum)
	return raw, nil
}

// ParsePage 解析单个完整 page（输入不允许包含多余 trailing 数据）。
func ParsePage(raw []byte) (*Page, error) {
	p, consumed, err := parsePagePrefix(raw)
	if err != nil {
		return nil, err
	}
	if consumed != len(raw) {
		return nil, fmt.Errorf("ogg: parse page: trailing data: %d bytes", len(raw)-consumed)
	}
	return p, nil
}

func parsePagePrefix(raw []byte) (*Page, int, error) {
	if len(raw) < pageHeaderSize {
		return nil, 0, fmt.Errorf("ogg: parse page: too short header: got %d, want >= %d", len(raw), pageHeaderSize)
	}
	if string(raw[:4]) != CapturePattern {
		return nil, 0, fmt.Errorf("ogg: parse page: invalid capture pattern %q", raw[:4])
	}
	if raw[4] != 0 {
		return nil, 0, fmt.Errorf("ogg: parse page: unsupported version %d", raw[4])
	}

	segCount := int(raw[26])
	headerLen := pageHeaderSize + segCount
	if len(raw) < headerLen {
		return nil, 0, fmt.Errorf("ogg: parse page: truncated segment table: got %d, want >= %d", len(raw), headerLen)
	}

	payloadLen := 0
	for _, seg := range raw[27:headerLen] {
		payloadLen += int(seg)
	}
	totalLen := headerLen + payloadLen
	if len(raw) < totalLen {
		return nil, 0, fmt.Errorf("ogg: parse page: truncated payload: got %d, want >= %d", len(raw), totalLen)
	}

	pageBytes := raw[:totalLen]
	wantChecksum := binary.LittleEndian.Uint32(pageBytes[22:26])
	checksumInput := make([]byte, totalLen)
	copy(checksumInput, pageBytes)
	checksumInput[22] = 0
	checksumInput[23] = 0
	checksumInput[24] = 0
	checksumInput[25] = 0
	gotChecksum := oggCRC(checksumInput)
	if gotChecksum != wantChecksum {
		return nil, 0, fmt.Errorf("ogg: parse page: checksum mismatch: expected 0x%08x, got 0x%08x", wantChecksum, gotChecksum)
	}

	p := &Page{
		Version:         pageBytes[4],
		HeaderType:      pageBytes[5],
		GranulePosition: binary.LittleEndian.Uint64(pageBytes[6:14]),
		BitstreamSerial: binary.LittleEndian.Uint32(pageBytes[14:18]),
		PageSequence:    binary.LittleEndian.Uint32(pageBytes[18:22]),
		Checksum:        wantChecksum,
		Segments:        append([]byte(nil), pageBytes[27:headerLen]...),
		Payload:         append([]byte(nil), pageBytes[headerLen:totalLen]...),
	}

	if err := p.Validate(); err != nil {
		return nil, 0, fmt.Errorf("ogg: parse page: %w", err)
	}

	return p, totalLen, nil
}

// ParsePages 解析连续拼接的 OGG page 字节流。
func ParsePages(stream []byte) ([]*Page, error) {
	if len(stream) == 0 {
		return nil, nil
	}

	pages := make([]*Page, 0, 4)
	offset := 0
	for offset < len(stream) {
		p, consumed, err := parsePagePrefix(stream[offset:])
		if err != nil {
			return nil, fmt.Errorf("ogg: parse pages: offset %d: %w", offset, err)
		}
		pages = append(pages, p)
		offset += consumed
	}

	return pages, nil
}
