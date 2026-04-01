package client

import (
	"context"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/gears"
)

func TestAdminGearsLifecycle(t *testing.T) {
	srv, cancel := startTestServer(t)
	defer cancel()

	admin := newTestClient(t, srv)
	adminResult, err := admin.Register(context.Background(), gears.RegistrationRequest{
		Device:            gears.DeviceInfo{Name: "admin"},
		RegistrationToken: "admin_default",
	})
	if err != nil {
		t.Fatalf("admin register error: %v", err)
	}

	device := newTestClient(t, srv)
	deviceResult, err := device.Register(context.Background(), gears.RegistrationRequest{
		Device: gears.DeviceInfo{
			Name: "device",
			SN:   "sn/1",
			Hardware: gears.HardwareInfo{
				Depot:  "demo/main",
				IMEIs:  []gears.GearIMEI{{Name: "main", TAC: "12345678", Serial: "0000001"}},
				Labels: []gears.GearLabel{{Key: "batch", Value: "cn/east"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("device register error: %v", err)
	}

	items, err := admin.ListGears(context.Background())
	if err != nil {
		t.Fatalf("ListGears error: %v", err)
	}
	if len(items) < 2 {
		t.Fatalf("ListGears returned %d items", len(items))
	}

	if _, err := admin.ApproveGear(context.Background(), deviceResult.Gear.PublicKey, gears.GearRoleDevice); err != nil {
		t.Fatalf("ApproveGear error: %v", err)
	}
	if _, err := admin.GetGear(context.Background(), deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("GetGear error: %v", err)
	}
	if publicKey, err := admin.ResolveGearBySN(context.Background(), "sn/1"); err != nil || publicKey != deviceResult.Gear.PublicKey {
		t.Fatalf("ResolveGearBySN = %q, %v", publicKey, err)
	}
	if publicKey, err := admin.ResolveGearByIMEI(context.Background(), "12345678", "0000001"); err != nil || publicKey != deviceResult.Gear.PublicKey {
		t.Fatalf("ResolveGearByIMEI = %q, %v", publicKey, err)
	}
	if _, err := admin.PutGearConfig(context.Background(), deviceResult.Gear.PublicKey, gears.Configuration{
		Certifications: []gears.GearCertification{{
			Type:      gears.GearCertificationTypeCertification,
			Authority: gears.GearCertificationAuthorityCE,
			ID:        "ce/001",
		}},
		Firmware: gears.FirmwareConfig{Channel: gears.GearFirmwareChannelStable},
	}); err != nil {
		t.Fatalf("PutGearConfig error: %v", err)
	}
	if _, err := admin.GetGearInfo(context.Background(), deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("GetGearInfo error: %v", err)
	}
	if _, err := admin.GetGearConfig(context.Background(), deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("GetGearConfig error: %v", err)
	}
	if _, err := admin.GetGearRuntime(context.Background(), deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("GetGearRuntime error: %v", err)
	}
	if _, err := admin.ListGearsByLabel(context.Background(), "batch", "cn/east"); err != nil {
		t.Fatalf("ListGearsByLabel error: %v", err)
	}
	if _, err := admin.ListGearsByCertification(context.Background(), gears.GearCertificationTypeCertification, gears.GearCertificationAuthorityCE, "ce/001"); err != nil {
		t.Fatalf("ListGearsByCertification error: %v", err)
	}
	if _, err := admin.ListGearsByFirmware(context.Background(), "demo/main", gears.GearFirmwareChannelStable); err != nil {
		t.Fatalf("ListGearsByFirmware error: %v", err)
	}
	if _, err := admin.BlockGear(context.Background(), deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("BlockGear error: %v", err)
	}
	if _, err := admin.DeleteGear(context.Background(), adminResult.Gear.PublicKey); err != nil {
		t.Fatalf("DeleteGear error: %v", err)
	}
}
