package gear

import (
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
)

func TestIndexDedupeHelpers(t *testing.T) {
	imeis := dedupeIMEIs([]apitypes.GearIMEI{
		{Tac: "2", Serial: "b"},
		{Tac: "1", Serial: "a"},
		{Tac: "1", Serial: "a"},
		{Tac: "", Serial: "skip"},
	})
	if len(imeis) != 2 || imeis[0].Tac != "1" || imeis[1].Tac != "2" {
		t.Fatalf("dedupeIMEIs = %+v", imeis)
	}

	labels := dedupeLabels([]apitypes.GearLabel{
		{Key: "b", Value: "2"},
		{Key: "a", Value: "1"},
		{Key: "a", Value: "1"},
		{Key: "", Value: "skip"},
	})
	if len(labels) != 2 || labels[0].Key != "a" || labels[1].Key != "b" {
		t.Fatalf("dedupeLabels = %+v", labels)
	}

	certs := dedupeCertifications([]apitypes.GearCertification{
		{Type: "license", Authority: "ce", Id: "2"},
		{Type: "license", Authority: "ce", Id: "1"},
		{Type: "license", Authority: "ce", Id: "1"},
		{Type: "", Authority: "ce", Id: "skip"},
	})
	if len(certs) != 2 || certs[0].Id != "1" || certs[1].Id != "2" {
		t.Fatalf("dedupeCertifications = %+v", certs)
	}
}

func TestIndexEntriesAndKeys(t *testing.T) {
	stable := apitypes.GearFirmwareChannel("stable")
	sn := "sn-index"
	depot := "depot-index"
	gear := apitypes.Gear{
		PublicKey: "peer-index",
		Role:      apitypes.GearRolePeer,
		Status:    apitypes.GearStatusActive,
		CreatedAt: time.Unix(1, 0),
		UpdatedAt: time.Unix(2, 0),
		Device: apitypes.DeviceInfo{
			Sn: &sn,
			Hardware: &apitypes.HardwareInfo{
				Depot:  &depot,
				Imeis:  &[]apitypes.GearIMEI{{Tac: "123", Serial: "456"}},
				Labels: &[]apitypes.GearLabel{{Key: "site", Value: "lab"}},
			},
		},
		Configuration: apitypes.Configuration{
			Certifications: &[]apitypes.GearCertification{{
				Type:      apitypes.GearCertificationType("license"),
				Authority: apitypes.GearCertificationAuthority("ce"),
				Id:        "cert-1",
			}},
			Firmware: &apitypes.FirmwareConfig{Channel: &stable},
		},
	}

	entries := indexEntries(gear)
	keys := indexKeys(gear)
	if len(entries) != 7 {
		t.Fatalf("entries len = %d", len(entries))
	}
	if len(keys) != 7 {
		t.Fatalf("keys len = %d", len(keys))
	}
	if snKey(sn).String() != "by-sn:sn-index" {
		t.Fatalf("snKey = %s", snKey(sn).String())
	}
	if firmwareKey(depot, stable, gear.PublicKey).String() != "by-firmware-depot:depot-index:stable:peer-index" {
		t.Fatalf("firmwareKey = %s", firmwareKey(depot, stable, gear.PublicKey).String())
	}
}
