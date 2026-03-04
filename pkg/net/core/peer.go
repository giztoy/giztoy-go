package core

import (
	"bytes"
	"net"

	"github.com/haivivi/giztoy/go/pkg/net/kcp"
	"github.com/haivivi/giztoy/go/pkg/net/noise"
)

// isKCPClient determines if we are the KCP client for a peer.
// Uses deterministic rule: smaller public key is client (uses odd stream IDs).
// This ensures consistent stream ID allocation regardless of who initiated the connection.
func (u *UDP) isKCPClient(remotePK noise.PublicKey) bool {
	return bytes.Compare(u.localKey.Public[:], remotePK[:]) < 0
}

// createServiceMux creates a new ServiceMux for a peer.
func (u *UDP) createServiceMux(peer *peerState) *kcp.ServiceMux {
	isClient := u.isKCPClient(peer.pk)

	return kcp.NewServiceMux(kcp.ServiceMuxConfig{
		IsClient: isClient,
		Output: func(service uint64, data []byte) error {
			return u.sendToPeerWithService(peer, noise.ProtocolRPC, service, data)
		},
		OnOutputError: func(_ uint64, _ error) {
			u.kcpOutputErrors.Add(1)
		},
	})
}

// sendToPeerWithService sends data to a peer with protocol + service.
func (u *UDP) sendToPeerWithService(peer *peerState, protocol byte, service uint64, data []byte) error {
	if service != 0 {
		return ErrUnsupportedService
	}
	return u.sendDirectWithService(peer, protocol, service, data)
}

// sendDirectWithService sends data directly to a peer with protocol + service.
func (u *UDP) sendDirectWithService(peer *peerState, protocol byte, service uint64, data []byte) error {
	if !noise.IsFoundationProtocol(protocol) {
		return ErrUnsupportedProtocol
	}

	peer.mu.RLock()
	session := peer.session
	endpoint := peer.endpoint
	peer.mu.RUnlock()

	if endpoint == nil {
		return ErrNoEndpoint
	}
	if session == nil {
		return ErrNoSession
	}

	plaintext := noise.EncodePayload(protocol, data)
	ciphertext, counter, err := session.Encrypt(plaintext)
	if err != nil {
		return err
	}

	msg := noise.BuildTransportMessage(session.RemoteIndex(), counter, ciphertext)

	n, err := u.socket.WriteToUDP(msg, endpoint)
	if err != nil {
		return err
	}

	u.totalTx.Add(uint64(n))
	peer.mu.Lock()
	peer.txBytes += uint64(n)
	peer.mu.Unlock()

	return nil
}

// sendToPeer sends data to a peer with the given protocol byte (service=0 default).
func (u *UDP) sendToPeer(peer *peerState, protocol byte, data []byte) error {
	return u.sendToPeerWithService(peer, protocol, 0, data)
}

// sendDirect sends data directly to a peer (service=0 default).
func (u *UDP) sendDirect(peer *peerState, protocol byte, data []byte) error {
	return u.sendDirectWithService(peer, protocol, 0, data)
}

// OpenStream opens a direct KCP stream to the specified peer on the given service.
//
// Compatibility note:
// Remote AcceptStream() is notified when inbound packets are observed on the
// service. Callers should write at least one payload after OpenStream to ensure
// the peer-side accept unblocks.
func (u *UDP) OpenStream(pk noise.PublicKey, service uint64) (net.Conn, error) {
	if service != 0 {
		return nil, ErrUnsupportedService
	}

	if u.closed.Load() || u.closing.Load() {
		return nil, ErrClosed
	}

	u.mu.RLock()
	peer, exists := u.peers[pk]
	u.mu.RUnlock()

	if !exists {
		return nil, ErrPeerNotFound
	}

	peer.mu.RLock()
	state := peer.state
	m := peer.serviceMux
	peer.mu.RUnlock()

	if state != PeerStateEstablished {
		return nil, ErrNoSession
	}
	if m == nil {
		return nil, ErrNoSession
	}

	return m.OpenStream(service)
}

// AcceptStream accepts an incoming direct KCP stream from the specified peer.
// Returns the stream, service ID, and any error.
//
// The accept is edge-triggered by inbound service activity. If the remote peer
// only opens a stream without writing data, AcceptStream remains blocked.
func (u *UDP) AcceptStream(pk noise.PublicKey) (net.Conn, uint64, error) {
	if u.closed.Load() || u.closing.Load() {
		return nil, 0, ErrClosed
	}

	u.mu.RLock()
	peer, exists := u.peers[pk]
	u.mu.RUnlock()

	if !exists {
		return nil, 0, ErrPeerNotFound
	}

	peer.mu.RLock()
	m := peer.serviceMux
	peer.mu.RUnlock()

	if m == nil {
		return nil, 0, ErrNoSession
	}

	return m.AcceptStream()
}

// closedChan returns a channel that's closed when UDP is closed.
func (u *UDP) closedChan() <-chan struct{} {
	return u.closeChan
}

// Read reads raw data from the specified peer (non-KCP protocols).
// Returns the protocol byte, number of bytes read, and any error.
func (u *UDP) Read(pk noise.PublicKey, buf []byte) (proto byte, n int, err error) {
	if u.closed.Load() || u.closing.Load() {
		return 0, 0, ErrClosed
	}

	u.mu.RLock()
	peer, exists := u.peers[pk]
	u.mu.RUnlock()

	if !exists {
		return 0, 0, ErrPeerNotFound
	}

	peer.mu.RLock()
	inboundChan := peer.inboundChan
	peer.mu.RUnlock()

	if inboundChan == nil {
		peer.mu.Lock()
		if peer.inboundChan == nil {
			peer.inboundChan = make(chan protoPacket, InboundChanSize)
		}
		inboundChan = peer.inboundChan
		peer.mu.Unlock()
	}

	select {
	case pkt := <-inboundChan:
		n = copy(buf, pkt.payload)
		return pkt.protocol, n, nil
	case <-u.closeChan:
		return 0, 0, ErrClosed
	}
}

// Write writes raw data to the specified peer with the given protocol byte.
func (u *UDP) Write(pk noise.PublicKey, proto byte, data []byte) (n int, err error) {
	if u.closed.Load() || u.closing.Load() {
		return 0, ErrClosed
	}
	if proto == noise.ProtocolRPC {
		return 0, ErrRPCMustUseStream
	}
	if !noise.IsFoundationProtocol(proto) {
		return 0, ErrUnsupportedProtocol
	}

	u.mu.RLock()
	peer, exists := u.peers[pk]
	u.mu.RUnlock()

	if !exists {
		return 0, ErrPeerNotFound
	}

	if err := u.sendToPeer(peer, proto, data); err != nil {
		return 0, err
	}

	return len(data), nil
}

// GetServiceMux returns the ServiceMux for a peer.
func (u *UDP) GetServiceMux(pk noise.PublicKey) (*kcp.ServiceMux, error) {
	if u.closed.Load() || u.closing.Load() {
		return nil, ErrClosed
	}

	u.mu.RLock()
	peer, exists := u.peers[pk]
	u.mu.RUnlock()

	if !exists {
		return nil, ErrPeerNotFound
	}

	peer.mu.RLock()
	m := peer.serviceMux
	peer.mu.RUnlock()

	if m == nil {
		return nil, ErrNoSession
	}

	return m, nil
}
