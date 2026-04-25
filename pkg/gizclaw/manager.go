package gizclaw

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/peerpublic"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/gear"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/GizClaw/gizclaw-go/pkg/giznet/gizhttp"
)

var (
	ErrDeviceOffline = errors.New("gizclaw: device offline")
	errNoRefreshData = errors.New("gizclaw: no refresh data")
)

type activePeer struct {
	conn       *giznet.Conn
	client     *peerpublic.ClientWithResponses
	lastSeenAt time.Time
	online     bool
}

type Manager struct {
	Gears *gear.Server

	activePeersMu sync.RWMutex
	activePeers   map[string]*activePeer
}

func NewManager(gearsService *gear.Server) *Manager {
	return &Manager{
		Gears:       gearsService,
		activePeers: make(map[string]*activePeer),
	}
}

func (m *Manager) MarkPeerOnline(publicKey string, conn *giznet.Conn) {
	m.activePeersMu.Lock()
	defer m.activePeersMu.Unlock()
	if m.activePeers == nil {
		m.activePeers = make(map[string]*activePeer)
	}
	m.activePeers[publicKey] = &activePeer{
		conn:       conn,
		lastSeenAt: time.Now(),
		online:     true,
	}
}

func (m *Manager) TouchPeer(publicKey string, conn *giznet.Conn) {
	m.activePeersMu.Lock()
	defer m.activePeersMu.Unlock()
	state, ok := m.activePeers[publicKey]
	if !ok || state.conn != conn {
		return
	}
	state.lastSeenAt = time.Now()
}

func (m *Manager) MarkPeerOffline(publicKey string, conn *giznet.Conn) {
	m.activePeersMu.Lock()
	defer m.activePeersMu.Unlock()
	state, ok := m.activePeers[publicKey]
	if !ok || state.conn != conn {
		return
	}
	delete(m.activePeers, publicKey)
}

func (m *Manager) ActivePeer(publicKey string) (*giznet.Conn, bool) {
	m.activePeersMu.RLock()
	defer m.activePeersMu.RUnlock()
	state, ok := m.activePeers[publicKey]
	if !ok || !state.online || state.conn == nil {
		return nil, false
	}
	return state.conn, true
}

func (m *Manager) PeerRuntime(_ context.Context, publicKey string) apitypes.Runtime {
	m.activePeersMu.RLock()
	defer m.activePeersMu.RUnlock()
	state, ok := m.activePeers[publicKey]
	if !ok {
		return apitypes.Runtime{}
	}
	return apitypes.Runtime{
		Online:     state.online,
		LastSeenAt: state.lastSeenAt,
	}
}

func (m *Manager) RefreshGear(ctx context.Context, publicKey string) (adminservice.RefreshResult, bool, error) {
	if m.Gears == nil {
		return adminservice.RefreshResult{}, false, errors.New("gizclaw: gears service not configured")
	}
	gear, err := m.Gears.LoadGear(ctx, publicKey)
	if err != nil {
		return adminservice.RefreshResult{}, false, err
	}
	conn, ok := m.ActivePeer(publicKey)
	if !ok {
		return adminservice.RefreshResult{}, false, ErrDeviceOffline
	}
	client, err := m.peerPublicClientForConn(publicKey, conn)
	if err != nil {
		return adminservice.RefreshResult{}, true, err
	}

	next, updatedFields, errs, err := m.refreshGearFromPeer(ctx, client, gear)
	if err != nil {
		online := true
		if errors.Is(err, ErrDeviceOffline) {
			m.MarkPeerOffline(publicKey, conn)
			online = false
		}
		return adminservice.RefreshResult{
			Gear:   gear,
			Errors: optionalStrings(errs),
		}, online, err
	}
	if len(updatedFields) == 0 {
		return adminservice.RefreshResult{
			Gear:          next,
			Errors:        optionalStrings(errs),
			UpdatedFields: nil,
		}, true, nil
	}
	saved, err := m.Gears.SaveGear(ctx, next)
	if err != nil {
		return adminservice.RefreshResult{
			Gear:          next,
			Errors:        optionalStrings(errs),
			UpdatedFields: optionalStrings(updatedFields),
		}, true, err
	}
	return adminservice.RefreshResult{
		Gear:          saved,
		Errors:        optionalStrings(errs),
		UpdatedFields: optionalStrings(updatedFields),
	}, true, nil
}

func (m *Manager) peerPublicClient(publicKey string) (*peerpublic.ClientWithResponses, error) {
	conn, ok := m.ActivePeer(publicKey)
	if !ok {
		return nil, ErrDeviceOffline
	}
	return m.peerPublicClientForConn(publicKey, conn)
}

func (m *Manager) peerPublicClientForConn(publicKey string, conn *giznet.Conn) (*peerpublic.ClientWithResponses, error) {
	m.activePeersMu.Lock()
	defer m.activePeersMu.Unlock()

	state, ok := m.activePeers[publicKey]
	if !ok || !state.online || state.conn != conn || state.conn == nil {
		return nil, ErrDeviceOffline
	}
	if state.client != nil {
		return state.client, nil
	}
	client := &http.Client{
		Transport: gizhttp.NewRoundTripper(conn, ServicePeerPublic),
		Timeout:   30 * time.Second,
	}
	peerClient, err := peerpublic.NewClientWithResponses(
		"http://gizclaw",
		peerpublic.WithHTTPClient(client),
	)
	if err != nil {
		return nil, err
	}
	state.client = peerClient
	return peerClient, nil
}

func (m *Manager) refreshGearFromPeer(ctx context.Context, client *peerpublic.ClientWithResponses, gear apitypes.Gear) (apitypes.Gear, []string, []string, error) {
	var (
		errs          []string
		updatedFields []string
		haveData      bool
		disconnected  bool
	)

	infoResp, err := client.GetInfoWithResponse(ctx)
	if err != nil {
		if isPeerDisconnectedError(err) {
			disconnected = true
		}
		errs = append(errs, "info: "+err.Error())
	} else if infoResp.JSON200 == nil {
		errs = append(errs, fmt.Sprintf("info: unexpected status %d", infoResp.StatusCode()))
	} else {
		haveData = true
		applyPeerRefreshInfo(&gear, *infoResp.JSON200, &updatedFields)
	}

	identifiersResp, err := client.GetIdentifiersWithResponse(ctx)
	if err != nil {
		if isPeerDisconnectedError(err) {
			disconnected = true
		}
		errs = append(errs, "identifiers: "+err.Error())
	} else if identifiersResp.JSON200 == nil {
		errs = append(errs, fmt.Sprintf("identifiers: unexpected status %d", identifiersResp.StatusCode()))
	} else {
		haveData = true
		applyPeerRefreshIdentifiers(&gear, *identifiersResp.JSON200, &updatedFields)
	}

	versionResp, err := client.GetVersionWithResponse(ctx)
	if err != nil {
		if isPeerDisconnectedError(err) {
			disconnected = true
		}
		errs = append(errs, "version: "+err.Error())
	} else if versionResp.JSON200 == nil {
		errs = append(errs, fmt.Sprintf("version: unexpected status %d", versionResp.StatusCode()))
	} else {
		haveData = true
		applyPeerRefreshVersion(&gear, *versionResp.JSON200, &updatedFields)
	}

	if !haveData {
		if disconnected {
			return gear, updatedFields, errs, ErrDeviceOffline
		}
		return gear, updatedFields, errs, errNoRefreshData
	}
	return gear, updatedFields, errs, nil
}

func isPeerDisconnectedError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "conn closed") ||
		strings.Contains(msg, "closed network connection")
}

func applyPeerRefreshInfo(gear *apitypes.Gear, info apitypes.RefreshInfo, updatedFields *[]string) {
	if gear == nil {
		return
	}
	if info.Name != nil && *info.Name != "" && !equalStringPtr(gear.Device.Name, info.Name) {
		gear.Device.Name = info.Name
		*updatedFields = append(*updatedFields, "device.name")
	}
	if info.Manufacturer != nil && *info.Manufacturer != "" {
		hardware := ensureGearHardware(&gear.Device)
		if !equalStringPtr(hardware.Manufacturer, info.Manufacturer) {
			hardware.Manufacturer = info.Manufacturer
			*updatedFields = append(*updatedFields, "device.hardware.manufacturer")
		}
	}
	if info.Model != nil && *info.Model != "" {
		hardware := ensureGearHardware(&gear.Device)
		if !equalStringPtr(hardware.Model, info.Model) {
			hardware.Model = info.Model
			*updatedFields = append(*updatedFields, "device.hardware.model")
		}
	}
	if info.HardwareRevision != nil && *info.HardwareRevision != "" {
		hardware := ensureGearHardware(&gear.Device)
		if !equalStringPtr(hardware.HardwareRevision, info.HardwareRevision) {
			hardware.HardwareRevision = info.HardwareRevision
			*updatedFields = append(*updatedFields, "device.hardware.hardware_revision")
		}
	}
}

func applyPeerRefreshIdentifiers(gear *apitypes.Gear, identifiers apitypes.RefreshIdentifiers, updatedFields *[]string) {
	if gear == nil {
		return
	}
	if identifiers.Sn != nil && *identifiers.Sn != "" && !equalStringPtr(gear.Device.Sn, identifiers.Sn) {
		gear.Device.Sn = identifiers.Sn
		*updatedFields = append(*updatedFields, "device.sn")
	}
	if identifiers.Imeis != nil && len(*identifiers.Imeis) > 0 {
		items := toGearIMEIs(*identifiers.Imeis)
		hardware := ensureGearHardware(&gear.Device)
		if !equalGearIMEISlice(hardware.Imeis, items) {
			hardware.Imeis = &items
			*updatedFields = append(*updatedFields, "device.hardware.imeis")
		}
	}
	if identifiers.Labels != nil && len(*identifiers.Labels) > 0 {
		items := toGearLabels(*identifiers.Labels)
		hardware := ensureGearHardware(&gear.Device)
		if !equalGearLabelSlice(hardware.Labels, items) {
			hardware.Labels = &items
			*updatedFields = append(*updatedFields, "device.hardware.labels")
		}
	}
}

func applyPeerRefreshVersion(gear *apitypes.Gear, version apitypes.RefreshVersion, updatedFields *[]string) {
	if gear == nil {
		return
	}
	if version.Depot != nil && *version.Depot != "" {
		hardware := ensureGearHardware(&gear.Device)
		if !equalStringPtr(hardware.Depot, version.Depot) {
			hardware.Depot = version.Depot
			*updatedFields = append(*updatedFields, "device.hardware.depot")
		}
	}
	if version.FirmwareSemver != nil && *version.FirmwareSemver != "" {
		hardware := ensureGearHardware(&gear.Device)
		if !equalStringPtr(hardware.FirmwareSemver, version.FirmwareSemver) {
			hardware.FirmwareSemver = version.FirmwareSemver
			*updatedFields = append(*updatedFields, "device.hardware.firmware_semver")
		}
	}
}

func ensureGearHardware(device *apitypes.DeviceInfo) *apitypes.HardwareInfo {
	if device.Hardware == nil {
		device.Hardware = &apitypes.HardwareInfo{}
	}
	return device.Hardware
}

func toGearIMEIs(in []apitypes.GearIMEI) []apitypes.GearIMEI {
	out := make([]apitypes.GearIMEI, 0, len(in))
	for _, item := range in {
		out = append(out, apitypes.GearIMEI{
			Name:   item.Name,
			Tac:    item.Tac,
			Serial: item.Serial,
		})
	}
	return out
}

func toGearLabels(in []apitypes.GearLabel) []apitypes.GearLabel {
	out := make([]apitypes.GearLabel, 0, len(in))
	for _, item := range in {
		out = append(out, apitypes.GearLabel{
			Key:   item.Key,
			Value: item.Value,
		})
	}
	return out
}

func optionalStrings(values []string) *[]string {
	if len(values) == 0 {
		return nil
	}
	out := append([]string(nil), values...)
	return &out
}

func equalStringPtr(left, right *string) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}

func equalGearIMEISlice(current *[]apitypes.GearIMEI, next []apitypes.GearIMEI) bool {
	if current == nil {
		return len(next) == 0
	}
	if len(*current) != len(next) {
		return false
	}
	for i := range next {
		if !equalStringPtr((*current)[i].Name, next[i].Name) ||
			(*current)[i].Tac != next[i].Tac ||
			(*current)[i].Serial != next[i].Serial {
			return false
		}
	}
	return true
}

func equalGearLabelSlice(current *[]apitypes.GearLabel, next []apitypes.GearLabel) bool {
	if current == nil {
		return len(next) == 0
	}
	if len(*current) != len(next) {
		return false
	}
	for i := range next {
		if (*current)[i].Key != next[i].Key || (*current)[i].Value != next[i].Value {
			return false
		}
	}
	return true
}
