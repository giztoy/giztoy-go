package gizclaw_test

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/giztoy/giztoy-go/internal/server"
	"github.com/giztoy/giztoy-go/internal/stores"
	itest "github.com/giztoy/giztoy-go/internal/testutil"
	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/gizclaw"
	"github.com/giztoy/giztoy-go/pkg/giznet"
	"strings"
	"testing"
	"time"
)

type testServer struct {
	server *server.Server
	addr   string
	errCh  chan error
}

func startTestServer(t *testing.T) *testServer {
	t.Helper()
	return startTestServerWithConfig(t, server.Config{
		DataDir:    t.TempDir(),
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

func startTestServerWithConfig(t *testing.T, cfg server.Config) *testServer {
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

		ts := &testServer{
			server: srv,
			addr:   attemptCfg.ListenAddr,
			errCh:  make(chan error, 1),
		}
		go func() {
			ts.errCh <- srv.ListenAndServe(giznet.WithBindAddr(attemptCfg.ListenAddr))
		}()

		if err := waitForServerReady(ts.addr, srv.PublicKey(), ts.errCh); err == nil {
			t.Cleanup(func() { _ = ts.server.Close() })
			return ts
		} else {
			lastErr = err
		}

		_ = srv.Close()
		select {
		case <-ts.errCh:
		case <-time.After(500 * time.Millisecond):
		}
	}

	t.Fatalf("test server did not become ready: %v", lastErr)
	return nil
}

func newTestClient(t *testing.T, ts *testServer) *gizclaw.Client {
	t.Helper()

	key, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error: %v", err)
	}
	c, err := gizclaw.Dial(key, ts.addr, ts.server.PublicKey())
	if err != nil {
		t.Fatalf("Dial error: %v", err)
	}
	waitForClientPublicReady(t, c)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func waitForServerReady(addr string, pk giznet.PublicKey, errCh <-chan error) error {
	return itest.WaitUntil(itest.ReadyTimeout, func() error {
		select {
		case err := <-errCh:
			return fmt.Errorf("test server exited before ready: %w", err)
		default:
		}

		key, err := giznet.GenerateKeyPair()
		if err != nil {
			return fmt.Errorf("GenerateKeyPair(ready check): %w", err)
		}
		c, err := gizclaw.Dial(key, addr, pk)
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

func waitForClientPublicReady(t *testing.T, c *gizclaw.Client) {
	t.Helper()
	if err := itest.WaitUntil(itest.ReadyTimeout, func() error {
		return probeClientPublicReady(c)
	}); err != nil {
		t.Fatalf("test client public service did not become ready: %v", err)
	}
}

func probeClientPublicReady(c *gizclaw.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), itest.ProbeTimeout)
	defer cancel()
	_, err := c.GetServerInfo(ctx)
	return err
}

func buildReleaseTar(t *testing.T, release firmware.DepotRelease, files map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	manifest, err := json.Marshal(release)
	if err != nil {
		t.Fatalf("marshal release: %v", err)
	}
	writeTarFile(t, tw, "manifest.json", manifest)
	for name, data := range files {
		writeTarFile(t, tw, name, data)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	return buf.Bytes()
}

func writeTarFile(t *testing.T, tw *tar.Writer, name string, data []byte) {
	t.Helper()
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("tar write: %v", err)
	}
}
