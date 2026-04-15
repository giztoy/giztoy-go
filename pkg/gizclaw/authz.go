package gizclaw

import (
	"context"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/gear"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
)

type GearsSecurityPolicy struct {
	Gears *gear.Server
}

var _ SecurityPolicy = GearsSecurityPolicy{}

func (p GearsSecurityPolicy) AllowPeerService(publicKey giznet.PublicKey, service uint64) bool {
	switch service {
	case ServiceRPC, ServiceServerPublic:
		return true
	}
	if p.Gears == nil {
		return false
	}
	gear, err := p.Gears.LoadGear(context.Background(), publicKey.String())
	if err != nil {
		return false
	}
	switch service {
	case ServiceAdmin, ServiceGear:
		return gear.Role == gearservice.GearRoleAdmin && gear.Status == gearservice.GearStatusActive
	default:
		return false
	}
}
