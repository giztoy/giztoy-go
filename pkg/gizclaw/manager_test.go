package gizclaw

import (
	"context"
	"errors"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/giznet"
	"github.com/giztoy/giztoy-go/pkg/kv"
)

func TestManagerMarkPeerOfflineDeletesActivePeer(t *testing.T) {
	manager := &Manager{}
	conn := &giznet.Conn{}

	manager.MarkPeerOnline("device-pk", conn)
	if runtime := manager.PeerRuntime(context.Background(), "device-pk"); !runtime.Online {
		t.Fatalf("peer should be online before offline mark: %+v", runtime)
	}

	manager.MarkPeerOffline("device-pk", conn)
	if _, ok := manager.ActivePeer("device-pk"); ok {
		t.Fatal("peer should be removed after disconnect")
	}
	if runtime := manager.PeerRuntime(context.Background(), "device-pk"); runtime.Online || runtime.LastSeenAt != 0 {
		t.Fatalf("runtime after removal = %+v", runtime)
	}
}

func TestManagerMarkPeerOfflineKeepsNewerConnection(t *testing.T) {
	manager := &Manager{}
	oldConn := &giznet.Conn{}
	newConn := &giznet.Conn{}

	manager.MarkPeerOnline("device-pk", oldConn)
	manager.MarkPeerOnline("device-pk", newConn)
	manager.MarkPeerOffline("device-pk", oldConn)

	got, ok := manager.ActivePeer("device-pk")
	if !ok || got != newConn {
		t.Fatalf("ActivePeer after old disconnect = %v, %v", got, ok)
	}
	if runtime := manager.PeerRuntime(context.Background(), "device-pk"); !runtime.Online {
		t.Fatalf("runtime after old disconnect = %+v", runtime)
	}
}

func TestManagerRefreshDeviceErrors(t *testing.T) {
	service := gears.NewService(gears.NewStore(kv.NewMemory(nil)), nil)
	manager := NewManager(service)
	ctx := context.Background()

	if _, _, err := manager.RefreshDevice(ctx, "missing"); !errors.Is(err, gears.ErrGearNotFound) {
		t.Fatalf("RefreshDevice missing err = %v", err)
	}

	if _, err := service.Register(ctx, gears.RegistrationRequest{
		PublicKey: "device-pk",
		Device:    gears.DeviceInfo{},
	}); err != nil {
		t.Fatalf("Register error: %v", err)
	}

	if _, online, err := manager.RefreshDevice(ctx, "device-pk"); !errors.Is(err, ErrDeviceOffline) {
		t.Fatalf("RefreshDevice offline err = %v", err)
	} else if online {
		t.Fatal("offline RefreshDevice should report online=false")
	}
}
