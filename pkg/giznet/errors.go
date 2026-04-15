package giznet

import (
	"errors"

	"github.com/GizClaw/gizclaw-go/pkg/giznet/internal/core"
)

var (
	ErrNilListener = errors.New("giznet: nil listener")
	ErrNilConn     = errors.New("giznet: nil conn")
	ErrClosed      = errors.New("giznet: listener closed")
	ErrConnClosed  = errors.New("giznet: conn closed")

	ErrNoSession         = core.ErrNoSession
	ErrPeerNotFound      = core.ErrPeerNotFound
	ErrUDPClosed         = core.ErrClosed
	ErrAcceptQueueClosed = core.ErrAcceptQueueClosed
	ErrKCPMustUseStream  = core.ErrKCPMustUseStream
)
