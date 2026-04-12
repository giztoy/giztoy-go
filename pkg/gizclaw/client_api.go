package gizclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/adminservice"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/gearservice"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/serverpublic"
)

func (c *Client) GetServerInfo(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	err := c.doJSON(ctx, ServiceServerPublic, http.MethodGet, "/server-info", nil, &out)
	return out, err
}

func (c *Client) GetInfo(ctx context.Context) (gears.DeviceInfo, error) {
	var out gears.DeviceInfo
	err := c.doJSON(ctx, ServiceServerPublic, http.MethodGet, "/info", nil, &out)
	return out, err
}

func (c *Client) PutInfo(ctx context.Context, info gears.DeviceInfo) (gears.DeviceInfo, error) {
	var out gears.DeviceInfo
	err := c.doJSON(ctx, ServiceServerPublic, http.MethodPut, "/info", info, &out)
	return out, err
}

func (c *Client) GetRegistration(ctx context.Context) (gears.Registration, error) {
	api, err := c.PublicClient()
	if err != nil {
		return gears.Registration{}, err
	}
	out, err := api.GetRegistration(ctx)
	if err != nil {
		return gears.Registration{}, err
	}
	return fromPublicRegistration(out), nil
}

func (c *Client) GetRuntime(ctx context.Context) (gears.Runtime, error) {
	api, err := c.PublicClient()
	if err != nil {
		return gears.Runtime{}, err
	}
	out, err := api.GetRuntime(ctx)
	if err != nil {
		return gears.Runtime{}, err
	}
	return fromPublicRuntime(out), nil
}

func (c *Client) GetConfig(ctx context.Context) (gears.Configuration, error) {
	var out gears.Configuration
	err := c.doJSON(ctx, ServiceServerPublic, http.MethodGet, "/config", nil, &out)
	return out, err
}

func (c *Client) Register(ctx context.Context, req gears.RegistrationRequest) (gears.RegistrationResult, error) {
	api, err := c.PublicClient()
	if err != nil {
		return gears.RegistrationResult{}, err
	}
	body, err := reencode[serverpublic.RegisterGearJSONRequestBody](req)
	if err != nil {
		return gears.RegistrationResult{}, err
	}
	out, err := api.RegisterGear(ctx, body)
	if err != nil {
		return gears.RegistrationResult{}, err
	}
	return fromPublicRegistrationResult(out)
}

func (c *Client) GetOTA(ctx context.Context) (firmware.OTASummary, error) {
	var out firmware.OTASummary
	err := c.doJSON(ctx, ServiceServerPublic, http.MethodGet, "/ota", nil, &out)
	return out, err
}

func (c *Client) DownloadFirmware(ctx context.Context, path string) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://giztoy/download/firmware/"+escapePathSegment(path), nil)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.HTTPClient(ServiceServerPublic).Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, nil, fmt.Errorf("public http status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	return data, resp.Header, err
}

func (c *Client) ListGears(ctx context.Context) ([]gears.Registration, error) {
	var out gearservice.RegistrationList
	if err := c.doJSON(ctx, ServiceGear, http.MethodGet, "/gears", nil, &out); err != nil {
		return nil, err
	}
	items := make([]gears.Registration, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, fromGearRegistration(item))
	}
	return items, nil
}

func (c *Client) GetGear(ctx context.Context, publicKey string) (gears.Registration, error) {
	var out gearservice.Registration
	if err := c.doJSON(ctx, ServiceGear, http.MethodGet, "/gears/"+escapePathSegment(publicKey), nil, &out); err != nil {
		return gears.Registration{}, err
	}
	return fromGearRegistration(out), nil
}

func (c *Client) ResolveGearBySN(ctx context.Context, sn string) (string, error) {
	var out gearservice.PublicKeyResponse
	err := c.doJSON(ctx, ServiceGear, http.MethodGet, "/gears/sn/"+escapePathSegment(sn), nil, &out)
	return out.PublicKey, err
}

func (c *Client) ResolveGearByIMEI(ctx context.Context, tac, serial string) (string, error) {
	var out gearservice.PublicKeyResponse
	err := c.doJSON(ctx, ServiceGear, http.MethodGet, "/gears/imei/"+joinEscapedPath(tac, serial), nil, &out)
	return out.PublicKey, err
}

func (c *Client) ListGearsByLabel(ctx context.Context, key, value string) ([]gears.Registration, error) {
	var out gearservice.RegistrationList
	if err := c.doJSON(ctx, ServiceGear, http.MethodGet, "/gears/label/"+joinEscapedPath(key, value), nil, &out); err != nil {
		return nil, err
	}
	items := make([]gears.Registration, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, fromGearRegistration(item))
	}
	return items, nil
}

func (c *Client) ListGearsByCertification(ctx context.Context, certType gears.GearCertificationType, authority gears.GearCertificationAuthority, id string) ([]gears.Registration, error) {
	var out gearservice.RegistrationList
	if err := c.doJSON(ctx, ServiceGear, http.MethodGet, "/gears/certification/"+joinEscapedPath(string(certType), string(authority), id), nil, &out); err != nil {
		return nil, err
	}
	items := make([]gears.Registration, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, fromGearRegistration(item))
	}
	return items, nil
}

func (c *Client) ListGearsByFirmware(ctx context.Context, depot string, channel gears.GearFirmwareChannel) ([]gears.Registration, error) {
	var out gearservice.RegistrationList
	if err := c.doJSON(ctx, ServiceGear, http.MethodGet, "/gears/firmware/"+joinEscapedPath(depot, string(channel)), nil, &out); err != nil {
		return nil, err
	}
	items := make([]gears.Registration, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, fromGearRegistration(item))
	}
	return items, nil
}

func (c *Client) GetGearInfo(ctx context.Context, publicKey string) (gears.DeviceInfo, error) {
	var out gearservice.DeviceInfo
	if err := c.doJSON(ctx, ServiceGear, http.MethodGet, "/gears/"+escapePathSegment(publicKey)+"/info", nil, &out); err != nil {
		return gears.DeviceInfo{}, err
	}
	return reencode[gears.DeviceInfo](out)
}

func (c *Client) GetGearConfig(ctx context.Context, publicKey string) (gears.Configuration, error) {
	var out gearservice.Configuration
	if err := c.doJSON(ctx, ServiceGear, http.MethodGet, "/gears/"+escapePathSegment(publicKey)+"/config", nil, &out); err != nil {
		return gears.Configuration{}, err
	}
	return reencode[gears.Configuration](out)
}

func (c *Client) PutGearConfig(ctx context.Context, publicKey string, cfg gears.Configuration) (gears.Configuration, error) {
	var out gearservice.Configuration
	if err := c.doJSON(ctx, ServiceGear, http.MethodPut, "/gears/"+escapePathSegment(publicKey)+"/config", cfg, &out); err != nil {
		return gears.Configuration{}, err
	}
	return reencode[gears.Configuration](out)
}

func (c *Client) GetGearRuntime(ctx context.Context, publicKey string) (gears.Runtime, error) {
	var out gearservice.Runtime
	if err := c.doJSON(ctx, ServiceGear, http.MethodGet, "/gears/"+escapePathSegment(publicKey)+"/runtime", nil, &out); err != nil {
		return gears.Runtime{}, err
	}
	return fromGearRuntime(out), nil
}

func (c *Client) GetGearOTA(ctx context.Context, publicKey string) (firmware.OTASummary, error) {
	var out gearservice.OTASummary
	if err := c.doJSON(ctx, ServiceGear, http.MethodGet, "/gears/"+escapePathSegment(publicKey)+"/ota", nil, &out); err != nil {
		return firmware.OTASummary{}, err
	}
	return reencode[firmware.OTASummary](out)
}

func (c *Client) ApproveGear(ctx context.Context, publicKey string, role gears.GearRole) (gears.Registration, error) {
	var out gearservice.Registration
	if err := c.doJSON(ctx, ServiceGear, http.MethodPost, "/gears/"+escapePathSegment(publicKey)+":approve", map[string]any{"role": role}, &out); err != nil {
		return gears.Registration{}, err
	}
	return fromGearRegistration(out), nil
}

func (c *Client) BlockGear(ctx context.Context, publicKey string) (gears.Registration, error) {
	var out gearservice.Registration
	if err := c.doJSON(ctx, ServiceGear, http.MethodPost, "/gears/"+escapePathSegment(publicKey)+":block", nil, &out); err != nil {
		return gears.Registration{}, err
	}
	return fromGearRegistration(out), nil
}

func (c *Client) DeleteGear(ctx context.Context, publicKey string) (gears.Registration, error) {
	var out gearservice.Registration
	if err := c.doJSON(ctx, ServiceGear, http.MethodDelete, "/gears/"+escapePathSegment(publicKey), nil, &out); err != nil {
		return gears.Registration{}, err
	}
	return fromGearRegistration(out), nil
}

func (c *Client) RefreshGear(ctx context.Context, publicKey string) (gears.RefreshResult, error) {
	var out gearservice.RefreshResult
	if err := c.doJSON(ctx, ServiceGear, http.MethodPost, "/gears/"+escapePathSegment(publicKey)+":refresh", nil, &out); err != nil {
		return gears.RefreshResult{}, err
	}
	return fromGearRefreshResult(out)
}

func (c *Client) ListFirmwares(ctx context.Context) ([]firmware.Depot, error) {
	var out adminservice.DepotList
	if err := c.doJSON(ctx, ServiceAdmin, http.MethodGet, "/firmwares", nil, &out); err != nil {
		return nil, err
	}
	items := make([]firmware.Depot, 0, len(out.Items))
	for _, item := range out.Items {
		depot, err := reencode[firmware.Depot](item)
		if err != nil {
			return nil, err
		}
		items = append(items, depot)
	}
	return items, nil
}

func (c *Client) GetFirmwareDepot(ctx context.Context, depot string) (firmware.Depot, error) {
	var out adminservice.Depot
	if err := c.doJSON(ctx, ServiceAdmin, http.MethodGet, "/firmwares/"+escapePathSegment(depot), nil, &out); err != nil {
		return firmware.Depot{}, err
	}
	return reencode[firmware.Depot](out)
}

func (c *Client) PutFirmwareInfo(ctx context.Context, depot string, info firmware.DepotInfo) (firmware.Depot, error) {
	var out adminservice.Depot
	if err := c.doJSON(ctx, ServiceAdmin, http.MethodPut, "/firmwares/"+escapePathSegment(depot), info, &out); err != nil {
		return firmware.Depot{}, err
	}
	return reencode[firmware.Depot](out)
}

func (c *Client) GetFirmwareChannel(ctx context.Context, depot string, channel firmware.Channel) (firmware.DepotRelease, error) {
	var out adminservice.DepotRelease
	if err := c.doJSON(ctx, ServiceAdmin, http.MethodGet, "/firmwares/"+joinEscapedPath(depot, string(channel)), nil, &out); err != nil {
		return firmware.DepotRelease{}, err
	}
	return reencode[firmware.DepotRelease](out)
}

func (c *Client) RollbackFirmware(ctx context.Context, depot string) (firmware.Depot, error) {
	var out adminservice.Depot
	if err := c.doJSON(ctx, ServiceAdmin, http.MethodPut, "/firmwares/"+escapePathSegment(depot)+":rollback", nil, &out); err != nil {
		return firmware.Depot{}, err
	}
	return reencode[firmware.Depot](out)
}

func (c *Client) ReleaseFirmware(ctx context.Context, depot string) (firmware.Depot, error) {
	var out adminservice.Depot
	if err := c.doJSON(ctx, ServiceAdmin, http.MethodPut, "/firmwares/"+escapePathSegment(depot)+":release", nil, &out); err != nil {
		return firmware.Depot{}, err
	}
	return reencode[firmware.Depot](out)
}

func (c *Client) UploadFirmware(ctx context.Context, depot string, channel firmware.Channel, tarData []byte) (firmware.DepotRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://giztoy/firmwares/"+joinEscapedPath(depot, string(channel)), bytes.NewReader(tarData))
	if err != nil {
		return firmware.DepotRelease{}, err
	}
	req.Header.Set("Content-Type", "application/x-tar")
	resp, err := c.HTTPClient(ServiceAdmin).Do(req)
	if err != nil {
		return firmware.DepotRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		data, _ := io.ReadAll(resp.Body)
		return firmware.DepotRelease{}, fmt.Errorf("http status %d: %s", resp.StatusCode, string(data))
	}
	var out adminservice.DepotRelease
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return firmware.DepotRelease{}, err
	}
	return reencode[firmware.DepotRelease](out)
}

func reencode[T any](in any) (T, error) {
	var out T
	data, err := json.Marshal(in)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

func boolValue(v *bool) bool {
	return v != nil && *v
}

func millis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func millisPtr(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return millis(*t)
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func fromPublicRegistration(in serverpublic.Registration) gears.Registration {
	return gears.Registration{
		PublicKey:      in.PublicKey,
		Role:           gears.GearRole(in.Role),
		Status:         gears.GearStatus(in.Status),
		AutoRegistered: boolValue(in.AutoRegistered),
		CreatedAt:      millis(in.CreatedAt),
		UpdatedAt:      millis(in.UpdatedAt),
		ApprovedAt:     millisPtr(in.ApprovedAt),
	}
}

func fromGearRegistration(in gearservice.Registration) gears.Registration {
	return gears.Registration{
		PublicKey:      in.PublicKey,
		Role:           gears.GearRole(in.Role),
		Status:         gears.GearStatus(in.Status),
		AutoRegistered: boolValue(in.AutoRegistered),
		CreatedAt:      millis(in.CreatedAt),
		UpdatedAt:      millis(in.UpdatedAt),
		ApprovedAt:     millisPtr(in.ApprovedAt),
	}
}

func fromPublicGear(in serverpublic.Gear) (gears.Gear, error) {
	device, err := reencode[gears.DeviceInfo](in.Device)
	if err != nil {
		return gears.Gear{}, err
	}
	cfg, err := reencode[gears.Configuration](in.Configuration)
	if err != nil {
		return gears.Gear{}, err
	}
	return gears.Gear{
		PublicKey:      in.PublicKey,
		Role:           gears.GearRole(in.Role),
		Status:         gears.GearStatus(in.Status),
		Device:         device,
		Configuration:  cfg,
		AutoRegistered: boolValue(in.AutoRegistered),
		CreatedAt:      millis(in.CreatedAt),
		UpdatedAt:      millis(in.UpdatedAt),
		ApprovedAt:     millisPtr(in.ApprovedAt),
	}, nil
}

func fromGearGear(in gearservice.Gear) (gears.Gear, error) {
	device, err := reencode[gears.DeviceInfo](in.Device)
	if err != nil {
		return gears.Gear{}, err
	}
	cfg, err := reencode[gears.Configuration](in.Configuration)
	if err != nil {
		return gears.Gear{}, err
	}
	return gears.Gear{
		PublicKey:      in.PublicKey,
		Role:           gears.GearRole(in.Role),
		Status:         gears.GearStatus(in.Status),
		Device:         device,
		Configuration:  cfg,
		AutoRegistered: boolValue(in.AutoRegistered),
		CreatedAt:      millis(in.CreatedAt),
		UpdatedAt:      millis(in.UpdatedAt),
		ApprovedAt:     millisPtr(in.ApprovedAt),
	}, nil
}

func fromPublicRegistrationResult(in serverpublic.RegistrationResult) (gears.RegistrationResult, error) {
	gear, err := fromPublicGear(in.Gear)
	if err != nil {
		return gears.RegistrationResult{}, err
	}
	return gears.RegistrationResult{
		Gear:       gear,
		Registered: fromPublicRegistration(in.Registration),
	}, nil
}

func fromGearRuntime(in gearservice.Runtime) gears.Runtime {
	return gears.Runtime{
		Online:     in.Online,
		LastSeenAt: millis(in.LastSeenAt),
		LastAddr:   stringValue(in.LastAddr),
	}
}

func fromPublicRuntime(in serverpublic.Runtime) gears.Runtime {
	return gears.Runtime{
		Online:     in.Online,
		LastSeenAt: millis(in.LastSeenAt),
		LastAddr:   stringValue(in.LastAddr),
	}
}

func fromGearRefreshResult(in gearservice.RefreshResult) (gears.RefreshResult, error) {
	gear, err := fromGearGear(in.Gear)
	if err != nil {
		return gears.RefreshResult{}, err
	}
	var updated []string
	if in.UpdatedFields != nil {
		updated = append(updated, (*in.UpdatedFields)...)
	}
	return gears.RefreshResult{
		Gear:          gear,
		UpdatedFields: updated,
	}, nil
}

func (c *Client) doJSON(ctx context.Context, service uint64, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://giztoy"+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTPClient(service).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http status %d: %s", resp.StatusCode, string(data))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func escapePathSegment(value string) string {
	return url.PathEscape(value)
}

func joinEscapedPath(parts ...string) string {
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, escapePathSegment(part))
	}
	return strings.Join(escaped, "/")
}
