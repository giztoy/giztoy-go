package gizclaw

import (
	"context"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/giznet"
	"github.com/giztoy/giztoy-go/pkg/kv"
	"testing"
)

func TestGearsSecurityPolicyAllowsAdminServicesForActiveAdmin(t *testing.T) {
	keyPair, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error = %v", err)
	}

	service := gears.NewService(gears.NewStore(kv.NewMemory(nil)), map[string]gears.RegistrationToken{
		"admin_default": {Role: gears.GearRoleAdmin},
	})
	if _, err := service.Register(context.Background(), gears.RegistrationRequest{
		PublicKey:         keyPair.Public.String(),
		RegistrationToken: "admin_default",
	}); err != nil {
		t.Fatalf("Register error = %v", err)
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

	service := gears.NewService(gears.NewStore(kv.NewMemory(nil)), map[string]gears.RegistrationToken{
		"admin_default": {Role: gears.GearRoleAdmin},
	})
	ctx := context.Background()
	if _, err := service.Register(ctx, gears.RegistrationRequest{
		PublicKey:         keyPair.Public.String(),
		RegistrationToken: "admin_default",
	}); err != nil {
		t.Fatalf("Register error = %v", err)
	}
	if _, err := service.Block(ctx, keyPair.Public.String()); err != nil {
		t.Fatalf("Block error = %v", err)
	}

	policy := GearsSecurityPolicy{Gears: service}
	if policy.AllowPeerService(keyPair.Public, ServiceAdmin) {
		t.Fatal("blocked admin should not allow admin service")
	}
	if policy.AllowPeerService(keyPair.Public, ServiceGear) {
		t.Fatal("blocked admin should not allow gear service")
	}
}
