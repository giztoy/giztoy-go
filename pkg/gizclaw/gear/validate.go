package gear

import (
	"fmt"
	"strings"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
)

func validateGear(gear gearservice.Gear) error {
	gear.PublicKey = normalizePublicKey(gear.PublicKey)
	if gear.PublicKey == "" {
		return fmt.Errorf("gear: empty public key")
	}
	if !gear.Role.Valid() {
		return fmt.Errorf("gear: invalid role %q", gear.Role)
	}
	if !gear.Status.Valid() {
		return fmt.Errorf("gear: invalid status %q", gear.Status)
	}
	return validateConfiguration(gear.Configuration)
}

func validateConfiguration(cfg gearservice.Configuration) error {
	channel := firmwareChannel(cfg)
	if channel == "" {
		return nil
	}
	switch channel {
	case "rollback", "stable", "beta", "testing":
		return nil
	default:
		return fmt.Errorf("gear: invalid firmware channel %q", channel)
	}
}

func normalizePublicKey(publicKey string) string {
	return strings.TrimSpace(publicKey)
}
