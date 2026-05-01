package gizclaw_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
)

func TestIntegrationGearServiceLifecycle(t *testing.T) {
	ts := startTestServer(t)

	admin := newTestClient(t, ts)
	adminResult, err := register(context.Background(), admin, gearservice.RegistrationRequest{
		Device: apitypes.DeviceInfo{Name: strPtr("admin")},
	})
	if err != nil {
		t.Fatalf("admin register error: %v", err)
	}

	device := newTestClient(t, ts)
	deviceResult, err := register(context.Background(), device, gearservice.RegistrationRequest{
		Device: apitypes.DeviceInfo{
			Name: strPtr("gear"),
			Sn:   strPtr("sn/1"),
			Hardware: &apitypes.HardwareInfo{
				Depot: strPtr("demo-main"),
				Imeis: &[]apitypes.GearIMEI{{Name: strPtr("main"), Tac: "12345678", Serial: "0000001"}},
				Labels: &[]apitypes.GearLabel{{
					Key:   "batch",
					Value: "cn/east",
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("device register error: %v", err)
	}

	items, err := listGears(context.Background(), admin)
	if err != nil {
		t.Fatalf("ListGears error: %v", err)
	}
	if len(items) < 2 {
		t.Fatalf("ListGears returned %d items", len(items))
	}

	if _, err := approveGear(context.Background(), admin, deviceResult.Gear.PublicKey, apitypes.GearRoleGear); err != nil {
		t.Fatalf("ApproveGear error: %v", err)
	}
	if _, err := getGear(context.Background(), admin, deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("GetGear error: %v", err)
	}
	if publicKey, err := resolveGearBySN(context.Background(), admin, "sn/1"); err != nil || publicKey != deviceResult.Gear.PublicKey {
		t.Fatalf("ResolveGearBySN = %q, %v", publicKey, err)
	}
	if publicKey, err := resolveGearByIMEI(context.Background(), admin, "12345678", "0000001"); err != nil || publicKey != deviceResult.Gear.PublicKey {
		t.Fatalf("ResolveGearByIMEI = %q, %v", publicKey, err)
	}
	if _, err := putGearConfig(context.Background(), admin, deviceResult.Gear.PublicKey, apitypes.Configuration{
		Certifications: &[]apitypes.GearCertification{{
			Type:      apitypes.GearCertificationType("certification"),
			Authority: apitypes.GearCertificationAuthority("ce"),
			Id:        "ce/001",
		}},
		Firmware: &apitypes.FirmwareConfig{Channel: func() *apitypes.GearFirmwareChannel {
			ch := apitypes.GearFirmwareChannel("stable")
			return &ch
		}()},
	}); err != nil {
		t.Fatalf("PutGearConfig error: %v", err)
	}
	if _, err := getGearInfo(context.Background(), admin, deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("GetGearInfo error: %v", err)
	}
	if _, err := getGearConfig(context.Background(), admin, deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("GetGearConfig error: %v", err)
	}
	if _, err := getGearRuntime(context.Background(), admin, deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("GetGearRuntime error: %v", err)
	}
	if _, err := listGearsByLabel(context.Background(), admin, "batch", "cn/east"); err != nil {
		t.Fatalf("ListGearsByLabel error: %v", err)
	}
	if _, err := listGearsByCertification(context.Background(), admin, apitypes.GearCertificationType("certification"), apitypes.GearCertificationAuthority("ce"), "ce/001"); err != nil {
		t.Fatalf("ListGearsByCertification error: %v", err)
	}
	if _, err := listGearsByFirmware(context.Background(), admin, "demo-main", apitypes.GearFirmwareChannel("stable")); err != nil {
		t.Fatalf("ListGearsByFirmware error: %v", err)
	}
	if _, err := blockGear(context.Background(), admin, deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("BlockGear error: %v", err)
	}
	if _, err := deleteGear(context.Background(), admin, adminResult.Gear.PublicKey); err != nil {
		t.Fatalf("DeleteGear error: %v", err)
	}
}

func TestIntegrationAdminResourceAPIs(t *testing.T) {
	ts := startTestServer(t)

	admin := newTestClient(t, ts)
	if _, err := register(context.Background(), admin, gearservice.RegistrationRequest{
		Device: apitypes.DeviceInfo{Name: strPtr("admin")},
	}); err != nil {
		t.Fatalf("admin register error: %v", err)
	}

	api, err := admin.ServerAdminClient()
	if err != nil {
		t.Fatalf("ServerAdminClient error: %v", err)
	}

	missingResp, err := api.GetResourceWithResponse(context.Background(), apitypes.ResourceKindCredential, "missing")
	if err != nil {
		t.Fatalf("GetResourceWithResponse(missing) error: %v", err)
	}
	if missingResp.JSON404 == nil || missingResp.JSON404.Error.Code != "RESOURCE_NOT_FOUND" {
		t.Fatalf("GetResource missing response status=%d body=%s", missingResp.StatusCode(), string(missingResp.Body))
	}

	resource := mustAdminResource(t, `{
		"apiVersion": "gizclaw.admin/v1alpha1",
		"kind": "Credential",
		"metadata": {"name": "minimax-main"},
		"spec": {
			"provider": "minimax",
			"method": "api_key",
			"body": {"api_key": "secret"}
		}
	}`)

	applyResp, err := api.ApplyResourceWithResponse(context.Background(), resource)
	if err != nil {
		t.Fatalf("ApplyResourceWithResponse(create) error: %v", err)
	}
	if applyResp.JSON200 == nil || applyResp.JSON200.Action != apitypes.ApplyActionCreated {
		t.Fatalf("ApplyResource create response status=%d body=%s", applyResp.StatusCode(), string(applyResp.Body))
	}

	getResp, err := api.GetResourceWithResponse(context.Background(), apitypes.ResourceKindCredential, "minimax-main")
	if err != nil {
		t.Fatalf("GetResourceWithResponse error: %v", err)
	}
	if getResp.JSON200 == nil {
		t.Fatalf("GetResource response status=%d body=%s", getResp.StatusCode(), string(getResp.Body))
	}

	updatedResource := mustAdminResource(t, `{
		"apiVersion": "gizclaw.admin/v1alpha1",
		"kind": "Credential",
		"metadata": {"name": "minimax-main"},
		"spec": {
			"provider": "minimax",
			"method": "api_key",
			"description": "updated credential",
			"body": {"api_key": "secret"}
		}
	}`)
	updatedResp, err := api.ApplyResourceWithResponse(context.Background(), updatedResource)
	if err != nil {
		t.Fatalf("ApplyResourceWithResponse(update) error: %v", err)
	}
	if updatedResp.JSON200 == nil || updatedResp.JSON200.Action != apitypes.ApplyActionUpdated {
		t.Fatalf("ApplyResource update response status=%d body=%s", updatedResp.StatusCode(), string(updatedResp.Body))
	}

	putResp, err := api.PutResourceWithResponse(context.Background(), apitypes.ResourceKindCredential, "minimax-main", updatedResource)
	if err != nil {
		t.Fatalf("PutResourceWithResponse error: %v", err)
	}
	if putResp.JSON200 == nil {
		t.Fatalf("PutResource response status=%d body=%s", putResp.StatusCode(), string(putResp.Body))
	}

	deleteResp, err := api.DeleteResourceWithResponse(context.Background(), apitypes.ResourceKindCredential, "minimax-main")
	if err != nil {
		t.Fatalf("DeleteResourceWithResponse error: %v", err)
	}
	if deleteResp.JSON200 == nil {
		t.Fatalf("DeleteResource response status=%d body=%s", deleteResp.StatusCode(), string(deleteResp.Body))
	}
	getAfterDeleteResp, err := api.GetResourceWithResponse(context.Background(), apitypes.ResourceKindCredential, "minimax-main")
	if err != nil {
		t.Fatalf("GetResourceWithResponse(after delete) error: %v", err)
	}
	if getAfterDeleteResp.JSON404 == nil || getAfterDeleteResp.JSON404.Error.Code != "RESOURCE_NOT_FOUND" {
		t.Fatalf("GetResource after delete response status=%d body=%s", getAfterDeleteResp.StatusCode(), string(getAfterDeleteResp.Body))
	}
}

func mustAdminResource(t *testing.T, raw string) apitypes.Resource {
	t.Helper()

	var resource apitypes.Resource
	if err := json.Unmarshal([]byte(raw), &resource); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return resource
}
