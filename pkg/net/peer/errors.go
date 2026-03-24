package peer

import "errors"

var (
	ErrNilListener             = errors.New("peer: nil listener")
	ErrNilConn                 = errors.New("peer: nil conn")
	ErrClosed                  = errors.New("peer: listener closed")
	ErrConnClosed              = errors.New("peer: conn closed")
	ErrOpusFrameTooShort       = errors.New("peer: opus frame too short")
	ErrInvalidOpusFrameVersion = errors.New("peer: invalid opus frame version")
	ErrInvalidV                = errors.New("peer: invalid version")
	ErrMissingID               = errors.New("peer: missing id")
	ErrMissingName             = errors.New("peer: missing name")
	ErrMissingMethod           = errors.New("peer: missing method")
	ErrRPCErrorMessageRequired = errors.New("peer: rpc error message is required")
)
