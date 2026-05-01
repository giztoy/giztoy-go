package gizclaw

import (
	"context"
	"errors"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/gear"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
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

func TestManagerEnsurePeerGearCreatesDefaultUnspecifiedGear(t *testing.T) {
	service := &gear.Server{Store: mustBadgerInMemory(t, nil)}
	manager := NewManager(service)
	ctx := context.Background()

	created, err := manager.EnsurePeerGear(ctx, "peer-pk")
	if err != nil {
		t.Fatalf("EnsurePeerGear error = %v", err)
	}
	if created.PublicKey != "peer-pk" {
		t.Fatalf("PublicKey = %q, want peer-pk", created.PublicKey)
	}
	if created.Role != apitypes.GearRoleUnspecified {
		t.Fatalf("Role = %q, want unspecified", created.Role)
	}
	if created.Status != apitypes.GearStatusActive {
		t.Fatalf("Status = %q, want active", created.Status)
	}
	if created.AutoRegistered == nil || !*created.AutoRegistered {
		t.Fatalf("AutoRegistered = %v, want true", created.AutoRegistered)
	}

	loaded, err := service.LoadGear(ctx, "peer-pk")
	if err != nil {
		t.Fatalf("LoadGear error = %v", err)
	}
	if loaded.Role != apitypes.GearRoleUnspecified || loaded.Status != apitypes.GearStatusActive {
		t.Fatalf("loaded gear = %+v", loaded)
	}
}

func TestManagerEnsurePeerGearPreservesExistingGear(t *testing.T) {
	service := &gear.Server{Store: mustBadgerInMemory(t, nil)}
	manager := NewManager(service)
	ctx := context.Background()
	if _, err := service.SaveGear(ctx, apitypes.Gear{
		PublicKey:     "peer-pk",
		Role:          apitypes.GearRoleAdmin,
		Status:        apitypes.GearStatusBlocked,
		Device:        apitypes.DeviceInfo{},
		Configuration: apitypes.Configuration{},
	}); err != nil {
		t.Fatalf("SaveGear error = %v", err)
	}

	got, err := manager.EnsurePeerGear(ctx, "peer-pk")
	if err != nil {
		t.Fatalf("EnsurePeerGear error = %v", err)
	}
	if got.Role != apitypes.GearRoleAdmin || got.Status != apitypes.GearStatusBlocked {
		t.Fatalf("EnsurePeerGear overwrote existing gear: %+v", got)
	}
}

func TestManagerRefreshDeviceErrors(t *testing.T) {
	service := &gear.Server{Store: mustBadgerInMemory(t, nil)}
	manager := NewManager(service)
	ctx := context.Background()

	if _, _, err := manager.RefreshGear(ctx, "missing"); !errors.Is(err, gear.ErrGearNotFound) {
		t.Fatalf("RefreshGear missing err = %v", err)
	}

	if _, err := service.SaveGear(ctx, apitypes.Gear{
		PublicKey:     "device-pk",
		Role:          apitypes.GearRoleUnspecified,
		Status:        apitypes.GearStatusUnspecified,
		Device:        apitypes.DeviceInfo{},
		Configuration: apitypes.Configuration{},
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
	gear := apitypes.Gear{
		Device: apitypes.DeviceInfo{
			Sn: &sn,
			Hardware: &apitypes.HardwareInfo{
				Imeis: &[]apitypes.GearIMEI{{
					Name:   &name,
					Tac:    "12345678",
					Serial: "0000001",
				}},
				Labels: &[]apitypes.GearLabel{{
					Key:   "batch",
					Value: "cn-east",
				}},
			},
		},
	}
	identifiers := apitypes.RefreshIdentifiers{
		Sn: &sn,
		Imeis: &[]apitypes.GearIMEI{{
			Name:   &name,
			Tac:    "12345678",
			Serial: "0000001",
		}},
		Labels: &[]apitypes.GearLabel{{
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
	gear := apitypes.Gear{
		Device: apitypes.DeviceInfo{
			Hardware: &apitypes.HardwareInfo{
				Imeis: &[]apitypes.GearIMEI{{
					Name:   &name,
					Tac:    "12345678",
					Serial: "0000001",
				}},
				Labels: &[]apitypes.GearLabel{{
					Key:   "batch",
					Value: "cn-east",
				}},
			},
		},
	}
	identifiers := apitypes.RefreshIdentifiers{
		Imeis: &[]apitypes.GearIMEI{{
			Name:   &nextName,
			Tac:    "87654321",
			Serial: "0000009",
		}},
		Labels: &[]apitypes.GearLabel{{
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
