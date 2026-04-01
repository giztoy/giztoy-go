package client

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/giztoy/giztoy-go/internal/server"
	"github.com/giztoy/giztoy-go/pkg/net/core"
	"github.com/giztoy/giztoy-go/pkg/net/httptransport"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
	"github.com/giztoy/giztoy-go/pkg/net/peer"
)

// Client connects to a Giztoy server.
type Client struct {
	listener *peer.Listener
	conn     *peer.Conn
	serverPK noise.PublicKey
}

// Dial connects to a server and performs the Noise handshake.
func Dial(localKey *noise.KeyPair, serverAddr string, serverPK noise.PublicKey) (*Client, error) {
	c := &Client{serverPK: serverPK}

	l, err := peer.Listen(localKey,
		core.WithBindAddr("127.0.0.1:0"),
		core.WithAllowUnknown(true),
		core.WithServiceMuxConfig(core.ServiceMuxConfig{
			OnNewService: func(_ noise.PublicKey, service uint64) bool {
				return service == peer.ServicePublic || service == peer.ServiceReverse
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("client: listen: %w", err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		l.Close()
		return nil, fmt.Errorf("client: resolve addr: %w", err)
	}

	conn, err := l.Dial(serverPK, udpAddr)
	if err != nil {
		l.Close()
		return nil, fmt.Errorf("client: dial: %w", err)
	}

	c.listener = l
	c.conn = conn

	return c, nil
}

// PingResult holds the result of a peer.ping.
type PingResult struct {
	ServerTime time.Time
	RTT        time.Duration
	ClockDiff  time.Duration
}

// Ping sends a peer.ping RPC and returns NTP-style timing information.
func (c *Client) Ping() (*PingResult, error) {
	stream, err := c.conn.OpenService(peer.ServicePublic)
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
	if c.conn != nil {
		c.conn.Close()
	}
	if c.listener != nil {
		return c.listener.Close()
	}
	return nil
}

func (c *Client) HTTPClient(service uint64) *http.Client {
	return httptransport.NewClient(c.conn, service)
}

func (c *Client) PeerConn() *peer.Conn {
	return c.conn
}

