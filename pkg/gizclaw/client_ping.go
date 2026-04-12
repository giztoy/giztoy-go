package gizclaw

import (
	"context"
	"fmt"
	"time"
)

// PingResult holds the result of a peer.ping.
type PingResult struct {
	ServerTime time.Time
	RTT        time.Duration
	ClockDiff  time.Duration
}

// Ping sends a peer.ping RPC and returns NTP-style timing information.
func (c *Client) Ping() (*PingResult, error) {
	rpcClient, err := c.OpenRPC()
	if err != nil {
		return nil, fmt.Errorf("gizclaw: ping: %w", err)
	}
	defer func() { _ = rpcClient.Close() }()

	t1 := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ping, err := rpcClient.Ping(ctx, "ping")
	if err != nil {
		return nil, fmt.Errorf("gizclaw: ping: %w", err)
	}
	t4 := time.Now()

	rtt := t4.Sub(t1)
	serverTime := time.UnixMilli(ping.ServerTime)
	clientMid := t1.Add(rtt / 2)

	return &PingResult{
		ServerTime: serverTime,
		RTT:        rtt,
		ClockDiff:  serverTime.Sub(clientMid),
	}, nil
}
