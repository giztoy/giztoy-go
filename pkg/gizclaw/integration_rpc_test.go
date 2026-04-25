package gizclaw_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/integration/testutil"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
)

func TestIntegrationRPCDialAndPing(t *testing.T) {
	ts := startTestServer(t)
	client := newTestClient(t, ts)

	var serverTime time.Time
	var rtt time.Duration
	var clockDiff time.Duration
	var secondServerTime time.Time
	var pingErr error
	if err := testutil.WaitUntil(testutil.ReadyTimeout, func() error {
		t1 := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ping, err := client.Ping(ctx, "ping")
		if err != nil {
			cancel()
			pingErr = err
			return err
		}
		ping2, err := client.Ping(ctx, "ping-2")
		cancel()
		if err != nil {
			pingErr = err
			return err
		}
		t4 := time.Now()
		rtt = t4.Sub(t1)
		serverTime = time.UnixMilli(ping.ServerTime)
		secondServerTime = time.UnixMilli(ping2.ServerTime)
		clientMid := t1.Add(rtt / 2)
		clockDiff = serverTime.Sub(clientMid)
		pingErr = nil
		return nil
	}); err != nil {
		t.Fatalf("Ping err=%v", pingErr)
	}
	if serverTime.IsZero() {
		t.Fatal("ServerTime is zero")
	}
	if secondServerTime.IsZero() {
		t.Fatal("second ServerTime is zero")
	}
	if secondServerTime.Before(serverTime) {
		t.Fatalf("second ServerTime %v is before first %v", secondServerTime, serverTime)
	}
	if rtt <= 0 {
		t.Fatalf("RTT=%v", rtt)
	}
	if clockDiff > time.Second || clockDiff < -time.Second {
		t.Fatalf("ClockDiff=%v (too large for localhost)", clockDiff)
	}
}

func TestIntegrationRPCReversePingClient(t *testing.T) {
	ts := startTestServer(t)
	client := newTestClient(t, ts)

	var clientTime time.Time
	var secondClientTime time.Time
	var pingErr error
	if err := testutil.WaitUntil(testutil.ReadyTimeout, func() error {
		manager := ts.server.Manager()
		if manager == nil {
			return fmt.Errorf("server manager not ready")
		}
		conn, ok := manager.ActivePeer(client.KeyPair.Public.String())
		if !ok {
			return fmt.Errorf("active peer not ready")
		}
		host := &gizclaw.GearPeer{
			Conn:    conn,
			Service: ts.server.PeerService(),
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ping, err := host.Ping(ctx, "reverse-ping")
		if err != nil {
			cancel()
			pingErr = err
			return err
		}
		ping2, err := host.Ping(ctx, "reverse-ping-2")
		cancel()
		if err != nil {
			pingErr = err
			return err
		}
		clientTime = time.UnixMilli(ping.ServerTime)
		secondClientTime = time.UnixMilli(ping2.ServerTime)
		pingErr = nil
		return nil
	}); err != nil {
		t.Fatalf("reverse Ping err=%v", pingErr)
	}
	if clientTime.IsZero() {
		t.Fatal("client ServerTime is zero")
	}
	if secondClientTime.IsZero() {
		t.Fatal("second client ServerTime is zero")
	}
	if secondClientTime.Before(clientTime) {
		t.Fatalf("second client ServerTime %v is before first %v", secondClientTime, clientTime)
	}
}
