package gear

import (
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
)

func TestConvertHelpers(t *testing.T) {
	now := time.Unix(1_700_600_000, 0).UTC()
	autoRegistered := true
	stable := apitypes.GearFirmwareChannel("stable")
	deviceName := "convert-device"
	gear := apitypes.Gear{
		PublicKey:      "peer-convert",
		Role:           apitypes.GearRoleServer,
		Status:         apitypes.GearStatusActive,
		AutoRegistered: &autoRegistered,
		CreatedAt:      now,
		UpdatedAt:      now,
		Configuration: apitypes.Configuration{
			Firmware: &apitypes.FirmwareConfig{Channel: &stable},
		},
		Device: apitypes.DeviceInfo{
			Name: &deviceName,
		},
	}

	registration := toGearRegistration(gear)
	if registration.PublicKey != gear.PublicKey || registration.Role != apitypes.GearRole(gear.Role) {
		t.Fatalf("toGearRegistration = %+v", registration)
	}

	publicRegistration := toPublicRegistration(gear)
	if publicRegistration.PublicKey != gear.PublicKey || publicRegistration.Role != apitypes.GearRole(gear.Role) {
		t.Fatalf("toPublicRegistration = %+v", publicRegistration)
	}

	cfg, err := toPublicConfiguration(gear.Configuration)
	if err != nil {
		t.Fatalf("toPublicConfiguration error: %v", err)
	}
	if cfg.Firmware == nil || cfg.Firmware.Channel == nil || *cfg.Firmware.Channel != apitypes.GearFirmwareChannel(stable) {
		t.Fatalf("toPublicConfiguration = %+v", cfg)
	}

	result, err := toPublicRegistrationResult(gear)
	if err != nil {
		t.Fatalf("toPublicRegistrationResult error: %v", err)
	}
	if result.Registration.PublicKey != gear.PublicKey || result.Gear.PublicKey != gear.PublicKey {
		t.Fatalf("toPublicRegistrationResult = %+v", result)
	}

	adminRegistrations := toAdminRegistrationList([]apitypes.Gear{gear}, false, nil)
	if len(adminRegistrations.Items) != 1 || adminRegistrations.Items[0].PublicKey != gear.PublicKey {
		t.Fatalf("toAdminRegistrationList = %+v", adminRegistrations)
	}

	adminOTA, err := toAdminOTASummary(apitypes.OTASummary{
		Depot:          "demo",
		Channel:        "stable",
		FirmwareSemver: "1.0.0",
		Files: []apitypes.DepotFile{{
			Path:   "bundles/fw.bin",
			Sha256: "sha256",
			Md5:    "md5",
		}},
	})
	if err != nil {
		t.Fatalf("toAdminOTASummary error: %v", err)
	}
	if adminOTA.Depot != "demo" || len(adminOTA.Files) != 1 {
		t.Fatalf("toAdminOTASummary = %+v", adminOTA)
	}

	convertedDevice, err := toGearDeviceInfo(gear.Device)
	if err != nil {
		t.Fatalf("toGearDeviceInfo error: %v", err)
	}
	if convertedDevice.Name == nil || *convertedDevice.Name != *gear.Device.Name {
		t.Fatalf("toGearDeviceInfo = %+v", convertedDevice)
	}

	adminDevice, err := toAdminDeviceInfo(apitypes.DeviceInfo{
		Name: gear.Device.Name,
		Sn:   gear.Device.Sn,
	})
	if err != nil {
		t.Fatalf("toAdminDeviceInfo error: %v", err)
	}
	if adminDevice.Name == nil || *adminDevice.Name != *gear.Device.Name {
		t.Fatalf("toAdminDeviceInfo = %+v", adminDevice)
	}

	adminRuntime := toAdminRuntime(apitypes.Runtime{Online: true, LastSeenAt: now})
	if !adminRuntime.Online || !adminRuntime.LastSeenAt.Equal(now) {
		t.Fatalf("toAdminRuntime = %+v", adminRuntime)
	}
}
