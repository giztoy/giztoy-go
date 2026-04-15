package gizclaw

import (
	"context"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/gear"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

func TestGearsSecurityPolicyAllowsAdminServicesForActiveAdmin(t *testing.T) {
	keyPair, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error = %v", err)
	}

	service := &gear.Server{Store: kv.NewMemory(nil)}
	if _, err := service.SaveGear(context.Background(), gearservice.Gear{
		PublicKey:     keyPair.Public.String(),
		Role:          gearservice.GearRoleAdmin,
		Status:        gearservice.GearStatusActive,
		Device:        gearservice.DeviceInfo{},
		Configuration: gearservice.Configuration{},
	}); err != nil {
		t.Fatalf("SaveGear error = %v", err)
	}

	policy := GearsSecurityPolicy{Gears: service}
	if !policy.AllowPeerService(keyPair.Public, ServiceAdmin) {
		t.Fatal("admin policy should allow admin service")
	}
	if !policy.AllowPeerService(keyPair.Public, ServiceGear) {
		t.Fatal("admin policy should allow gear service")
	}
	if !policy.AllowPeerService(keyPair.Public, ServiceServerPublic) {
		t.Fatal("admin policy should allow server public service")
	}
	if policy.AllowPeerService(keyPair.Public, ServicePeerPublic) {
		t.Fatal("admin policy should not allow peer public service")
	}
	if policy.AllowPeerService(keyPair.Public, 0xffff) {
		t.Fatal("admin policy should not allow unknown service")
	}
}

func TestGearsSecurityPolicyAllowsPublicServicesWithoutGearLookup(t *testing.T) {
	policy := GearsSecurityPolicy{}
	if !policy.AllowPeerService(giznet.PublicKey{}, ServiceRPC) {
		t.Fatal("policy should allow rpc service")
	}
	if !policy.AllowPeerService(giznet.PublicKey{}, ServiceServerPublic) {
		t.Fatal("policy should allow server public service")
	}
}

func TestGearsSecurityPolicyDeniesAdminServicesForBlockedAdmin(t *testing.T) {
	keyPair, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error = %v", err)
	}

	service := &gear.Server{Store: kv.NewMemory(nil)}
	ctx := context.Background()
	if _, err := service.SaveGear(ctx, gearservice.Gear{
		PublicKey:     keyPair.Public.String(),
		Role:          gearservice.GearRoleAdmin,
		Status:        gearservice.GearStatusBlocked,
		Device:        gearservice.DeviceInfo{},
		Configuration: gearservice.Configuration{},
	}); err != nil {
		t.Fatalf("SaveGear error = %v", err)
	}

	policy := GearsSecurityPolicy{Gears: service}
	if policy.AllowPeerService(keyPair.Public, ServiceAdmin) {
		t.Fatal("blocked admin should not allow admin service")
	}
	if policy.AllowPeerService(keyPair.Public, ServiceGear) {
		t.Fatal("blocked admin should not allow gear service")
	}
}
