package adapters

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

const noiseWireNonceSize = 8

func newNoiseSessionPair() (a, b *noise.Session, err error) {
	initiatorStatic, err := noise.GenerateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("generate initiator key: %w", err)
	}
	responderStatic, err := noise.GenerateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("generate responder key: %w", err)
	}

	initiator, err := noise.NewHandshakeState(noise.Config{
		Pattern:      noise.PatternIK,
		Initiator:    true,
		LocalStatic:  initiatorStatic,
		RemoteStatic: &responderStatic.Public,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("new initiator hs: %w", err)
	}
	responder, err := noise.NewHandshakeState(noise.Config{
		Pattern:     noise.PatternIK,
		Initiator:   false,
		LocalStatic: responderStatic,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("new responder hs: %w", err)
	}

	msg1, err := initiator.WriteMessage(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("initiator write msg1: %w", err)
	}
	if _, err := responder.ReadMessage(msg1); err != nil {
		return nil, nil, fmt.Errorf("responder read msg1: %w", err)
	}

	msg2, err := responder.WriteMessage(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("responder write msg2: %w", err)
	}
	if _, err := initiator.ReadMessage(msg2); err != nil {
		return nil, nil, fmt.Errorf("initiator read msg2: %w", err)
	}

	sendI, recvI, err := initiator.Split()
	if err != nil {
		return nil, nil, fmt.Errorf("initiator split: %w", err)
	}
	sendR, recvR, err := responder.Split()
	if err != nil {
		return nil, nil, fmt.Errorf("responder split: %w", err)
	}

	idxI, err := noise.GenerateIndex()
	if err != nil {
		return nil, nil, fmt.Errorf("generate idxI: %w", err)
	}
	idxR, err := noise.GenerateIndex()
	if err != nil {
		return nil, nil, fmt.Errorf("generate idxR: %w", err)
	}

	a, err = noise.NewSession(noise.SessionConfig{
		LocalIndex:  idxI,
		RemoteIndex: idxR,
		SendKey:     sendI.Key(),
		RecvKey:     recvI.Key(),
		RemotePK:    responderStatic.Public,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("new session A: %w", err)
	}
	b, err = noise.NewSession(noise.SessionConfig{
		LocalIndex:  idxR,
		RemoteIndex: idxI,
		SendKey:     sendR.Key(),
		RecvKey:     recvR.Key(),
		RemotePK:    initiatorStatic.Public,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("new session B: %w", err)
	}

	return a, b, nil
}

func encodeNoiseTransport(s *noise.Session, payload []byte) ([]byte, error) {
	ct, nonce, err := s.Encrypt(payload)
	if err != nil {
		return nil, err
	}
	packet := make([]byte, noiseWireNonceSize+len(ct))
	binary.LittleEndian.PutUint64(packet[:noiseWireNonceSize], nonce)
	copy(packet[noiseWireNonceSize:], ct)
	return packet, nil
}

func decodeNoiseTransport(s *noise.Session, packet []byte) ([]byte, error) {
	if len(packet) < noiseWireNonceSize {
		return nil, errors.New("noise wire packet too short")
	}
	nonce := binary.LittleEndian.Uint64(packet[:noiseWireNonceSize])
	return s.Decrypt(packet[noiseWireNonceSize:], nonce)
}
