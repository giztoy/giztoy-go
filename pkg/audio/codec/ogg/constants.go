package ogg

const (
	// CapturePattern is the fixed page sync marker in the OGG framing spec.
	CapturePattern = "OggS"

	pageHeaderSize  = 27
	maxPageSegments = 255
	maxSegmentSize  = 255

	// HeaderTypeContinued marks a continuation page.
	HeaderTypeContinued byte = 0x01
	// HeaderTypeBOS marks beginning of stream.
	HeaderTypeBOS byte = 0x02
	// HeaderTypeEOS marks end of stream.
	HeaderTypeEOS byte = 0x04

	// GranulePositionUnknown is the sentinel used on non-terminal pages.
	GranulePositionUnknown = ^uint64(0)
)
