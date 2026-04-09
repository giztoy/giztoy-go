package client

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/internal/server"
	"github.com/giztoy/giztoy-go/internal/stores"
	itest "github.com/giztoy/giztoy-go/internal/testutil"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

var testServerAddrs sync.Map
var testServerRunErrs sync.Map

func startTestServer(t *testing.T) (*server.Server, context.CancelFunc) {
	t.Helper()
	dataDir := t.TempDir()
	return startTestServerWithConfig(t, server.Config{
		DataDir:    dataDir,
		ListenAddr: "127.0.0.1:0",
		Stores: map[string]stores.Config{
			"mem": {Kind: stores.KindKeyValue, Backend: "memory"},
			"fw":  {Kind: stores.KindFS, Backend: "filesystem", Dir: "firmware"},
		},
		Gears: server.GearsConfig{
			Store: "mem",
			RegistrationTokens: map[string]gears.RegistrationToken{
				"admin_default":  {Role: gears.GearRoleAdmin},
				"device_default": {Role: gears.GearRoleDevice},
			},
		},
		Depots: server.DepotsConfig{Store: "fw"},
	})
}

func startTestServerWithConfig(t *testing.T, cfg server.Config) (*server.Server, context.CancelFunc) {
	t.Helper()
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		attemptCfg := cfg
		if attemptCfg.ListenAddr == "" || strings.HasSuffix(attemptCfg.ListenAddr, ":0") {
			attemptCfg.ListenAddr = itest.AllocateUDPAddr(t)
		}

		srv, err := server.New(attemptCfg)
		if err != nil {
			t.Fatalf("server.New error: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		runErrCh := make(chan error, 1)
		go func() {
			runErrCh <- srv.Run(ctx)
		}()

		if err := waitForServerReady(attemptCfg.ListenAddr, srv.PublicKey(), runErrCh); err == nil {
			testServerAddrs.Store(srv, attemptCfg.ListenAddr)
			testServerRunErrs.Store(srv, runErrCh)
			t.Cleanup(func() {
				testServerAddrs.Delete(srv)
				testServerRunErrs.Delete(srv)
			})
			return srv, cancel
		} else {
			lastErr = err
		}

		cancel()
		select {
		case <-runErrCh:
		case <-time.After(500 * time.Millisecond):
		}
	}
	t.Fatalf("test server did not become ready: %v", lastErr)
	return nil, nil
}

func newTestClient(t *testing.T, srv *server.Server) *Client {
	t.Helper()
	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	c, err := Dial(key, testServerAddr(t, srv), srv.PublicKey())
	if err != nil {
		t.Fatalf("Dial error: %v", err)
	}
	waitForClientPublicReady(t, c)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func waitForTestServerReady(t *testing.T, srv *server.Server) {
	t.Helper()

	if err := waitForServerReady(testServerAddr(t, srv), srv.PublicKey(), testServerRunErrCh(t, srv)); err != nil {
		t.Fatal(err)
	}
}

func testServerAddr(t *testing.T, srv *server.Server) string {
	t.Helper()
	addr, ok := testServerAddrs.Load(srv)
	if !ok {
		t.Fatal("missing test server addr")
	}
	return addr.(string)
}

func testServerRunErrCh(t *testing.T, srv *server.Server) <-chan error {
	t.Helper()
	runErrCh, ok := testServerRunErrs.Load(srv)
	if !ok {
		t.Fatal("missing test server run error channel")
	}
	return runErrCh.(chan error)
}

func waitForServerReady(addr string, pk noise.PublicKey, errCh <-chan error) error {
	return itest.WaitUntil(itest.ReadyTimeout, func() error {
		select {
		case err := <-errCh:
			return fmt.Errorf("test server exited before ready: %w", err)
		default:
		}

		key, err := noise.GenerateKeyPair()
		if err != nil {
			return fmt.Errorf("GenerateKeyPair(ready check): %w", err)
		}
		c, err := Dial(key, addr, pk)
		if err == nil {
			infoErr := probeClientPublicReady(c)
			_ = c.Close()
			if infoErr == nil {
				return nil
			}
			return fmt.Errorf("probe client public ready: %w", infoErr)
		}
		return fmt.Errorf("dial ready check: %w", err)
	})
}

func waitForClientPublicReady(t *testing.T, c *Client) {
	t.Helper()

	if err := itest.WaitUntil(itest.ReadyTimeout, func() error {
		return probeClientPublicReady(c)
	}); err != nil {
		t.Fatalf("test client public service did not become ready: %v", err)
	}
}

func probeClientPublicReady(c *Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), itest.ProbeTimeout)
	defer cancel()
	_, err := c.GetServerInfo(ctx)
	return err
}
