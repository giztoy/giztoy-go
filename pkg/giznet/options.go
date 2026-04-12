package giznet

import "github.com/giztoy/giztoy-go/pkg/giznet/internal/core"

type Option = core.Option
type SocketConfig = core.SocketConfig
type ServiceMuxConfig = core.ServiceMuxConfig
type PeerState = core.PeerState
type PeerEvent = core.PeerEvent
type HostInfo = core.HostInfo
type UDP = core.UDP

const (
	PeerStateNew         = core.PeerStateNew
	PeerStateConnecting  = core.PeerStateConnecting
	PeerStateEstablished = core.PeerStateEstablished
	PeerStateFailed      = core.PeerStateFailed
)

var (
	WithBindAddr          = core.WithBindAddr
	WithAllowUnknown      = core.WithAllowUnknown
	WithDecryptWorkers    = core.WithDecryptWorkers
	WithRawChanSize       = core.WithRawChanSize
	WithDecryptedChanSize = core.WithDecryptedChanSize
	WithSocketConfig      = core.WithSocketConfig
	WithServiceMuxConfig  = core.WithServiceMuxConfig
	WithOnPeerEvent       = core.WithOnPeerEvent
)
