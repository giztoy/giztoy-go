package gizclaw

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/giznet"
)

func TestWebRTCRTPMillisDelta(t *testing.T) {
	tests := []struct {
		name      string
		clockRate uint32
		base      uint32
		timestamp uint32
		want      uint64
	}{
		{
			name:      "twenty milliseconds at opus clock rate",
			clockRate: webRTCOpusClockRate,
			base:      1000,
			timestamp: 1960,
			want:      20,
		},
		{
			name:      "timestamp wrap",
			clockRate: webRTCOpusClockRate,
			base:      ^uint32(479),
			timestamp: 480,
			want:      20,
		},
		{
			name:      "zero clock rate",
			clockRate: 0,
			base:      1000,
			timestamp: 1960,
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := webRTCRTPMillisDelta(tt.clockRate, tt.base, tt.timestamp)
			if got != tt.want {
				t.Fatalf("webRTCRTPMillisDelta() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestWebRTCOpusRTPTimestamp(t *testing.T) {
	tests := []struct {
		name          string
		stampedMillis uint64
		want          uint32
	}{
		{name: "twenty milliseconds", stampedMillis: 20, want: 960},
		{name: "wraps to uint32", stampedMillis: ((uint64(1) << 32) / 48) + 20, want: 944},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := webRTCOpusRTPTimestamp(tt.stampedMillis)
			if got != tt.want {
				t.Fatalf("webRTCOpusRTPTimestamp() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestIsWebRTCRPCDataChannel(t *testing.T) {
	tests := []struct {
		label string
		want  bool
	}{
		{label: "rpc", want: true},
		{label: "rpc:play-1", want: true},
		{label: "rpc-bootstrap", want: false},
		{label: "chat", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := isWebRTCRPCDataChannel(tt.label)
			if got != tt.want {
				t.Fatalf("isWebRTCRPCDataChannel(%q) = %t, want %t", tt.label, got, tt.want)
			}
		})
	}
}

func TestClientPeerPacketSubscriptionCopiesAndUnsubscribes(t *testing.T) {
	client := &Client{}
	packets, unsubscribe := client.subscribePeerPackets(ProtocolStampedOpus, 1)

	payload := []byte("frame")
	client.dispatchPeerPacket(ProtocolStampedOpus, payload)
	payload[0] = 'x'

	select {
	case got := <-packets:
		if !bytes.Equal(got, []byte("frame")) {
			t.Fatalf("packet = %q, want copied payload", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for packet")
	}

	unsubscribe()
	client.dispatchPeerPacket(ProtocolStampedOpus, []byte("dropped"))

	select {
	case got := <-packets:
		t.Fatalf("received packet after unsubscribe: %q", got)
	case <-time.After(10 * time.Millisecond):
	}
}

func TestIsPeerPacketReadClosed(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "eof", err: io.EOF, want: true},
		{name: "net closed", err: net.ErrClosed, want: true},
		{name: "conn closed", err: giznet.ErrConnClosed, want: true},
		{name: "udp closed", err: giznet.ErrUDPClosed, want: true},
		{name: "service mux closed", err: giznet.ErrServiceMuxClosed, want: true},
		{name: "wrapped", err: errors.Join(errors.New("read failed"), giznet.ErrUDPClosed), want: true},
		{name: "other", err: errors.New("boom"), want: false},
		{name: "nil", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPeerPacketReadClosed(tt.err)
			if got != tt.want {
				t.Fatalf("isPeerPacketReadClosed(%v) = %t, want %t", tt.err, got, tt.want)
			}
		})
	}
}
