package gears

import (
	"context"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/kv"
)

func TestCanAccessMatrix(t *testing.T) {
	tests := []struct {
		name    string
		role    GearRole
		status  GearStatus
		service ServiceKind
		want    bool
	}{
		{name: "public bootstrap", role: GearRoleUnspecified, status: GearStatusBlocked, service: ServiceKindPublicDevice, want: true},
		{name: "admin active", role: GearRoleAdmin, status: GearStatusActive, service: ServiceKindAdmin, want: true},
		{name: "admin blocked", role: GearRoleAdmin, status: GearStatusBlocked, service: ServiceKindAdmin, want: false},
		{name: "peer active", role: GearRolePeer, status: GearStatusActive, service: ServiceKindPeer, want: true},
		{name: "device reverse active", role: GearRoleDevice, status: GearStatusActive, service: ServiceKindDeviceReverse, want: true},
		{name: "unknown service", role: GearRoleDevice, status: GearStatusActive, service: ServiceKind("other"), want: false},
	}
	for _, tt := range tests {
		if got := CanAccess(tt.role, tt.status, tt.service); got != tt.want {
			t.Fatalf("%s: CanAccess() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestValidationAndDedupeHelpers(t *testing.T) {
	if IsValidRole(GearRole("bad")) {
		t.Fatal("invalid role should be rejected")
	}
	if IsValidStatus(GearStatus("bad")) {
		t.Fatal("invalid status should be rejected")
	}
	if IsValidChannel(GearFirmwareChannel("bad")) {
		t.Fatal("invalid channel should be rejected")
	}

	imeis := dedupeIMEIs([]GearIMEI{
		{TAC: "1", Serial: "2"},
		{TAC: "1", Serial: "2"},
		{TAC: "", Serial: "3"},
	})
	if len(imeis) != 1 {
		t.Fatalf("dedupeIMEIs len = %d", len(imeis))
	}

	labels := dedupeLabels([]GearLabel{
		{Key: "batch", Value: "b1"},
		{Key: "batch", Value: "b1"},
		{Key: "", Value: "bad"},
	})
	if len(labels) != 1 {
		t.Fatalf("dedupeLabels len = %d", len(labels))
	}

	certs := dedupeCertifications([]GearCertification{
		{Type: GearCertificationTypeCertification, Authority: GearCertificationAuthorityCE, ID: "1"},
		{Type: GearCertificationTypeCertification, Authority: GearCertificationAuthorityCE, ID: "1"},
		{Type: "", Authority: GearCertificationAuthorityCE, ID: "2"},
	})
	if len(certs) != 1 {
		t.Fatalf("dedupeCertifications len = %d", len(certs))
	}

	imeis = dedupeIMEIs([]GearIMEI{
		{TAC: "a:", Serial: "b"},
		{TAC: "a", Serial: ":b"},
	})
	if len(imeis) != 2 {
		t.Fatalf("dedupeIMEIs with separators len = %d", len(imeis))
	}

	labels = dedupeLabels([]GearLabel{
		{Key: "batch:", Value: "b1"},
		{Key: "batch", Value: ":b1"},
	})
	if len(labels) != 2 {
		t.Fatalf("dedupeLabels with separators len = %d", len(labels))
	}

	certs = dedupeCertifications([]GearCertification{
		{Type: GearCertificationTypeCertification, Authority: GearCertificationAuthorityCE, ID: "id:1"},
		{Type: GearCertificationTypeCertification, Authority: GearCertificationAuthorityCE, ID: "id"},
	})
	if len(certs) != 2 {
		t.Fatalf("dedupeCertifications with separators len = %d", len(certs))
	}
}

func TestServiceListWrappersAndStoreClose(t *testing.T) {
	store := NewStore(kv.NewMemory(nil))
	svc := NewService(store, nil)
	ctx := context.Background()
	if _, err := svc.Register(ctx, RegistrationRequest{
		PublicKey: "list-pk",
		Device: DeviceInfo{
			SN: "sn-list",
			Hardware: HardwareInfo{
				Depot:  "demo",
				IMEIs:  []GearIMEI{{TAC: "123", Serial: "456"}},
				Labels: []GearLabel{{Key: "batch", Value: "b2"}},
			},
		},
	}); err != nil {
		t.Fatalf("Register error: %v", err)
	}
	if _, err := svc.PutConfig(ctx, "list-pk", Configuration{
		Certifications: []GearCertification{{
			Type:      GearCertificationTypeCertification,
			Authority: GearCertificationAuthorityCE,
			ID:        "ce-list",
		}},
		Firmware: FirmwareConfig{Channel: GearFirmwareChannelStable},
	}); err != nil {
		t.Fatalf("PutConfig error: %v", err)
	}

	if items, err := svc.ListByLabel(ctx, "batch", "b2"); err != nil || len(items) != 1 {
		t.Fatalf("ListByLabel = %d, %v", len(items), err)
	}
	if items, err := svc.ListByCertification(ctx, GearCertificationTypeCertification, GearCertificationAuthorityCE, "ce-list"); err != nil || len(items) != 1 {
		t.Fatalf("ListByCertification = %d, %v", len(items), err)
	}
	if items, err := svc.ListByFirmware(ctx, "demo", GearFirmwareChannelStable); err != nil || len(items) != 1 {
		t.Fatalf("ListByFirmware = %d, %v", len(items), err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

func TestStoreNormalizesPublicKeyBeforeCreateAndPut(t *testing.T) {
	store := NewStore(kv.NewMemory(nil))
	ctx := context.Background()

	if err := store.Create(ctx, Gear{
		PublicKey: "device-pk",
		Role:      GearRoleDevice,
		Status:    GearStatusActive,
		Configuration: Configuration{
			Firmware: FirmwareConfig{Channel: GearFirmwareChannelStable},
		},
		Device: DeviceInfo{
			SN: "sn-old",
			Hardware: HardwareInfo{
				Depot:  "demo-old",
				IMEIs:  []GearIMEI{{TAC: "12345678", Serial: "0000001"}},
				Labels: []GearLabel{{Key: "batch", Value: "old"}},
			},
		},
	}); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if err := store.Create(ctx, Gear{
		PublicKey: " device-pk ",
		Role:      GearRoleDevice,
		Status:    GearStatusActive,
		Configuration: Configuration{
			Firmware: FirmwareConfig{Channel: GearFirmwareChannelStable},
		},
	}); err != ErrGearAlreadyExists {
		t.Fatalf("Create duplicate err = %v, want %v", err, ErrGearAlreadyExists)
	}

	if err := store.Put(ctx, Gear{
		PublicKey: " device-pk ",
		Role:      GearRoleAdmin,
		Status:    GearStatusBlocked,
		Configuration: Configuration{
			Firmware: FirmwareConfig{Channel: GearFirmwareChannelBeta},
			Certifications: []GearCertification{{
				Type:      GearCertificationTypeCertification,
				Authority: GearCertificationAuthorityCE,
				ID:        "cert-new",
			}},
		},
		Device: DeviceInfo{
			SN: "sn-new",
			Hardware: HardwareInfo{
				Depot:  "demo-new",
				IMEIs:  []GearIMEI{{TAC: "87654321", Serial: "0000002"}},
				Labels: []GearLabel{{Key: "batch", Value: "new"}},
			},
		},
	}); err != nil {
		t.Fatalf("Put error: %v", err)
	}

	if _, err := store.ResolveBySN(ctx, "sn-old"); err != ErrGearNotFound {
		t.Fatalf("ResolveBySN old err = %v, want %v", err, ErrGearNotFound)
	}
	if publicKey, err := store.ResolveBySN(ctx, "sn-new"); err != nil || publicKey != "device-pk" {
		t.Fatalf("ResolveBySN new = %q, %v", publicKey, err)
	}
	if items, err := store.ListByLabel(ctx, "batch", "old"); err != nil || len(items) != 0 {
		t.Fatalf("ListByLabel old = %d, %v", len(items), err)
	}
	if items, err := store.ListByLabel(ctx, "batch", "new"); err != nil || len(items) != 1 {
		t.Fatalf("ListByLabel new = %d, %v", len(items), err)
	}
	if items, err := store.ListByFirmware(ctx, "demo-old", GearFirmwareChannelStable); err != nil || len(items) != 0 {
		t.Fatalf("ListByFirmware old = %d, %v", len(items), err)
	}
	if items, err := store.ListByFirmware(ctx, "demo-new", GearFirmwareChannelBeta); err != nil || len(items) != 1 {
		t.Fatalf("ListByFirmware new = %d, %v", len(items), err)
	}
}
