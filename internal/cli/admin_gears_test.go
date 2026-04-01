package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/gears"
)

func TestAdminGearsPutConfigMergesExistingConfig(t *testing.T) {
	original := openGearConfigClient
	fake := &fakeGearConfigClient{
		getCfg: gears.Configuration{
			Certifications: []gears.GearCertification{{
				Type:      gears.GearCertificationTypeCertification,
				Authority: gears.GearCertificationAuthorityCE,
				ID:        "ce-001",
			}},
			Firmware: gears.FirmwareConfig{Channel: gears.GearFirmwareChannelBeta},
		},
	}
	openGearConfigClient = func(string) (gearConfigClient, error) {
		return fake, nil
	}
	defer func() { openGearConfigClient = original }()

	cmd := newAdminGearsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"put-config", "device-pk", "stable"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if fake.putCfg.Firmware.Channel != gears.GearFirmwareChannelStable {
		t.Fatalf("channel = %q", fake.putCfg.Firmware.Channel)
	}
	if len(fake.putCfg.Certifications) != 1 || fake.putCfg.Certifications[0].ID != "ce-001" {
		t.Fatalf("certifications lost: %+v", fake.putCfg.Certifications)
	}

	var got gears.Configuration
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if got.Firmware.Channel != gears.GearFirmwareChannelStable {
		t.Fatalf("output channel = %q", got.Firmware.Channel)
	}
	if len(got.Certifications) != 1 {
		t.Fatalf("output certifications = %+v", got.Certifications)
	}
}

type fakeGearConfigClient struct {
	getCfg gears.Configuration
	putCfg gears.Configuration
}

func (f *fakeGearConfigClient) GetGearConfig(context.Context, string) (gears.Configuration, error) {
	return f.getCfg, nil
}

func (f *fakeGearConfigClient) PutGearConfig(_ context.Context, _ string, cfg gears.Configuration) (gears.Configuration, error) {
	f.putCfg = cfg
	return cfg, nil
}

func (f *fakeGearConfigClient) Close() error { return nil }
