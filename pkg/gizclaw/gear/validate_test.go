package gear

import (
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
)

func TestValidateGear(t *testing.T) {
	roleErr := validateGear(gearservice.Gear{
		PublicKey: "x",
		Role:      gearservice.GearRole("bad"),
		Status:    gearservice.GearStatusActive,
	})
	if roleErr == nil {
		t.Fatal("validateGear should fail on invalid role")
	}

	statusErr := validateGear(gearservice.Gear{
		PublicKey: "x",
		Role:      gearservice.GearRolePeer,
		Status:    gearservice.GearStatus("bad"),
	})
	if statusErr == nil {
		t.Fatal("validateGear should fail on invalid status")
	}
}

func TestValidateConfiguration(t *testing.T) {
	invalid := gearservice.GearFirmwareChannel("weird")
	if err := validateConfiguration(gearservice.Configuration{
		Firmware: &gearservice.FirmwareConfig{Channel: &invalid},
	}); err == nil {
		t.Fatal("validateConfiguration should reject invalid channel")
	}

	stable := gearservice.GearFirmwareChannel("stable")
	if err := validateConfiguration(gearservice.Configuration{
		Firmware: &gearservice.FirmwareConfig{Channel: &stable},
	}); err != nil {
		t.Fatalf("validateConfiguration stable err = %v", err)
	}
}
