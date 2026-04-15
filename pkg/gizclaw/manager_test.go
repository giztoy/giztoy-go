package gizclaw

import (
	"context"
	"errors"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/peerpublic"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/gear"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
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
	if runtime := manager.PeerRuntime(context.Background(), "device-pk"); runtime.Online || !runtime.LastSeenAt.IsZero() {
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
	service := &gear.Server{Store: kv.NewMemory(nil)}
	manager := NewManager(service)
	ctx := context.Background()

	if _, _, err := manager.RefreshGear(ctx, "missing"); !errors.Is(err, gear.ErrGearNotFound) {
		t.Fatalf("RefreshGear missing err = %v", err)
	}

	if _, err := service.SaveGear(ctx, gearservice.Gear{
		PublicKey:     "device-pk",
		Role:          gearservice.GearRoleUnspecified,
		Status:        gearservice.GearStatusUnspecified,
		Device:        gearservice.DeviceInfo{},
		Configuration: gearservice.Configuration{},
	}); err != nil {
		t.Fatalf("SaveGear error: %v", err)
	}

	if _, online, err := manager.RefreshGear(ctx, "device-pk"); !errors.Is(err, ErrDeviceOffline) {
		t.Fatalf("RefreshGear offline err = %v", err)
	} else if online {
		t.Fatal("offline RefreshGear should report online=false")
	}
}

func TestApplyPeerRefreshIdentifiersSkipsUnchangedCollections(t *testing.T) {
	name := "primary"
	sn := "sn-1"
	gear := gearservice.Gear{
		Device: gearservice.DeviceInfo{
			Sn: &sn,
			Hardware: &gearservice.HardwareInfo{
				Imeis: &[]gearservice.GearIMEI{{
					Name:   &name,
					Tac:    "12345678",
					Serial: "0000001",
				}},
				Labels: &[]gearservice.GearLabel{{
					Key:   "batch",
					Value: "cn-east",
				}},
			},
		},
	}
	identifiers := peerpublic.RefreshIdentifiers{
		Sn: &sn,
		Imeis: &[]peerpublic.GearIMEI{{
			Name:   &name,
			Tac:    "12345678",
			Serial: "0000001",
		}},
		Labels: &[]peerpublic.GearLabel{{
			Key:   "batch",
			Value: "cn-east",
		}},
	}

	var updatedFields []string
	applyPeerRefreshIdentifiers(&gear, identifiers, &updatedFields)

	if len(updatedFields) != 0 {
		t.Fatalf("applyPeerRefreshIdentifiers() updatedFields = %v, want none", updatedFields)
	}
}

func TestApplyPeerRefreshIdentifiersUpdatesChangedCollections(t *testing.T) {
	name := "primary"
	nextName := "secondary"
	gear := gearservice.Gear{
		Device: gearservice.DeviceInfo{
			Hardware: &gearservice.HardwareInfo{
				Imeis: &[]gearservice.GearIMEI{{
					Name:   &name,
					Tac:    "12345678",
					Serial: "0000001",
				}},
				Labels: &[]gearservice.GearLabel{{
					Key:   "batch",
					Value: "cn-east",
				}},
			},
		},
	}
	identifiers := peerpublic.RefreshIdentifiers{
		Imeis: &[]peerpublic.GearIMEI{{
			Name:   &nextName,
			Tac:    "87654321",
			Serial: "0000009",
		}},
		Labels: &[]peerpublic.GearLabel{{
			Key:   "batch",
			Value: "cn-west",
		}},
	}

	var updatedFields []string
	applyPeerRefreshIdentifiers(&gear, identifiers, &updatedFields)

	if len(updatedFields) != 2 {
		t.Fatalf("applyPeerRefreshIdentifiers() updatedFields = %v, want 2 entries", updatedFields)
	}
	if gear.Device.Hardware == nil || gear.Device.Hardware.Imeis == nil || (*gear.Device.Hardware.Imeis)[0].Tac != "87654321" {
		t.Fatalf("IMEIs not updated: %+v", gear.Device.Hardware)
	}
	if gear.Device.Hardware.Labels == nil || (*gear.Device.Hardware.Labels)[0].Value != "cn-west" {
		t.Fatalf("labels not updated: %+v", gear.Device.Hardware)
	}
}

func TestIsPeerDisconnectedError(t *testing.T) {
	t.Run("closed connection errors are offline", func(t *testing.T) {
		if !isPeerDisconnectedError(errors.New("gizhttp: read response: kcp: conn closed: local")) {
			t.Fatal("conn closed error should be treated as disconnected")
		}
	})

	t.Run("generic read response errors stay online", func(t *testing.T) {
		if isPeerDisconnectedError(errors.New("gizhttp: read response: malformed HTTP response")) {
			t.Fatal("generic read response error should not be treated as disconnected")
		}
	})
}
