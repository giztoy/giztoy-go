package pingrpcclientreuse_test

import (
	"context"
	"testing"
	"time"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestPingRPCClientReuseUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "003-ping-rpc-client-reuse")
	h.StartServerFromFixture("server_config.yaml")

	h.CreateContext("client-a").MustSucceed(t)

	client := h.ConnectClientFromContext("client-a")
	defer func() { _ = client.Close() }()

	rpcClient, err := client.RPCClient()
	if err != nil {
		t.Fatalf("open rpc client: %v", err)
	}
	defer func() { _ = rpcClient.Close() }()

	var previousServerTime time.Time
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ping, err := rpcClient.Ping(ctx, "ping-"+itoa(i))
		cancel()
		if err != nil {
			t.Fatalf("ping round %d failed: %v", i, err)
		}
		if ping == nil {
			t.Fatalf("ping round %d returned nil response", i)
		}

		serverTime := time.UnixMilli(ping.ServerTime)
		if serverTime.IsZero() {
			t.Fatalf("ping round %d returned zero server time", i)
		}
		if i > 0 && serverTime.Before(previousServerTime) {
			t.Fatalf("ping round %d server time %v went backwards from %v", i, serverTime, previousServerTime)
		}
		previousServerTime = serverTime
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
