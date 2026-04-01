package gears

import (
	"context"
	"errors"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/kv"
)

func TestServiceRegisterApproveBlockDeleteRefresh(t *testing.T) {
	store := NewStore(kv.NewMemory(nil))
	svc := NewService(store, map[string]RegistrationToken{
		"device_default": {Role: GearRoleDevice},
	})
	ctx := context.Background()

	result, err := svc.Register(ctx, RegistrationRequest{
		PublicKey:         "device-pk",
		RegistrationToken: "device_default",
		Device: DeviceInfo{
			Name: "dev-1",
			SN:   "sn-1",
			Hardware: HardwareInfo{
				Depot:          "demo",
				FirmwareSemVer: "1.0.0",
				IMEIs: []GearIMEI{
					{Name: "main", TAC: "12345678", Serial: "0000001"},
				},
				Labels: []GearLabel{{Key: "batch", Value: "b1"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}
	if result.Gear.Role != GearRoleDevice || result.Gear.Status != GearStatusActive {
		t.Fatalf("Register role/status = %q/%q", result.Gear.Role, result.Gear.Status)
	}

	resolvedBySN, err := svc.ResolveBySN(ctx, "sn-1")
	if err != nil {
		t.Fatalf("ResolveBySN error: %v", err)
	}
	if resolvedBySN.PublicKey != "device-pk" {
		t.Fatalf("ResolveBySN public key = %q", resolvedBySN.PublicKey)
	}

	resolvedByIMEI, err := svc.ResolveByIMEI(ctx, "12345678", "0000001")
	if err != nil {
		t.Fatalf("ResolveByIMEI error: %v", err)
	}
	if resolvedByIMEI.PublicKey != "device-pk" {
		t.Fatalf("ResolveByIMEI public key = %q", resolvedByIMEI.PublicKey)
	}

	if _, err := svc.PutConfig(ctx, "device-pk", Configuration{
		Firmware: FirmwareConfig{Channel: GearFirmwareChannelStable},
	}); err != nil {
		t.Fatalf("PutConfig error: %v", err)
	}

	refreshResult, err := svc.Refresh(ctx, "device-pk", RefreshPatch{
		Info: &RefreshInfo{
			Manufacturer:     "Acme",
			Model:            "M1",
			HardwareRevision: "rev-a",
		},
		Version: &RefreshVersion{
			Depot:          "demo",
			FirmwareSemVer: "1.1.0",
		},
	})
	if err != nil {
		t.Fatalf("Refresh error: %v", err)
	}
	if refreshResult.Gear.Device.Hardware.Manufacturer != "Acme" {
		t.Fatalf("Refresh manufacturer = %q", refreshResult.Gear.Device.Hardware.Manufacturer)
	}

	if _, err := svc.PutInfo(ctx, "device-pk", DeviceInfo{
		Name: "dev-2",
		SN:   "sn-2",
		Hardware: HardwareInfo{
			Manufacturer: "Acme",
			Model:        "M2",
			Depot:        "demo",
			Labels:       []GearLabel{{Key: "batch", Value: "b1"}},
		},
	}); err != nil {
		t.Fatalf("PutInfo error: %v", err)
	}

	if _, err := svc.Approve(ctx, "device-pk", GearRoleAdmin); err != nil {
		t.Fatalf("Approve error: %v", err)
	}

	items, err := svc.List(ctx, ListOptions{})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("List returned %d items", len(items))
	}

	labelItems, err := store.ListByLabel(ctx, "batch", "b1")
	if err != nil {
		t.Fatalf("ListByLabel error: %v", err)
	}
	if len(labelItems) != 1 {
		t.Fatalf("ListByLabel returned %d items", len(labelItems))
	}

	if _, err := svc.PutConfig(ctx, "device-pk", Configuration{
		Certifications: []GearCertification{{
			Type:      GearCertificationTypeCertification,
			Authority: GearCertificationAuthorityCE,
			ID:        "ce-001",
		}},
		Firmware: FirmwareConfig{Channel: GearFirmwareChannelStable},
	}); err != nil {
		t.Fatalf("PutConfig(cert) error: %v", err)
	}

	roleItems, err := store.ListByRole(ctx, GearRoleAdmin)
	if err != nil {
		t.Fatalf("ListByRole error: %v", err)
	}
	if len(roleItems) != 1 {
		t.Fatalf("ListByRole returned %d items", len(roleItems))
	}

	statusItems, err := store.ListByStatus(ctx, GearStatusActive)
	if err != nil {
		t.Fatalf("ListByStatus error: %v", err)
	}
	if len(statusItems) != 1 {
		t.Fatalf("ListByStatus returned %d items", len(statusItems))
	}

	firmwareItems, err := store.ListByFirmware(ctx, "demo", GearFirmwareChannelStable)
	if err != nil {
		t.Fatalf("ListByFirmware error: %v", err)
	}
	if len(firmwareItems) != 1 {
		t.Fatalf("ListByFirmware returned %d items", len(firmwareItems))
	}

	certItems, err := store.ListByCertification(ctx, GearCertificationTypeCertification, GearCertificationAuthorityCE, "ce-001")
	if err != nil {
		t.Fatalf("ListByCertification error: %v", err)
	}
	if len(certItems) != 1 {
		t.Fatalf("ListByCertification returned %d items", len(certItems))
	}

	blocked, err := svc.Block(ctx, "device-pk")
	if err != nil {
		t.Fatalf("Block error: %v", err)
	}
	if blocked.Status != GearStatusBlocked {
		t.Fatalf("Blocked status = %q", blocked.Status)
	}

	deleted, err := svc.Delete(ctx, "device-pk")
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if deleted.Role != GearRoleUnspecified || deleted.Status != GearStatusUnspecified {
		t.Fatalf("Delete role/status = %q/%q", deleted.Role, deleted.Status)
	}

	if !CanAccess(GearRoleAdmin, GearStatusActive, ServiceKindAdmin) {
		t.Fatal("admin should access admin service")
	}
	if CanAccess(GearRoleDevice, GearStatusBlocked, ServiceKindDeviceApp) {
		t.Fatal("blocked device should not access device app service")
	}
}

func TestRefreshFromProvider(t *testing.T) {
	store := NewStore(kv.NewMemory(nil))
	svc := NewService(store, nil)
	ctx := context.Background()
	if _, err := svc.Register(ctx, RegistrationRequest{
		PublicKey: "provider-pk",
		Device:    DeviceInfo{},
	}); err != nil {
		t.Fatalf("Register error: %v", err)
	}
	result, err := svc.RefreshFromProvider(ctx, "provider-pk", stubProvider{})
	if err != nil {
		t.Fatalf("RefreshFromProvider error: %v", err)
	}
	if result.Gear.Device.Hardware.Depot != "demo" {
		t.Fatalf("depot = %q", result.Gear.Device.Hardware.Depot)
	}
}

func TestRefreshFromProviderPartialSuccess(t *testing.T) {
	store := NewStore(kv.NewMemory(nil))
	svc := NewService(store, nil)
	ctx := context.Background()
	if _, err := svc.Register(ctx, RegistrationRequest{
		PublicKey: "provider-partial-pk",
		Device:    DeviceInfo{},
	}); err != nil {
		t.Fatalf("Register error: %v", err)
	}
	result, err := svc.RefreshFromProvider(ctx, "provider-partial-pk", partialStubProvider{})
	if err != nil {
		t.Fatalf("RefreshFromProvider error: %v", err)
	}
	if result.Gear.Device.Hardware.Depot != "demo" {
		t.Fatalf("depot = %q", result.Gear.Device.Hardware.Depot)
	}
	if result.Errors["identifiers"] == "" {
		t.Fatal("expected identifiers error")
	}
}

func TestRefreshFromProviderAllFail(t *testing.T) {
	store := NewStore(kv.NewMemory(nil))
	svc := NewService(store, nil)
	ctx := context.Background()
	if _, err := svc.Register(ctx, RegistrationRequest{
		PublicKey: "provider-fail-pk",
		Device:    DeviceInfo{},
	}); err != nil {
		t.Fatalf("Register error: %v", err)
	}
	if _, err := svc.RefreshFromProvider(ctx, "provider-fail-pk", failingStubProvider{}); !errors.Is(err, ErrNoRefreshData) {
		t.Fatalf("RefreshFromProvider err = %v, want %v", err, ErrNoRefreshData)
	}
}

func TestServiceIndexesSupportSeparatorCharacters(t *testing.T) {
	store := NewStore(kv.NewMemory(nil))
	svc := NewService(store, map[string]RegistrationToken{
		"device_default": {Role: GearRoleDevice},
	})
	ctx := context.Background()

	if _, err := svc.Register(ctx, RegistrationRequest{
		PublicKey:         "device-sep-pk",
		RegistrationToken: "device_default",
		Device: DeviceInfo{
			SN: "sn:001",
			Hardware: HardwareInfo{
				Depot:  "demo:main",
				IMEIs:  []GearIMEI{{TAC: "1234:5678", Serial: "0000:001"}},
				Labels: []GearLabel{{Key: "region:cn", Value: "hz:01"}},
			},
		},
	}); err != nil {
		t.Fatalf("Register with separators error: %v", err)
	}
	if _, err := svc.PutConfig(ctx, "device-sep-pk", Configuration{
		Certifications: []GearCertification{{
			Type:      GearCertificationTypeCertification,
			Authority: GearCertificationAuthorityCE,
			ID:        "cert:001",
		}},
		Firmware: FirmwareConfig{Channel: GearFirmwareChannelStable},
	}); err != nil {
		t.Fatalf("PutConfig with separators error: %v", err)
	}

	if item, err := svc.ResolveBySN(ctx, "sn:001"); err != nil || item.PublicKey != "device-sep-pk" {
		t.Fatalf("ResolveBySN = %+v, %v", item, err)
	}
	if item, err := svc.ResolveByIMEI(ctx, "1234:5678", "0000:001"); err != nil || item.PublicKey != "device-sep-pk" {
		t.Fatalf("ResolveByIMEI = %+v, %v", item, err)
	}
	if items, err := svc.ListByLabel(ctx, "region:cn", "hz:01"); err != nil || len(items) != 1 {
		t.Fatalf("ListByLabel = %d, %v", len(items), err)
	}
	if items, err := svc.ListByCertification(ctx, GearCertificationTypeCertification, GearCertificationAuthorityCE, "cert:001"); err != nil || len(items) != 1 {
		t.Fatalf("ListByCertification = %d, %v", len(items), err)
	}
	if items, err := svc.ListByFirmware(ctx, "demo:main", GearFirmwareChannelStable); err != nil || len(items) != 1 {
		t.Fatalf("ListByFirmware = %d, %v", len(items), err)
	}
}

func TestRegisterRejectsInvalidRoleFromRegistrationToken(t *testing.T) {
	store := NewStore(kv.NewMemory(nil))
	svc := NewService(store, map[string]RegistrationToken{
		"device_default": {Role: GearRole("devcie")},
	})

	if _, err := svc.Register(context.Background(), RegistrationRequest{
		PublicKey:         "bad-role-pk",
		RegistrationToken: "device_default",
		Device:            DeviceInfo{Name: "bad-role"},
	}); err == nil {
		t.Fatal("Register should fail for invalid token role")
	}
	if _, err := svc.Get(context.Background(), "bad-role-pk"); !errors.Is(err, ErrGearNotFound) {
		t.Fatalf("Get err = %v, want %v", err, ErrGearNotFound)
	}
}

type stubProvider struct{}

func (stubProvider) GetInfo(context.Context, string) (RefreshInfo, error) {
	return RefreshInfo{Name: "stub", Manufacturer: "Acme"}, nil
}

func (stubProvider) GetIdentifiers(context.Context, string) (RefreshIdentifiers, error) {
	return RefreshIdentifiers{SN: "stub-sn"}, nil
}

func (stubProvider) GetVersion(context.Context, string) (RefreshVersion, error) {
	return RefreshVersion{Depot: "demo", FirmwareSemVer: "1.0.0"}, nil
}

type partialStubProvider struct{}

func (partialStubProvider) GetInfo(context.Context, string) (RefreshInfo, error) {
	return RefreshInfo{Name: "partial", Manufacturer: "Acme"}, nil
}

func (partialStubProvider) GetIdentifiers(context.Context, string) (RefreshIdentifiers, error) {
	return RefreshIdentifiers{}, errors.New("identifiers unavailable")
}

func (partialStubProvider) GetVersion(context.Context, string) (RefreshVersion, error) {
	return RefreshVersion{Depot: "demo", FirmwareSemVer: "1.0.1"}, nil
}

type failingStubProvider struct{}

func (failingStubProvider) GetInfo(context.Context, string) (RefreshInfo, error) {
	return RefreshInfo{}, errors.New("info unavailable")
}

func (failingStubProvider) GetIdentifiers(context.Context, string) (RefreshIdentifiers, error) {
	return RefreshIdentifiers{}, errors.New("identifiers unavailable")
}

func (failingStubProvider) GetVersion(context.Context, string) (RefreshVersion, error) {
	return RefreshVersion{}, errors.New("version unavailable")
}
