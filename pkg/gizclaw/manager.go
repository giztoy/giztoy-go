package gizclaw

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/gizclaw/api/peerpublic"
	"github.com/giztoy/giztoy-go/pkg/giznet"
	"github.com/giztoy/giztoy-go/pkg/giznet/gizhttp"
)

var ErrDeviceOffline = errors.New("gizclaw: device offline")

type activePeer struct {
	conn       *giznet.Conn
	client     *peerpublic.Client
	lastSeenAt int64
	online     bool
}

type Manager struct {
	Gears *gears.Service

	activePeersMu sync.RWMutex
	activePeers   map[string]*activePeer
}

func NewManager(gearsService *gears.Service) *Manager {
	return &Manager{
		Gears:       gearsService,
		activePeers: make(map[string]*activePeer),
	}
}

func (m *Manager) GetInfo(ctx context.Context, publicKey string) (gears.RefreshInfo, error) {
	client, err := m.peerPublicClient(publicKey)
	if err != nil {
		return gears.RefreshInfo{}, err
	}
	info, err := client.GetInfo(ctx)
	if err != nil {
		return gears.RefreshInfo{}, err
	}
	return gears.RefreshInfo{
		Name:             deref(info.Name),
		Manufacturer:     deref(info.Manufacturer),
		Model:            deref(info.Model),
		HardwareRevision: deref(info.HardwareRevision),
	}, nil
}

func (m *Manager) GetIdentifiers(ctx context.Context, publicKey string) (gears.RefreshIdentifiers, error) {
	client, err := m.peerPublicClient(publicKey)
	if err != nil {
		return gears.RefreshIdentifiers{}, err
	}
	identifiers, err := client.GetIdentifiers(ctx)
	if err != nil {
		return gears.RefreshIdentifiers{}, err
	}
	return gears.RefreshIdentifiers{
		SN:     deref(identifiers.Sn),
		IMEIs:  toGearIMEIs(identifiers.Imeis),
		Labels: toGearLabels(identifiers.Labels),
	}, nil
}

func (m *Manager) GetVersion(ctx context.Context, publicKey string) (gears.RefreshVersion, error) {
	client, err := m.peerPublicClient(publicKey)
	if err != nil {
		return gears.RefreshVersion{}, err
	}
	version, err := client.GetVersion(ctx)
	if err != nil {
		return gears.RefreshVersion{}, err
	}
	return gears.RefreshVersion{
		Depot:          deref(version.Depot),
		FirmwareSemVer: deref(version.FirmwareSemver),
	}, nil
}

func (m *Manager) MarkPeerOnline(publicKey string, conn *giznet.Conn) {
	m.activePeersMu.Lock()
	defer m.activePeersMu.Unlock()
	if m.activePeers == nil {
		m.activePeers = make(map[string]*activePeer)
	}
	m.activePeers[publicKey] = &activePeer{
		conn:       conn,
		lastSeenAt: time.Now().UnixMilli(),
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
	state.lastSeenAt = time.Now().UnixMilli()
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

func (m *Manager) PeerRuntime(_ context.Context, publicKey string) gears.Runtime {
	m.activePeersMu.RLock()
	defer m.activePeersMu.RUnlock()
	state, ok := m.activePeers[publicKey]
	if !ok {
		return gears.Runtime{}
	}
	return gears.Runtime{
		Online:     state.online,
		LastSeenAt: state.lastSeenAt,
	}
}

func (m *Manager) RefreshDevice(ctx context.Context, publicKey string) (gears.RefreshResult, bool, error) {
	if m.Gears == nil {
		return gears.RefreshResult{}, false, errors.New("gizclaw: gears service not configured")
	}
	if _, err := m.Gears.Get(ctx, publicKey); err != nil {
		return gears.RefreshResult{}, false, err
	}
	conn, ok := m.ActivePeer(publicKey)
	if !ok {
		return gears.RefreshResult{}, false, ErrDeviceOffline
	}
	if _, err := m.peerPublicClientForConn(publicKey, conn); err != nil {
		return gears.RefreshResult{}, true, err
	}
	result, err := m.Gears.RefreshFromProvider(ctx, publicKey, m)
	if err != nil {
		return result, true, err
	}
	return result, true, nil
}

func (m *Manager) peerPublicClient(publicKey string) (*peerpublic.Client, error) {
	conn, ok := m.ActivePeer(publicKey)
	if !ok {
		return nil, ErrDeviceOffline
	}
	return m.peerPublicClientForConn(publicKey, conn)
}

func (m *Manager) peerPublicClientForConn(publicKey string, conn *giznet.Conn) (*peerpublic.Client, error) {
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
	peerClient, err := peerpublic.NewClient(client)
	if err != nil {
		return nil, err
	}
	state.client = peerClient
	return peerClient, nil
}

func deref[T any](v *T) T {
	var zero T
	if v == nil {
		return zero
	}
	return *v
}

func toGearIMEIs(in *[]peerpublic.GearIMEI) []gears.GearIMEI {
	if in == nil {
		return nil
	}
	out := make([]gears.GearIMEI, 0, len(*in))
	for _, item := range *in {
		out = append(out, gears.GearIMEI{
			Name:   deref(item.Name),
			TAC:    item.Tac,
			Serial: item.Serial,
		})
	}
	return out
}

func toGearLabels(in *[]peerpublic.GearLabel) []gears.GearLabel {
	if in == nil {
		return nil
	}
	out := make([]gears.GearLabel, 0, len(*in))
	for _, item := range *in {
		out = append(out, gears.GearLabel{
			Key:   item.Key,
			Value: item.Value,
		})
	}
	return out
}
