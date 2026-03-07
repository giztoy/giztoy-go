package client

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/haivivi/giztoy/go/internal/server"
	"github.com/haivivi/giztoy/go/pkg/net/core"
	"github.com/haivivi/giztoy/go/pkg/net/noise"
	"github.com/haivivi/giztoy/go/pkg/net/peer"
)

// Client connects to a Giztoy server.
type Client struct {
	udp      *core.UDP
	listener *peer.Listener
	conn     *peer.Conn
	serverPK noise.PublicKey
}

// Dial connects to a server and performs the Noise handshake.
func Dial(localKey *noise.KeyPair, serverAddr string, serverPK noise.PublicKey) (*Client, error) {
	u, err := core.NewUDP(localKey,
		core.WithBindAddr("127.0.0.1:0"),
		core.WithAllowUnknown(true),
	)
	if err != nil {
		return nil, fmt.Errorf("client: udp: %w", err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		u.Close()
		return nil, fmt.Errorf("client: resolve addr: %w", err)
	}

	u.SetPeerEndpoint(serverPK, udpAddr)
	u.Connect(serverPK)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		info := u.PeerInfo(serverPK)
		if info != nil && info.State == core.PeerStateEstablished {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	info := u.PeerInfo(serverPK)
	if info == nil || info.State != core.PeerStateEstablished {
		u.Close()
		return nil, fmt.Errorf("client: handshake timeout")
	}

	l, err := peer.Wrap(u)
	if err != nil {
		u.Close()
		return nil, fmt.Errorf("client: wrap: %w", err)
	}

	conn, err := l.Peer(serverPK)
	if err != nil {
		l.Close()
		return nil, fmt.Errorf("client: peer: %w", err)
	}

	return &Client{udp: u, listener: l, conn: conn, serverPK: serverPK}, nil
}

// PingResult holds the result of a peer.ping.
type PingResult struct {
	ServerTime time.Time
	RTT        time.Duration
	ClockDiff  time.Duration
}

// Ping sends a peer.ping RPC and returns NTP-style timing information.
func (c *Client) Ping() (*PingResult, error) {
	stream, err := c.conn.OpenService(0)
	if err != nil {
		return nil, fmt.Errorf("client: open stream: %w", err)
	}
	defer stream.Close()

	t1 := time.Now()

	req := server.RPCRequest{V: 1, ID: "ping", Method: "peer.ping"}
	reqData, _ := json.Marshal(req)
	if err := server.WriteFrame(stream, reqData); err != nil {
		return nil, fmt.Errorf("client: write: %w", err)
	}

	respData, err := server.ReadFrame(stream)
	if err != nil {
		return nil, fmt.Errorf("client: read: %w", err)
	}

	t4 := time.Now()

	var resp server.RPCResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("client: unmarshal: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("client: server error: %s", resp.Error.Message)
	}

	var ping server.PingResponse
	if err := json.Unmarshal(resp.Result, &ping); err != nil {
		return nil, fmt.Errorf("client: unmarshal result: %w", err)
	}

	rtt := t4.Sub(t1)

	// NTP-style clock offset estimation:
	// t1 = client send time, t2 ≈ t3 = server time, t4 = client receive time
	// offset = server_time - (t1 + t4) / 2
	serverTime := time.UnixMilli(ping.ServerTime)
	clientMid := t1.Add(rtt / 2)
	clockDiff := serverTime.Sub(clientMid)

	return &PingResult{
		ServerTime: serverTime,
		RTT:        rtt,
		ClockDiff:  clockDiff,
	}, nil
}

// Close releases all resources including the underlying UDP socket.
func (c *Client) Close() error {
	if c.listener != nil {
		c.listener.Close()
	}
	if c.udp != nil {
		return c.udp.Close()
	}
	return nil
}
