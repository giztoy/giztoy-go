package core

import (
	"bytes"

	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

// isKCPClient determines if we are the KCP client for a peer.
// Uses deterministic rule: smaller public key is client (uses odd stream IDs).
// This ensures consistent stream ID allocation regardless of who initiated the connection.
func (u *UDP) isKCPClient(remotePK noise.PublicKey) bool {
	return bytes.Compare(u.localKey.Public[:], remotePK[:]) < 0
}

// createServiceMux creates a new ServiceMux for a peer.
func (u *UDP) createServiceMux(peer *peerState) *ServiceMux {
	isClient := u.isKCPClient(peer.pk)
	cfg := u.serviceMuxConfig
	userOutputError := cfg.OnOutputError

	cfg.IsClient = isClient
	cfg.Output = func(_ noise.PublicKey, service uint64, protocol byte, data []byte) error {
		return u.sendToPeerWithService(peer, protocol, service, data)
	}
	cfg.OnOutputError = func(_ noise.PublicKey, service uint64, err error) {
		u.kcpOutputErrors.Add(1)
		if userOutputError != nil {
			userOutputError(peer.pk, service, err)
		}
	}

	return NewServiceMux(peer.pk, cfg)
}

// sendToPeerWithService sends data to a peer with protocol + service.
func (u *UDP) sendToPeerWithService(peer *peerState, protocol byte, service uint64, data []byte) error {
	return u.sendDirectWithService(peer, protocol, service, data)
}

// sendDirectWithService sends data directly to a peer with protocol + service.
// For stream protocols, the service ID is encoded as a varint prefix in the payload
// so the receiver can route to the correct ServiceMux entry.
func (u *UDP) sendDirectWithService(peer *peerState, protocol byte, service uint64, data []byte) error {
	if !IsFoundationProtocol(protocol) {
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

	wirePayload := data
	if IsStreamProtocol(protocol) {
		wirePayload = noise.AppendVarint(nil, service)
		wirePayload = append(wirePayload, data...)
	}

	plaintext := noise.EncodePayload(protocol, wirePayload)
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

// GetServiceMux returns the ServiceMux for a peer.
func (u *UDP) GetServiceMux(pk noise.PublicKey) (*ServiceMux, error) {
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

// closedChan returns a channel that's closed when UDP is closed.
func (u *UDP) closedChan() <-chan struct{} {
	return u.closeChan
}
