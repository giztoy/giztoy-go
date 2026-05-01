package gizclaw

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/gear"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/GizClaw/gizclaw-go/pkg/store/depotstore"
)

type countCloser struct {
	calls int
}

func (c *countCloser) Close() error {
	c.calls++
	return nil
}

func TestServerListenAndServeRequiresGearStore(t *testing.T) {
	keyPair, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error = %v", err)
	}

	server := &Server{KeyPair: keyPair, DepotStore: depotstore.Dir(t.TempDir())}
	err = server.ListenAndServe(nil)
	if !errors.Is(err, ErrNilGearStore) {
		t.Fatalf("ListenAndServe error = %v, want %v", err, ErrNilGearStore)
	}
}

func TestServerListenAndServeRequiresDepotStore(t *testing.T) {
	keyPair, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error = %v", err)
	}

	server := &Server{KeyPair: keyPair, GearStore: mustBadgerInMemory(t, nil)}
	err = server.ListenAndServe(nil)
	if !errors.Is(err, ErrNilDepotStore) {
		t.Fatalf("ListenAndServe error = %v, want %v", err, ErrNilDepotStore)
	}
}

func TestAllowAllAllowsPeerService(t *testing.T) {
	var policy SecurityPolicy = AllowAll{}
	if !policy.AllowPeerService(giznet.PublicKey{}, ServiceAdmin) {
		t.Fatal("AllowAll should allow admin service")
	}
	if !policy.AllowPeerService(giznet.PublicKey{}, 0xffff) {
		t.Fatal("AllowAll should allow arbitrary service")
	}
}

func TestServerListenAndServeValidatesReceiverAndKeyPair(t *testing.T) {
	t.Run("nil server", func(t *testing.T) {
		var server *Server
		if err := server.ListenAndServe(nil); err == nil || !strings.Contains(err.Error(), "nil server") {
			t.Fatalf("ListenAndServe(nil) err = %v", err)
		}
	})

	t.Run("nil key pair", func(t *testing.T) {
		server := &Server{}
		if err := server.ListenAndServe(nil); err == nil || !strings.Contains(err.Error(), "nil key pair") {
			t.Fatalf("ListenAndServe(nil key pair) err = %v", err)
		}
	})
}

func TestServerPublicKeyAndPeerServiceAccessors(t *testing.T) {
	keyPair, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error = %v", err)
	}

	service := &PeerService{}
	server := &Server{KeyPair: keyPair, peerService: service}
	if got := server.PublicKey(); got != keyPair.Public {
		t.Fatalf("PublicKey() = %v, want %v", got, keyPair.Public)
	}
	if got := server.PeerService(); got != service {
		t.Fatalf("PeerService() = %v, want %v", got, service)
	}

	listenerKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair(listener) error = %v", err)
	}
	listener, err := giznet.Listen(listenerKey, giznet.WithBindAddr("127.0.0.1:0"), giznet.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("giznet.Listen error = %v", err)
	}
	defer listener.Close()

	server = &Server{}
	server.setListener(listener)
	if got := server.PublicKey(); got != listener.HostInfo().PublicKey {
		t.Fatalf("PublicKey() from listener = %v, want %v", got, listener.HostInfo().PublicKey)
	}
}

func TestServerCloseClosesStoreCloser(t *testing.T) {
	closer := &countCloser{}
	server := &Server{StoreCloser: closer}

	if err := server.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if closer.calls != 1 {
		t.Fatalf("StoreCloser calls = %d, want 1", closer.calls)
	}
	if err := server.Close(); err != nil {
		t.Fatalf("Close second: %v", err)
	}
	if closer.calls != 1 {
		t.Fatalf("StoreCloser calls after second close = %d, want 1", closer.calls)
	}
}

func TestServerAllowPeerServiceDefaults(t *testing.T) {
	var nilServer *Server
	if nilServer.allowPeerService(giznet.PublicKey{}, ServiceRPC) {
		t.Fatal("nil server should deny all services")
	}

	server := &Server{}
	if !server.allowPeerService(giznet.PublicKey{}, ServiceRPC) {
		t.Fatal("server should allow rpc before manager is initialized")
	}
	if !server.allowPeerService(giznet.PublicKey{}, ServiceServerPublic) {
		t.Fatal("server should allow server public before manager is initialized")
	}
	if server.allowPeerService(giznet.PublicKey{}, ServiceAdmin) {
		t.Fatal("server should not allow admin before manager is initialized")
	}

	keyPair, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error = %v", err)
	}
	server.AdminPublicKey = keyPair.Public.String()
	if !server.allowPeerService(keyPair.Public, ServiceAdmin) {
		t.Fatal("configured admin public key should allow admin before manager is initialized")
	}
}

func TestResolveGearTarget(t *testing.T) {
	ctx := context.Background()
	store := mustBadgerInMemory(t, nil)
	gearsServer := &gear.Server{Store: store}

	saveGear := func(t *testing.T, publicKey string, device apitypes.DeviceInfo, config apitypes.Configuration) {
		t.Helper()
		if _, err := gearsServer.SaveGear(ctx, apitypes.Gear{
			PublicKey:     publicKey,
			Role:          apitypes.GearRoleGear,
			Status:        apitypes.GearStatusActive,
			Device:        device,
			Configuration: config,
		}); err != nil {
			t.Fatalf("SaveGear(%s) error = %v", publicKey, err)
		}
	}

	saveGear(t, "missing-depot", apitypes.DeviceInfo{}, apitypes.Configuration{
		Firmware: &apitypes.FirmwareConfig{Channel: func() *apitypes.GearFirmwareChannel {
			ch := apitypes.GearFirmwareChannel("stable")
			return &ch
		}()},
	})
	if _, _, err := resolveGearTarget(ctx, gearsServer, "missing-depot"); err == nil || !strings.Contains(err.Error(), "missing depot") {
		t.Fatalf("resolveGearTarget(missing depot) err = %v", err)
	}

	saveGear(t, "missing-channel", apitypes.DeviceInfo{
		Hardware: &apitypes.HardwareInfo{Depot: func() *string { v := "demo-main"; return &v }()},
	}, apitypes.Configuration{})
	if _, _, err := resolveGearTarget(ctx, gearsServer, "missing-channel"); err == nil || !strings.Contains(err.Error(), "missing channel") {
		t.Fatalf("resolveGearTarget(missing channel) err = %v", err)
	}

	if _, _, err := resolveGearTarget(ctx, gearsServer, "missing-gear"); !errors.Is(err, gear.ErrGearNotFound) {
		t.Fatalf("resolveGearTarget(missing gear) err = %v", err)
	}

	saveGear(t, "valid", apitypes.DeviceInfo{
		Hardware: &apitypes.HardwareInfo{Depot: func() *string { v := "demo-main"; return &v }()},
	}, apitypes.Configuration{
		Firmware: &apitypes.FirmwareConfig{Channel: func() *apitypes.GearFirmwareChannel {
			ch := apitypes.GearFirmwareChannel("stable")
			return &ch
		}()},
	})
	depot, channel, err := resolveGearTarget(ctx, gearsServer, "valid")
	if err != nil {
		t.Fatalf("resolveGearTarget(valid) err = %v", err)
	}
	if depot != "demo-main" || channel != "stable" {
		t.Fatalf("resolveGearTarget(valid) = (%q, %q)", depot, channel)
	}
}
