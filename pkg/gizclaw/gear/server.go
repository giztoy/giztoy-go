package gear

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

var (
	ErrGearNotFound      = errors.New("gear: gear not found")
	ErrGearAlreadyExists = errors.New("gear: gear already exists")
)

const (
	defaultListLimit = 50
	maxListLimit     = 200
)

type PeerManager interface {
	PeerRuntime(context.Context, string) apitypes.Runtime
	RefreshGear(context.Context, string) (adminservice.RefreshResult, bool, error)
}

type Server struct {
	Store              kv.Store
	RegistrationTokens map[string]apitypes.GearRole
	BuildCommit        string
	ServerPublicKey    string
	PeerManager        PeerManager

	mu sync.Mutex
}

type GearsAdminService interface {
	ListGears(context.Context, adminservice.ListGearsRequestObject) (adminservice.ListGearsResponseObject, error)
	ListByCertification(context.Context, adminservice.ListByCertificationRequestObject) (adminservice.ListByCertificationResponseObject, error)
	ListByFirmware(context.Context, adminservice.ListByFirmwareRequestObject) (adminservice.ListByFirmwareResponseObject, error)
	ResolveByIMEI(context.Context, adminservice.ResolveByIMEIRequestObject) (adminservice.ResolveByIMEIResponseObject, error)
	ListByLabel(context.Context, adminservice.ListByLabelRequestObject) (adminservice.ListByLabelResponseObject, error)
	ResolveBySN(context.Context, adminservice.ResolveBySNRequestObject) (adminservice.ResolveBySNResponseObject, error)
	DeleteGear(context.Context, adminservice.DeleteGearRequestObject) (adminservice.DeleteGearResponseObject, error)
	GetGear(context.Context, adminservice.GetGearRequestObject) (adminservice.GetGearResponseObject, error)
	GetGearConfig(context.Context, adminservice.GetGearConfigRequestObject) (adminservice.GetGearConfigResponseObject, error)
	PutGearConfig(context.Context, adminservice.PutGearConfigRequestObject) (adminservice.PutGearConfigResponseObject, error)
	GetGearInfo(context.Context, adminservice.GetGearInfoRequestObject) (adminservice.GetGearInfoResponseObject, error)
	GetGearRuntime(context.Context, adminservice.GetGearRuntimeRequestObject) (adminservice.GetGearRuntimeResponseObject, error)
	ApproveGear(context.Context, adminservice.ApproveGearRequestObject) (adminservice.ApproveGearResponseObject, error)
	BlockGear(context.Context, adminservice.BlockGearRequestObject) (adminservice.BlockGearResponseObject, error)
	RefreshGear(context.Context, adminservice.RefreshGearRequestObject) (adminservice.RefreshGearResponseObject, error)
}

type GearsGearService interface {
	GetConfig(context.Context, gearservice.GetConfigRequestObject) (gearservice.GetConfigResponseObject, error)
	GetInfo(context.Context, gearservice.GetInfoRequestObject) (gearservice.GetInfoResponseObject, error)
	PutInfo(context.Context, gearservice.PutInfoRequestObject) (gearservice.PutInfoResponseObject, error)
	GetRegistration(context.Context, gearservice.GetRegistrationRequestObject) (gearservice.GetRegistrationResponseObject, error)
	GetRuntime(context.Context, gearservice.GetRuntimeRequestObject) (gearservice.GetRuntimeResponseObject, error)
}

type GearsServerPublic interface {
	RegisterGear(context.Context, serverpublic.RegisterGearRequestObject) (serverpublic.RegisterGearResponseObject, error)
	GetServerInfo(context.Context, serverpublic.GetServerInfoRequestObject) (serverpublic.GetServerInfoResponseObject, error)
}

var _ GearsAdminService = (*Server)(nil)
var _ GearsGearService = (*Server)(nil)
var _ GearsServerPublic = (*Server)(nil)

// ListGears implements `adminservice.StrictServerInterface.ListGears`.
func (s *Server) ListGears(ctx context.Context, request adminservice.ListGearsRequestObject) (adminservice.ListGearsResponseObject, error) {
	cursor, limit := normalizeListParams(request.Params.Cursor, request.Params.Limit)
	items, hasNext, nextCursor, err := s.listPage(ctx, cursor, limit)
	if err != nil {
		return adminservice.ListGears500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	return adminservice.ListGears200JSONResponse(toAdminRegistrationList(items, hasNext, nextCursor)), nil
}

// ListByCertification implements `adminservice.StrictServerInterface.ListByCertification`.
func (s *Server) ListByCertification(ctx context.Context, request adminservice.ListByCertificationRequestObject) (adminservice.ListByCertificationResponseObject, error) {
	id, err := pathUnescape(request.Id)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	cursor, limit := normalizeListParams(request.Params.Cursor, request.Params.Limit)
	items, hasNext, nextCursor, err := s.listByCertification(ctx, apitypes.GearCertificationType(request.Type), apitypes.GearCertificationAuthority(request.Authority), id, cursor, limit)
	if err != nil {
		return adminservice.ListByCertification500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	return adminservice.ListByCertification200JSONResponse(toAdminRegistrationList(items, hasNext, nextCursor)), nil
}

// ListByFirmware implements `adminservice.StrictServerInterface.ListByFirmware`.
func (s *Server) ListByFirmware(ctx context.Context, request adminservice.ListByFirmwareRequestObject) (adminservice.ListByFirmwareResponseObject, error) {
	depot, err := pathUnescape(request.Depot)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	cursor, limit := normalizeListParams(request.Params.Cursor, request.Params.Limit)
	items, hasNext, nextCursor, err := s.listByFirmware(ctx, depot, apitypes.GearFirmwareChannel(request.Channel), cursor, limit)
	if err != nil {
		return adminservice.ListByFirmware500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	return adminservice.ListByFirmware200JSONResponse(toAdminRegistrationList(items, hasNext, nextCursor)), nil
}

// ResolveByIMEI implements `adminservice.StrictServerInterface.ResolveByIMEI`.
func (s *Server) ResolveByIMEI(ctx context.Context, request adminservice.ResolveByIMEIRequestObject) (adminservice.ResolveByIMEIResponseObject, error) {
	tac, err := pathUnescape(request.Tac)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	serial, err := pathUnescape(request.Serial)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	publicKey, err := s.resolveByIMEI(ctx, tac, serial)
	if err != nil {
		return adminservice.ResolveByIMEI404JSONResponse(adminError("GEAR_IMEI_NOT_FOUND", err.Error())), nil
	}
	return adminservice.ResolveByIMEI200JSONResponse(adminservice.PublicKeyResponse{PublicKey: publicKey}), nil
}

// ListByLabel implements `adminservice.StrictServerInterface.ListByLabel`.
func (s *Server) ListByLabel(ctx context.Context, request adminservice.ListByLabelRequestObject) (adminservice.ListByLabelResponseObject, error) {
	key, err := pathUnescape(request.Key)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	value, err := pathUnescape(request.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	cursor, limit := normalizeListParams(request.Params.Cursor, request.Params.Limit)
	items, hasNext, nextCursor, err := s.listByLabel(ctx, key, value, cursor, limit)
	if err != nil {
		return adminservice.ListByLabel500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
	}
	return adminservice.ListByLabel200JSONResponse(toAdminRegistrationList(items, hasNext, nextCursor)), nil
}

// ResolveBySN implements `adminservice.StrictServerInterface.ResolveBySN`.
func (s *Server) ResolveBySN(ctx context.Context, request adminservice.ResolveBySNRequestObject) (adminservice.ResolveBySNResponseObject, error) {
	sn, err := pathUnescape(request.Sn)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	publicKey, err := s.resolveBySN(ctx, sn)
	if err != nil {
		return adminservice.ResolveBySN404JSONResponse(adminError("GEAR_SN_NOT_FOUND", err.Error())), nil
	}
	return adminservice.ResolveBySN200JSONResponse(adminservice.PublicKeyResponse{PublicKey: publicKey}), nil
}

// DeleteGear implements `adminservice.StrictServerInterface.DeleteGear`.
func (s *Server) DeleteGear(ctx context.Context, request adminservice.DeleteGearRequestObject) (adminservice.DeleteGearResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.delete(ctx, publicKey)
	if err != nil {
		return adminservice.DeleteGear404JSONResponse(adminError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return adminservice.DeleteGear200JSONResponse(toAdminRegistration(gear)), nil
}

// GetGear implements `adminservice.StrictServerInterface.GetGear`.
func (s *Server) GetGear(ctx context.Context, request adminservice.GetGearRequestObject) (adminservice.GetGearResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return adminservice.GetGear404JSONResponse(adminError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return adminservice.GetGear200JSONResponse(toAdminRegistration(gear)), nil
}

// GetGearConfig implements `adminservice.StrictServerInterface.GetGearConfig`.
func (s *Server) GetGearConfig(ctx context.Context, request adminservice.GetGearConfigRequestObject) (adminservice.GetGearConfigResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return adminservice.GetGearConfig404JSONResponse(adminError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return adminservice.GetGearConfig200JSONResponse(gear.Configuration), nil
}

// PutGearConfig implements `adminservice.StrictServerInterface.PutGearConfig`.
func (s *Server) PutGearConfig(ctx context.Context, request adminservice.PutGearConfigRequestObject) (adminservice.PutGearConfigResponseObject, error) {
	if request.Body == nil {
		return adminservice.PutGearConfig400JSONResponse(adminError("INVALID_PARAMS", "request body required")), nil
	}
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return adminservice.PutGearConfig400JSONResponse(adminError("INVALID_PARAMS", err.Error())), nil
	}
	gear, err := s.putConfig(ctx, publicKey, *request.Body)
	if err != nil {
		if errors.Is(err, ErrGearNotFound) {
			return adminservice.PutGearConfig404JSONResponse(adminError("GEAR_NOT_FOUND", err.Error())), nil
		}
		return adminservice.PutGearConfig400JSONResponse(adminError("INVALID_PARAMS", err.Error())), nil
	}
	return adminservice.PutGearConfig200JSONResponse(gear.Configuration), nil
}

// GetGearInfo implements `adminservice.StrictServerInterface.GetGearInfo`.
func (s *Server) GetGearInfo(ctx context.Context, request adminservice.GetGearInfoRequestObject) (adminservice.GetGearInfoResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return adminservice.GetGearInfo404JSONResponse(adminError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return adminservice.GetGearInfo200JSONResponse(gear.Device), nil
}

// GetGearRuntime implements `adminservice.StrictServerInterface.GetGearRuntime`.
func (s *Server) GetGearRuntime(ctx context.Context, request adminservice.GetGearRuntimeRequestObject) (adminservice.GetGearRuntimeResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	return adminservice.GetGearRuntime200JSONResponse(toAdminRuntime(s.peerRuntime(ctx, publicKey))), nil
}

// ApproveGear implements `adminservice.StrictServerInterface.ApproveGear`.
func (s *Server) ApproveGear(ctx context.Context, request adminservice.ApproveGearRequestObject) (adminservice.ApproveGearResponseObject, error) {
	if request.Body == nil {
		return adminservice.ApproveGear400JSONResponse(adminError("INVALID_ROLE", "request body required")), nil
	}
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return adminservice.ApproveGear400JSONResponse(adminError("INVALID_PARAMS", err.Error())), nil
	}
	gear, err := s.approve(ctx, publicKey, apitypes.GearRole(request.Body.Role))
	if err != nil {
		return adminservice.ApproveGear400JSONResponse(adminError("INVALID_ROLE", err.Error())), nil
	}
	return adminservice.ApproveGear200JSONResponse(toAdminRegistration(gear)), nil
}

// BlockGear implements `adminservice.StrictServerInterface.BlockGear`.
func (s *Server) BlockGear(ctx context.Context, request adminservice.BlockGearRequestObject) (adminservice.BlockGearResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.block(ctx, publicKey)
	if err != nil {
		return adminservice.BlockGear404JSONResponse(adminError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return adminservice.BlockGear200JSONResponse(toAdminRegistration(gear)), nil
}

// RefreshGear implements `adminservice.StrictServerInterface.RefreshGear`.
func (s *Server) RefreshGear(ctx context.Context, request adminservice.RefreshGearRequestObject) (adminservice.RefreshGearResponseObject, error) {
	if s.PeerManager == nil {
		return adminservice.RefreshGear502JSONResponse(adminError("DEVICE_REFRESH_FAILED", "refresh provider not configured")), nil
	}
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	result, online, err := s.PeerManager.RefreshGear(ctx, publicKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrGearNotFound):
			return adminservice.RefreshGear404JSONResponse(adminError("GEAR_NOT_FOUND", err.Error())), nil
		case !online:
			return adminservice.RefreshGear409JSONResponse(adminError("DEVICE_OFFLINE", err.Error())), nil
		default:
			return adminservice.RefreshGear502JSONResponse(adminError("DEVICE_REFRESH_FAILED", err.Error())), nil
		}
	}
	return adminservice.RefreshGear200JSONResponse(result), nil
}

// GetConfig implements `gearservice.StrictServerInterface.GetConfig`.
func (s *Server) GetConfig(ctx context.Context, _ gearservice.GetConfigRequestObject) (gearservice.GetConfigResponseObject, error) {
	gear, err := s.get(ctx, gearservice.CallerPublicKey(ctx))
	if err != nil {
		return gearservice.GetConfig404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
	}
	cfg, err := toGearConfiguration(gear.Configuration)
	if err != nil {
		return getConfig500JSONResponse(gearError("INTERNAL_ERROR", err.Error())), nil
	}
	return gearservice.GetConfig200JSONResponse(cfg), nil
}

// GetInfo implements `gearservice.StrictServerInterface.GetInfo`.
func (s *Server) GetInfo(ctx context.Context, _ gearservice.GetInfoRequestObject) (gearservice.GetInfoResponseObject, error) {
	gear, err := s.get(ctx, gearservice.CallerPublicKey(ctx))
	if err != nil {
		return gearservice.GetInfo404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
	}
	info, err := toGearDeviceInfo(gear.Device)
	if err != nil {
		return getInfo500JSONResponse(gearError("INTERNAL_ERROR", err.Error())), nil
	}
	return gearservice.GetInfo200JSONResponse(info), nil
}

// PutInfo implements `gearservice.StrictServerInterface.PutInfo`.
func (s *Server) PutInfo(ctx context.Context, request gearservice.PutInfoRequestObject) (gearservice.PutInfoResponseObject, error) {
	if request.Body == nil {
		return gearservice.PutInfo400JSONResponse(gearError("INVALID_DEVICE_INFO", "request body required")), nil
	}
	info, err := toAdminDeviceInfo(*request.Body)
	if err != nil {
		return gearservice.PutInfo400JSONResponse(gearError("INVALID_DEVICE_INFO", err.Error())), nil
	}
	gear, err := s.putInfo(ctx, gearservice.CallerPublicKey(ctx), info)
	if err != nil {
		return gearservice.PutInfo404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
	}
	out, err := toGearDeviceInfo(gear.Device)
	if err != nil {
		return putInfo500JSONResponse(gearError("INTERNAL_ERROR", err.Error())), nil
	}
	return gearservice.PutInfo200JSONResponse(out), nil
}

// RegisterGear implements `serverpublic.StrictServerInterface.RegisterGear`.
func (s *Server) RegisterGear(ctx context.Context, request serverpublic.RegisterGearRequestObject) (serverpublic.RegisterGearResponseObject, error) {
	if request.Body == nil {
		return serverpublic.RegisterGear400JSONResponse(publicError("INVALID_PARAMS", "request body required")), nil
	}
	body := *request.Body
	body.PublicKey = serverpublic.CallerPublicKey(ctx)
	gear, err := s.register(ctx, body)
	if err != nil {
		if errors.Is(err, ErrGearAlreadyExists) {
			return serverpublic.RegisterGear409JSONResponse(publicError("GEAR_ALREADY_EXISTS", err.Error())), nil
		}
		return serverpublic.RegisterGear400JSONResponse(publicError("INVALID_PARAMS", err.Error())), nil
	}
	out, err := toPublicRegistrationResult(gear)
	if err != nil {
		return registerGear500JSONResponse(publicError("INTERNAL_ERROR", err.Error())), nil
	}
	return serverpublic.RegisterGear200JSONResponse(out), nil
}

// GetRegistration implements `gearservice.StrictServerInterface.GetRegistration`.
func (s *Server) GetRegistration(ctx context.Context, _ gearservice.GetRegistrationRequestObject) (gearservice.GetRegistrationResponseObject, error) {
	gear, err := s.get(ctx, gearservice.CallerPublicKey(ctx))
	if err != nil {
		return gearservice.GetRegistration404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return gearservice.GetRegistration200JSONResponse(toGearRegistration(gear)), nil
}

// GetRuntime implements `gearservice.StrictServerInterface.GetRuntime`.
func (s *Server) GetRuntime(ctx context.Context, _ gearservice.GetRuntimeRequestObject) (gearservice.GetRuntimeResponseObject, error) {
	return gearservice.GetRuntime200JSONResponse(s.peerRuntime(ctx, gearservice.CallerPublicKey(ctx))), nil
}

// GetServerInfo implements `serverpublic.StrictServerInterface.GetServerInfo`.
func (s *Server) GetServerInfo(_ context.Context, _ serverpublic.GetServerInfoRequestObject) (serverpublic.GetServerInfoResponseObject, error) {
	return serverpublic.GetServerInfo200JSONResponse(apitypes.ServerInfo{
		BuildCommit: s.BuildCommit,
		PublicKey:   s.ServerPublicKey,
		ServerTime:  time.Now().UnixMilli(),
	}), nil
}

func pathUnescape(value string) (string, error) {
	return url.PathUnescape(value)
}

func normalizeListParams(cursor *adminservice.Cursor, limit *adminservice.Limit) (string, int) {
	nextCursor := ""
	if cursor != nil {
		nextCursor = string(*cursor)
	}
	nextLimit := defaultListLimit
	if limit != nil {
		nextLimit = int(*limit)
	}
	if nextLimit <= 0 {
		nextLimit = defaultListLimit
	}
	if nextLimit > maxListLimit {
		nextLimit = maxListLimit
	}
	return nextCursor, nextLimit
}
