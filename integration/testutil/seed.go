package testutil

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
)

const (
	SeedDepotName             = "ui-seed-depot"
	SeedCredentialName        = "ui-seed-credential"
	SeedMiniMaxTenantName     = "ui-seed-tenant"
	SeedVoiceID               = "ui-seed-voice"
	SeedWorkspaceTemplateName = "ui-seed-template"
	SeedWorkspaceName         = "ui-seed-workspace"
)

//go:embed gizclaw_seed_data/**/*.json
var seedFS embed.FS

type RegistrationSeed struct {
	RegistrationToken string              `json:"registration_token"`
	Device            apitypes.DeviceInfo `json:"device"`
}

func LoadRegistrationSeed(name string) (RegistrationSeed, error) {
	var seed RegistrationSeed
	if err := readSeedJSON("gizclaw_seed_data/registrations/"+name+".json", &seed); err != nil {
		return RegistrationSeed{}, err
	}
	return seed, nil
}

func RegistrationRequest(publicKey string, seed RegistrationSeed) serverpublic.RegistrationRequest {
	request := serverpublic.RegistrationRequest{
		Device:    seed.Device,
		PublicKey: publicKey,
	}
	if strings.TrimSpace(seed.RegistrationToken) != "" {
		request.RegistrationToken = &seed.RegistrationToken
	}
	return request
}

func LoadDeviceConfigSeed() (apitypes.Configuration, error) {
	var config apitypes.Configuration
	if err := readSeedJSON("gizclaw_seed_data/gear_config/device.json", &config); err != nil {
		return apitypes.Configuration{}, err
	}
	return config, nil
}

func LoadDepotInfoSeed() (apitypes.DepotInfo, error) {
	var info apitypes.DepotInfo
	if err := readSeedJSON("gizclaw_seed_data/firmware/depot_info.json", &info); err != nil {
		return apitypes.DepotInfo{}, err
	}
	return info, nil
}

func LoadAdminCatalogSeed() (apitypes.Resource, error) {
	var resource apitypes.Resource
	if err := readSeedJSON("gizclaw_seed_data/resources/admin_catalog.json", &resource); err != nil {
		return apitypes.Resource{}, err
	}
	return resource, nil
}

func ApplyAdminCatalogSeed(ctx context.Context, api *adminservice.ClientWithResponses) error {
	resource, err := LoadAdminCatalogSeed()
	if err != nil {
		return err
	}
	resp, err := api.ApplyResourceWithResponse(ctx, resource)
	if err != nil {
		return err
	}
	if resp.JSON200 == nil {
		return seedResponseError("apply admin catalog", resp.StatusCode(), resp.Body, resp.JSON400, resp.JSON409, resp.JSON500, resp.JSON501)
	}
	return nil
}

func ApplyWorkspaceSeed(ctx context.Context, api *adminservice.ClientWithResponses) error {
	var templateResource apitypes.Resource
	if err := readSeedJSON("gizclaw_seed_data/resources/workspace_template.json", &templateResource); err != nil {
		return err
	}
	template, err := templateResource.AsWorkspaceTemplateResource()
	if err != nil {
		return err
	}
	templateResp, err := api.PutWorkspaceTemplateWithResponse(ctx, template.Metadata.Name, template.Spec)
	if err != nil {
		return err
	}
	if templateResp.JSON200 == nil {
		return seedResponseError("put workspace template", templateResp.StatusCode(), templateResp.Body, templateResp.JSON400, templateResp.JSON500)
	}

	var workspaceResource apitypes.Resource
	if err := readSeedJSON("gizclaw_seed_data/resources/workspace.json", &workspaceResource); err != nil {
		return err
	}
	workspace, err := workspaceResource.AsWorkspaceResource()
	if err != nil {
		return err
	}
	workspaceResp, err := api.PutWorkspaceWithResponse(ctx, workspace.Metadata.Name, adminservice.WorkspaceUpsert{
		Name:                  workspace.Metadata.Name,
		Parameters:            workspace.Spec.Parameters,
		WorkspaceTemplateName: workspace.Spec.WorkspaceTemplateName,
	})
	if err != nil {
		return err
	}
	if workspaceResp.JSON200 == nil {
		return seedResponseError("put workspace", workspaceResp.StatusCode(), workspaceResp.Body, workspaceResp.JSON400, workspaceResp.JSON500)
	}
	return nil
}

func ApplyFirmwareSeed(ctx context.Context, api *adminservice.ClientWithResponses) error {
	info, err := LoadDepotInfoSeed()
	if err != nil {
		return err
	}
	infoResp, err := api.PutDepotInfoWithResponse(ctx, SeedDepotName, info)
	if err != nil {
		return err
	}
	if infoResp.JSON200 == nil {
		return seedResponseError("put depot info", infoResp.StatusCode(), infoResp.Body, infoResp.JSON400, infoResp.JSON500)
	}

	releaseTar, err := FirmwareReleaseTarSeed("stable", "1.0.0")
	if err != nil {
		return err
	}
	channelResp, err := api.PutChannelWithBodyWithResponse(ctx, SeedDepotName, "stable", "application/octet-stream", bytes.NewReader(releaseTar))
	if err != nil {
		return err
	}
	if channelResp.JSON200 == nil {
		return seedResponseError("put channel", channelResp.StatusCode(), channelResp.Body, channelResp.JSON409)
	}
	return nil
}

func ApplyDeviceConfigSeed(ctx context.Context, api *adminservice.ClientWithResponses, publicKey string) error {
	config, err := LoadDeviceConfigSeed()
	if err != nil {
		return err
	}
	resp, err := api.PutGearConfigWithResponse(ctx, publicKey, config)
	if err != nil {
		return err
	}
	if resp.JSON200 == nil {
		return seedResponseError("put gear config", resp.StatusCode(), resp.Body, resp.JSON400, resp.JSON404)
	}
	return nil
}

func FirmwareReleaseTarSeed(channel, firmwareSemver string) ([]byte, error) {
	payload := []byte("seeded firmware payload")
	sum256 := sha256.Sum256(payload)
	sumMD5 := md5.Sum(payload)
	release := apitypes.DepotRelease{
		Channel:        &channel,
		FirmwareSemver: firmwareSemver,
		Files: &[]apitypes.DepotFile{{
			Md5:    hex.EncodeToString(sumMD5[:]),
			Path:   "firmware.bin",
			Sha256: hex.EncodeToString(sum256[:]),
		}},
	}

	manifest, err := json.Marshal(release)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := writeSeedTarEntry(tw, "manifest.json", manifest); err != nil {
		return nil, err
	}
	if err := writeSeedTarEntry(tw, "firmware.bin", payload); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func CopyAdminCatalogSeedJSON(w io.Writer) error {
	data, err := seedFS.ReadFile("gizclaw_seed_data/resources/admin_catalog.json")
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func readSeedJSON(path string, target interface{}) error {
	data, err := seedFS.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func writeSeedTarEntry(tw *tar.Writer, name string, data []byte) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func seedResponseError(action string, status int, body []byte, payloads ...interface{}) error {
	for _, payload := range payloads {
		if errorPayload, ok := payload.(*apitypes.ErrorResponse); ok && errorPayload != nil {
			return fmt.Errorf("%s failed: %s: %s", action, errorPayload.Error.Code, errorPayload.Error.Message)
		}
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		text = http.StatusText(status)
	}
	if text == "" {
		text = "empty response"
	}
	return fmt.Errorf("%s failed with status %d: %s", action, status, text)
}
