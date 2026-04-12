package gizclaw

import (
	"context"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/giznet"
)

type GearsSecurityPolicy struct {
	Gears *gears.Service
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
	gear, err := p.Gears.Get(context.Background(), publicKey.String())
	if err != nil {
		return false
	}
	switch service {
	case ServiceAdmin, ServiceGear:
		return gear.Role == gears.GearRoleAdmin && gear.Status == gears.GearStatusActive
	default:
		return false
	}
}
