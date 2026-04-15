package gizclaw

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/gear"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/GizClaw/gizclaw-go/pkg/store/depotstore"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

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

	server := &Server{KeyPair: keyPair, GearStore: kv.NewMemory(nil)}
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

func TestServerPublicKeyAndServiceAccessors(t *testing.T) {
	keyPair, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error = %v", err)
	}

	service := &Service{}
	server := &Server{KeyPair: keyPair, service: service}
	if got := server.PublicKey(); got != keyPair.Public {
		t.Fatalf("PublicKey() = %v, want %v", got, keyPair.Public)
	}
	if got := server.Service(); got != service {
		t.Fatalf("Service() = %v, want %v", got, service)
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
}

func TestResolveGearTarget(t *testing.T) {
	ctx := context.Background()
	store := kv.NewMemory(nil)
	gearsServer := &gear.Server{Store: store}

	saveGear := func(t *testing.T, publicKey string, device gearservice.DeviceInfo, config gearservice.Configuration) {
		t.Helper()
		if _, err := gearsServer.SaveGear(ctx, gearservice.Gear{
			PublicKey:     publicKey,
			Role:          gearservice.GearRoleDevice,
			Status:        gearservice.GearStatusActive,
			Device:        device,
			Configuration: config,
		}); err != nil {
			t.Fatalf("SaveGear(%s) error = %v", publicKey, err)
		}
	}

	saveGear(t, "missing-depot", gearservice.DeviceInfo{}, gearservice.Configuration{
		Firmware: &gearservice.FirmwareConfig{Channel: func() *gearservice.GearFirmwareChannel {
			ch := gearservice.GearFirmwareChannel("stable")
			return &ch
		}()},
	})
	if _, _, err := resolveGearTarget(ctx, gearsServer, "missing-depot"); err == nil || !strings.Contains(err.Error(), "missing depot") {
		t.Fatalf("resolveGearTarget(missing depot) err = %v", err)
	}

	saveGear(t, "missing-channel", gearservice.DeviceInfo{
		Hardware: &gearservice.HardwareInfo{Depot: func() *string { v := "demo-main"; return &v }()},
	}, gearservice.Configuration{})
	if _, _, err := resolveGearTarget(ctx, gearsServer, "missing-channel"); err == nil || !strings.Contains(err.Error(), "missing channel") {
		t.Fatalf("resolveGearTarget(missing channel) err = %v", err)
	}

	if _, _, err := resolveGearTarget(ctx, gearsServer, "missing-gear"); !errors.Is(err, gear.ErrGearNotFound) {
		t.Fatalf("resolveGearTarget(missing gear) err = %v", err)
	}

	saveGear(t, "valid", gearservice.DeviceInfo{
		Hardware: &gearservice.HardwareInfo{Depot: func() *string { v := "demo-main"; return &v }()},
	}, gearservice.Configuration{
		Firmware: &gearservice.FirmwareConfig{Channel: func() *gearservice.GearFirmwareChannel {
			ch := gearservice.GearFirmwareChannel("stable")
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
