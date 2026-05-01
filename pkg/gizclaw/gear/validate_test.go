package gear

import (
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
)

func TestValidateGear(t *testing.T) {
	roleErr := validateGear(apitypes.Gear{
		PublicKey: "x",
		Role:      apitypes.GearRole("bad"),
		Status:    apitypes.GearStatusActive,
	})
	if roleErr == nil {
		t.Fatal("validateGear should fail on invalid role")
	}

	statusErr := validateGear(apitypes.Gear{
		PublicKey: "x",
		Role:      apitypes.GearRoleServer,
		Status:    apitypes.GearStatus("bad"),
	})
	if statusErr == nil {
		t.Fatal("validateGear should fail on invalid status")
	}
}

func TestValidateConfiguration(t *testing.T) {
	invalid := apitypes.GearFirmwareChannel("weird")
	if err := validateConfiguration(apitypes.Configuration{
		Firmware: &apitypes.FirmwareConfig{Channel: &invalid},
	}); err == nil {
		t.Fatal("validateConfiguration should reject invalid channel")
	}

	stable := apitypes.GearFirmwareChannel("stable")
	if err := validateConfiguration(apitypes.Configuration{
		Firmware: &apitypes.FirmwareConfig{Channel: &stable},
	}); err != nil {
		t.Fatalf("validateConfiguration stable err = %v", err)
	}
}
