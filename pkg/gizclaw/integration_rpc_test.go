package gizclaw_test

import (
	"context"
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/integration/testutil"
)

func TestIntegrationRPCDialAndPing(t *testing.T) {
	ts := startTestServer(t)
	client := newTestClient(t, ts)

	var serverTime time.Time
	var rtt time.Duration
	var clockDiff time.Duration
	var pingErr error
	if err := testutil.WaitUntil(testutil.ReadyTimeout, func() error {
		rpcClient, err := client.RPCClient()
		if err != nil {
			pingErr = err
			return err
		}
		defer func() { _ = rpcClient.Close() }()

		t1 := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ping, err := rpcClient.Ping(ctx, "ping")
		cancel()
		if err != nil {
			pingErr = err
			return err
		}
		t4 := time.Now()
		rtt = t4.Sub(t1)
		serverTime = time.UnixMilli(ping.ServerTime)
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
	if rtt <= 0 {
		t.Fatalf("RTT=%v", rtt)
	}
	if clockDiff > time.Second || clockDiff < -time.Second {
		t.Fatalf("ClockDiff=%v (too large for localhost)", clockDiff)
	}
}
