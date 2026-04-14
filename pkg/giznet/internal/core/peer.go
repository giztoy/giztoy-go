package core

import (
	"bytes"

	"github.com/giztoy/giztoy-go/pkg/giznet/internal/noise"
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
		if protocol == ProtocolKCP {
			return u.sendKCP(peer, service, data)
		}
		return u.sendDirect(peer, protocol, data)
	}
	cfg.OnOutputError = func(_ noise.PublicKey, service uint64, err error) {
		u.kcpOutputErrors.Add(1)
		if userOutputError != nil {
			userOutputError(peer.pk, service, err)
		}
	}

	return NewServiceMux(peer.pk, cfg)
}

func (u *UDP) sendPayload(peer *peerState, protocol byte, payload []byte) error {
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

	plaintext := EncodePayload(protocol, payload)
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

// sendKCP sends KCP/service-mux traffic to a peer.
func (u *UDP) sendKCP(peer *peerState, service uint64, data []byte) error {
	payload := AppendVarint(nil, service)
	payload = append(payload, data...)
	return u.sendPayload(peer, ProtocolKCP, payload)
}

// sendDirect sends non-KCP traffic directly to a peer.
func (u *UDP) sendDirect(peer *peerState, protocol byte, data []byte) error {
	return u.sendPayload(peer, protocol, data)
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
