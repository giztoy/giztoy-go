package gizclaw_test

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
)

func TestIntegrationAdminServiceFirmwareLifecycle(t *testing.T) {
	ts := startTestServer(t)

	admin := newTestClient(t, ts)
	if _, err := register(context.Background(), admin, serverpublic.RegistrationRequest{
		Device:            apitypes.DeviceInfo{Name: strPtr("admin")},
		RegistrationToken: strPtr("admin_default"),
	}); err != nil {
		t.Fatalf("admin register error: %v", err)
	}

	if _, err := putFirmwareInfo(context.Background(), admin, "demo-main", apitypes.DepotInfo{
		Files: &[]apitypes.DepotInfoFile{{Path: "firmware.bin"}},
	}); err != nil {
		t.Fatalf("PutFirmwareInfo error: %v", err)
	}

	payload := []byte("firmware-v1")
	sum256 := sha256.Sum256(payload)
	sumMD5 := md5.Sum(payload)
	tarData := buildReleaseTar(t, apitypes.DepotRelease{
		FirmwareSemver: "1.0.0",
		Channel:        strPtr("stable"),
		Files: &[]apitypes.DepotFile{{
			Path:   "firmware.bin",
			Sha256: hex.EncodeToString(sum256[:]),
			Md5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload})

	if _, err := uploadFirmware(context.Background(), admin, "demo-main", adminservice.Channel("stable"), tarData); err != nil {
		t.Fatalf("UploadFirmware error: %v", err)
	}
	if _, err := uploadFirmware(context.Background(), admin, "demo-main", adminservice.Channel("beta"), buildReleaseTar(t, apitypes.DepotRelease{
		FirmwareSemver: "1.1.0",
		Channel:        strPtr("beta"),
		Files: &[]apitypes.DepotFile{{
			Path:   "firmware.bin",
			Sha256: hex.EncodeToString(sum256[:]),
			Md5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload})); err != nil {
		t.Fatalf("UploadFirmware beta error: %v", err)
	}
	if _, err := uploadFirmware(context.Background(), admin, "demo-main", adminservice.Channel("testing"), buildReleaseTar(t, apitypes.DepotRelease{
		FirmwareSemver: "1.2.0",
		Channel:        strPtr("testing"),
		Files: &[]apitypes.DepotFile{{
			Path:   "firmware.bin",
			Sha256: hex.EncodeToString(sum256[:]),
			Md5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload})); err != nil {
		t.Fatalf("UploadFirmware testing error: %v", err)
	}

	depot, err := getFirmwareDepot(context.Background(), admin, "demo-main")
	if err != nil {
		t.Fatalf("GetFirmwareDepot error: %v", err)
	}
	if depot.Stable.FirmwareSemver != "1.0.0" {
		t.Fatalf("stable semver = %q", depot.Stable.FirmwareSemver)
	}
	items, err := listFirmwares(context.Background(), admin)
	if err != nil {
		t.Fatalf("ListFirmwares error: %v", err)
	}
	if len(items) != 1 || items[0].Name != "demo-main" {
		t.Fatalf("ListFirmwares = %+v", items)
	}
	if release, err := getFirmwareChannel(context.Background(), admin, "demo-main", adminservice.Channel("stable")); err != nil || release.FirmwareSemver != "1.0.0" {
		t.Fatalf("GetFirmwareChannel = %+v, %v", release, err)
	}
	if _, err := releaseFirmware(context.Background(), admin, "demo-main"); err != nil {
		t.Fatalf("ReleaseFirmware error: %v", err)
	}
	if _, err := rollbackFirmware(context.Background(), admin, "demo-main"); err != nil {
		t.Fatalf("RollbackFirmware error: %v", err)
	}
}

func TestIntegrationAdminServiceWorkspaceTemplateLifecycle(t *testing.T) {
	ts := startTestServer(t)

	admin := newTestClient(t, ts)
	if _, err := register(context.Background(), admin, serverpublic.RegistrationRequest{
		Device:            apitypes.DeviceInfo{Name: strPtr("admin")},
		RegistrationToken: strPtr("admin_default"),
	}); err != nil {
		t.Fatalf("admin register error: %v", err)
	}

	createDoc := mustWorkflowTemplateDocument(t, `{
		"apiVersion": "gizclaw.flowcraft/v1alpha1",
		"kind": "SingleAgentGraphWorkflowTemplate",
		"metadata": {
			"name": "demo-assistant",
			"description": "single-agent graph workflow"
		},
		"spec": {}
	}`)
	created, err := createWorkspaceTemplate(context.Background(), admin, createDoc)
	if err != nil {
		t.Fatalf("CreateWorkspaceTemplate error: %v", err)
	}
	if kind, err := created.Discriminator(); err != nil || kind != "SingleAgentGraphWorkflowTemplate" {
		t.Fatalf("CreateWorkspaceTemplate discriminator = %q, %v", kind, err)
	}

	items, err := listWorkspaceTemplates(context.Background(), admin)
	if err != nil {
		t.Fatalf("ListWorkspaceTemplates error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListWorkspaceTemplates len = %d", len(items))
	}

	got, err := getWorkspaceTemplate(context.Background(), admin, "demo-assistant")
	if err != nil {
		t.Fatalf("GetWorkspaceTemplate error: %v", err)
	}
	gotSingle, err := got.AsSingleAgentGraphWorkflowTemplate()
	if err != nil {
		t.Fatalf("AsSingleAgentGraphWorkflowTemplate error: %v", err)
	}
	if gotSingle.Metadata.Name != "demo-assistant" {
		t.Fatalf("GetWorkspaceTemplate name = %q", gotSingle.Metadata.Name)
	}

	updateDoc := mustWorkflowTemplateDocument(t, `{
		"apiVersion": "gizclaw.flowcraft/v1alpha1",
		"kind": "SingleAgentGraphWorkflowTemplate",
		"metadata": {
			"name": "demo-assistant",
			"description": "updated description"
		},
		"spec": {
			"runtime": {
				"executor_ref": "local"
			}
		}
	}`)
	updated, err := putWorkspaceTemplate(context.Background(), admin, "demo-assistant", updateDoc)
	if err != nil {
		t.Fatalf("PutWorkspaceTemplate error: %v", err)
	}
	updatedSingle, err := updated.AsSingleAgentGraphWorkflowTemplate()
	if err != nil {
		t.Fatalf("updated.AsSingleAgentGraphWorkflowTemplate error: %v", err)
	}
	if updatedSingle.Metadata.Description == nil || *updatedSingle.Metadata.Description != "updated description" {
		t.Fatalf("PutWorkspaceTemplate description = %#v", updatedSingle.Metadata.Description)
	}

	if _, err := deleteWorkspaceTemplate(context.Background(), admin, "demo-assistant"); err != nil {
		t.Fatalf("DeleteWorkspaceTemplate error: %v", err)
	}
	if _, err := getWorkspaceTemplate(context.Background(), admin, "demo-assistant"); err == nil {
		t.Fatal("GetWorkspaceTemplate after delete expected error")
	}
}

func TestIntegrationAdminServiceWorkspaceLifecycle(t *testing.T) {
	ts := startTestServer(t)

	admin := newTestClient(t, ts)
	if _, err := register(context.Background(), admin, serverpublic.RegistrationRequest{
		Device:            apitypes.DeviceInfo{Name: strPtr("admin")},
		RegistrationToken: strPtr("admin_default"),
	}); err != nil {
		t.Fatalf("admin register error: %v", err)
	}

	templateDoc := mustWorkflowTemplateDocument(t, `{
		"apiVersion": "gizclaw.flowcraft/v1alpha1",
		"kind": "SingleAgentGraphWorkflowTemplate",
		"metadata": {
			"name": "demo-template"
		},
		"spec": {}
	}`)
	if _, err := createWorkspaceTemplate(context.Background(), admin, templateDoc); err != nil {
		t.Fatalf("CreateWorkspaceTemplate error: %v", err)
	}

	createBody := adminservice.WorkspaceUpsert{
		Name:                  "demo-workspace",
		WorkspaceTemplateName: "demo-template",
	}
	created, err := createWorkspace(context.Background(), admin, createBody)
	if err != nil {
		t.Fatalf("CreateWorkspace error: %v", err)
	}
	if created.Name != "demo-workspace" {
		t.Fatalf("CreateWorkspace = %#v", created)
	}

	items, err := listWorkspaces(context.Background(), admin)
	if err != nil {
		t.Fatalf("ListWorkspaces error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListWorkspaces len = %d", len(items))
	}

	got, err := getWorkspace(context.Background(), admin, "demo-workspace")
	if err != nil {
		t.Fatalf("GetWorkspace error: %v", err)
	}
	if got.WorkspaceTemplateName != "demo-template" {
		t.Fatalf("GetWorkspace template = %q", got.WorkspaceTemplateName)
	}

	updated, err := putWorkspace(context.Background(), admin, "demo-workspace", adminservice.WorkspaceUpsert{
		Name:                  "demo-workspace",
		WorkspaceTemplateName: "demo-template",
		Parameters:            &map[string]interface{}{"mode": "updated"},
	})
	if err != nil {
		t.Fatalf("PutWorkspace error: %v", err)
	}
	if updated.Parameters == nil || (*updated.Parameters)["mode"] != "updated" {
		t.Fatalf("PutWorkspace parameters = %#v", updated.Parameters)
	}

	if _, err := deleteWorkspace(context.Background(), admin, "demo-workspace"); err != nil {
		t.Fatalf("DeleteWorkspace error: %v", err)
	}
	if _, err := getWorkspace(context.Background(), admin, "demo-workspace"); err == nil {
		t.Fatal("GetWorkspace after delete expected error")
	}
}

func TestIntegrationAdminServiceCredentialLifecycle(t *testing.T) {
	ts := startTestServer(t)

	admin := newTestClient(t, ts)
	if _, err := register(context.Background(), admin, serverpublic.RegistrationRequest{
		Device:            apitypes.DeviceInfo{Name: strPtr("admin")},
		RegistrationToken: strPtr("admin_default"),
	}); err != nil {
		t.Fatalf("admin register error: %v", err)
	}

	createBody := mustCredentialUpsert(t, `{
		"name": "openai-primary",
		"provider": "openai",
		"method": "api_key",
		"description": "primary openai credential",
		"body": {"api_key": "sk-test"}
	}`)
	created, err := createCredential(context.Background(), admin, createBody)
	if err != nil {
		t.Fatalf("CreateCredential error: %v", err)
	}
	if created.Name != "openai-primary" || created.Method != apitypes.ApiKey {
		t.Fatalf("CreateCredential = %#v", created)
	}
	if created.Body["api_key"] != "sk-test" {
		t.Fatalf("CreateCredential body = %#v", created.Body)
	}

	items, err := listCredentials(context.Background(), admin, nil)
	if err != nil {
		t.Fatalf("ListCredentials error: %v", err)
	}
	if len(items) != 1 || items[0].Provider != "openai" {
		t.Fatalf("ListCredentials = %#v", items)
	}

	got, err := getCredential(context.Background(), admin, "openai-primary")
	if err != nil {
		t.Fatalf("GetCredential error: %v", err)
	}
	if got.Description == nil || *got.Description != "primary openai credential" {
		t.Fatalf("GetCredential description = %#v", got.Description)
	}
	if got.Body["api_key"] != "sk-test" {
		t.Fatalf("GetCredential body = %#v", got.Body)
	}

	updateBody := mustCredentialUpsert(t, `{
		"name": "openai-primary",
		"provider": "minimax",
		"method": "app_id_token",
		"description": "migrated credential",
		"body": {"app_id": "app-123", "token": "tok-123"}
	}`)
	updated, err := putCredential(context.Background(), admin, "openai-primary", updateBody)
	if err != nil {
		t.Fatalf("PutCredential error: %v", err)
	}
	if updated.Provider != "minimax" || updated.Method != apitypes.AppIdToken {
		t.Fatalf("PutCredential = %#v", updated)
	}
	if updated.Body["app_id"] != "app-123" || updated.Body["token"] != "tok-123" {
		t.Fatalf("PutCredential body = %#v", updated.Body)
	}

	provider := apitypes.CredentialProvider("minimax")
	filtered, err := listCredentials(context.Background(), admin, &provider)
	if err != nil {
		t.Fatalf("ListCredentials(provider) error: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Name != "openai-primary" {
		t.Fatalf("ListCredentials(provider) = %#v", filtered)
	}
	if filtered[0].Body["token"] != "tok-123" {
		t.Fatalf("ListCredentials(provider) body = %#v", filtered[0].Body)
	}

	if _, err := deleteCredential(context.Background(), admin, "openai-primary"); err != nil {
		t.Fatalf("DeleteCredential error: %v", err)
	}
	if _, err := getCredential(context.Background(), admin, "openai-primary"); err == nil {
		t.Fatal("GetCredential after delete expected error")
	}
}

func mustWorkflowTemplateDocument(t *testing.T, raw string) apitypes.WorkflowTemplateDocument {
	t.Helper()

	var doc apitypes.WorkflowTemplateDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return doc
}

func mustCredentialUpsert(t *testing.T, raw string) adminservice.CredentialUpsert {
	t.Helper()

	var upsert adminservice.CredentialUpsert
	if err := json.Unmarshal([]byte(raw), &upsert); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return upsert
}
