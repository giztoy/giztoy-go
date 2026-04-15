package gear

import (
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
)

func TestConvertHelpers(t *testing.T) {
	now := time.Unix(1_700_600_000, 0).UTC()
	autoRegistered := true
	stable := gearservice.GearFirmwareChannel("stable")
	gear := gearservice.Gear{
		PublicKey:      "peer-convert",
		Role:           gearservice.GearRolePeer,
		Status:         gearservice.GearStatusActive,
		AutoRegistered: &autoRegistered,
		CreatedAt:      now,
		UpdatedAt:      now,
		Configuration: gearservice.Configuration{
			Firmware: &gearservice.FirmwareConfig{Channel: &stable},
		},
	}

	registration := toGearRegistration(gear)
	if registration.PublicKey != gear.PublicKey || registration.Role != gear.Role {
		t.Fatalf("toGearRegistration = %+v", registration)
	}

	publicRegistration := toPublicRegistration(gear)
	if publicRegistration.PublicKey != gear.PublicKey || publicRegistration.Role != serverpublic.GearRole(gear.Role) {
		t.Fatalf("toPublicRegistration = %+v", publicRegistration)
	}

	cfg, err := toPublicConfiguration(gear.Configuration)
	if err != nil {
		t.Fatalf("toPublicConfiguration error: %v", err)
	}
	if cfg.Firmware == nil || cfg.Firmware.Channel == nil || *cfg.Firmware.Channel != serverpublic.GearFirmwareChannel(stable) {
		t.Fatalf("toPublicConfiguration = %+v", cfg)
	}

	publicRuntime := toPublicRuntime(gearservice.Runtime{Online: true, LastSeenAt: now})
	if !publicRuntime.Online || !publicRuntime.LastSeenAt.Equal(now) {
		t.Fatalf("toPublicRuntime = %+v", publicRuntime)
	}

	result, err := toPublicRegistrationResult(gearservice.RegistrationResult{Gear: gear, Registration: registration})
	if err != nil {
		t.Fatalf("toPublicRegistrationResult error: %v", err)
	}
	if result.Registration.PublicKey != gear.PublicKey || result.Gear.PublicKey != gear.PublicKey {
		t.Fatalf("toPublicRegistrationResult = %+v", result)
	}
}
