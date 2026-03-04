package peer

import "encoding/binary"

const (
	OpusFrameVersion = 1
	opusFrameHeader  = 8 // version(1B) + timestamp(7B)
)

type EpochMillis int64

// StampedOpusFrame format:
// [version:1 byte][timestamp:7 bytes][opus frame bytes:N]
//
// 与旧 chatgear/opus.Stamp 语义一致：
// - version 固定写入第 1 字节；
// - timestamp 使用 big-endian 的低 56bit（写入后 7 字节）。
type StampedOpusFrame []byte

func (sf StampedOpusFrame) Version() int {
	if len(sf) == 0 {
		return 0
	}
	return int(sf[0])
}

func (sf StampedOpusFrame) Stamp() EpochMillis {
	if len(sf) < opusFrameHeader {
		return 0
	}
	var buf [8]byte
	copy(buf[1:], sf[1:opusFrameHeader])
	return EpochMillis(binary.BigEndian.Uint64(buf[:]))
}

func (sf StampedOpusFrame) Frame() []byte {
	if len(sf) < opusFrameHeader {
		return nil
	}
	return sf[opusFrameHeader:]
}

func (sf StampedOpusFrame) Validate() error {
	if len(sf) < opusFrameHeader+1 {
		return ErrOpusFrameTooShort
	}
	if sf[0] != OpusFrameVersion {
		return ErrInvalidOpusFrameVersion
	}
	return nil
}

func StampOpusFrame(frame []byte, stamp EpochMillis) StampedOpusFrame {
	result := make([]byte, opusFrameHeader+len(frame))
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(stamp))
	buf[0] = OpusFrameVersion
	copy(result[:opusFrameHeader], buf[:])
	copy(result[opusFrameHeader:], frame)
	return StampedOpusFrame(result)
}

func ParseStampedOpusFrame(data []byte) (StampedOpusFrame, error) {
	sf := StampedOpusFrame(data)
	if err := sf.Validate(); err != nil {
		return nil, err
	}
	copyData := make([]byte, len(data))
	copy(copyData, data)
	return StampedOpusFrame(copyData), nil
}
