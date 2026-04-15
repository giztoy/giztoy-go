package gear

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

var (
	ErrGearNotFound      = errors.New("gear: gear not found")
	ErrGearAlreadyExists = errors.New("gear: gear already exists")
)

type PeerManager interface {
	PeerRuntime(context.Context, string) gearservice.Runtime
	RefreshGear(context.Context, string) (gearservice.RefreshResult, bool, error)
}

type Server struct {
	Store              kv.Store
	RegistrationTokens map[string]gearservice.GearRole
	BuildCommit        string
	ServerPublicKey    string
	PeerManager        PeerManager

	mu sync.Mutex
}

// GearsAdminService reserves gear-owned admin endpoints for future expansion.
type GearsAdminService interface{}

type GearsGearService interface {
	ListGears(context.Context, gearservice.ListGearsRequestObject) (gearservice.ListGearsResponseObject, error)
	ListByCertification(context.Context, gearservice.ListByCertificationRequestObject) (gearservice.ListByCertificationResponseObject, error)
	ListByFirmware(context.Context, gearservice.ListByFirmwareRequestObject) (gearservice.ListByFirmwareResponseObject, error)
	ResolveByIMEI(context.Context, gearservice.ResolveByIMEIRequestObject) (gearservice.ResolveByIMEIResponseObject, error)
	ListByLabel(context.Context, gearservice.ListByLabelRequestObject) (gearservice.ListByLabelResponseObject, error)
	ResolveBySN(context.Context, gearservice.ResolveBySNRequestObject) (gearservice.ResolveBySNResponseObject, error)
	DeleteGear(context.Context, gearservice.DeleteGearRequestObject) (gearservice.DeleteGearResponseObject, error)
	GetGear(context.Context, gearservice.GetGearRequestObject) (gearservice.GetGearResponseObject, error)
	GetGearConfig(context.Context, gearservice.GetGearConfigRequestObject) (gearservice.GetGearConfigResponseObject, error)
	PutGearConfig(context.Context, gearservice.PutGearConfigRequestObject) (gearservice.PutGearConfigResponseObject, error)
	GetGearInfo(context.Context, gearservice.GetGearInfoRequestObject) (gearservice.GetGearInfoResponseObject, error)
	GetGearRuntime(context.Context, gearservice.GetGearRuntimeRequestObject) (gearservice.GetGearRuntimeResponseObject, error)
	ApproveGear(context.Context, gearservice.ApproveGearRequestObject) (gearservice.ApproveGearResponseObject, error)
	BlockGear(context.Context, gearservice.BlockGearRequestObject) (gearservice.BlockGearResponseObject, error)
	RefreshGear(context.Context, gearservice.RefreshGearRequestObject) (gearservice.RefreshGearResponseObject, error)
}

type GearsServerPublic interface {
	GetConfig(context.Context, serverpublic.GetConfigRequestObject) (serverpublic.GetConfigResponseObject, error)
	GetInfo(context.Context, serverpublic.GetInfoRequestObject) (serverpublic.GetInfoResponseObject, error)
	PutInfo(context.Context, serverpublic.PutInfoRequestObject) (serverpublic.PutInfoResponseObject, error)
	RegisterGear(context.Context, serverpublic.RegisterGearRequestObject) (serverpublic.RegisterGearResponseObject, error)
	GetRegistration(context.Context, serverpublic.GetRegistrationRequestObject) (serverpublic.GetRegistrationResponseObject, error)
	GetRuntime(context.Context, serverpublic.GetRuntimeRequestObject) (serverpublic.GetRuntimeResponseObject, error)
	GetServerInfo(context.Context, serverpublic.GetServerInfoRequestObject) (serverpublic.GetServerInfoResponseObject, error)
}

var _ GearsGearService = (*Server)(nil)
var _ GearsServerPublic = (*Server)(nil)

// ListGears implements `gearservice.StrictServerInterface.ListGears`.
func (s *Server) ListGears(ctx context.Context, _ gearservice.ListGearsRequestObject) (gearservice.ListGearsResponseObject, error) {
	items, err := s.list(ctx)
	if err != nil {
		return gearservice.ListGears500JSONResponse(gearError("INTERNAL_ERROR", err.Error())), nil
	}
	return gearservice.ListGears200JSONResponse(toGearRegistrationList(items)), nil
}

// ListByCertification implements `gearservice.StrictServerInterface.ListByCertification`.
func (s *Server) ListByCertification(ctx context.Context, request gearservice.ListByCertificationRequestObject) (gearservice.ListByCertificationResponseObject, error) {
	id, err := pathUnescape(request.Id)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	items, err := s.listByCertification(ctx, request.Type, request.Authority, id)
	if err != nil {
		return gearservice.ListByCertification500JSONResponse(gearError("INTERNAL_ERROR", err.Error())), nil
	}
	return gearservice.ListByCertification200JSONResponse(toGearRegistrationList(items)), nil
}

// ListByFirmware implements `gearservice.StrictServerInterface.ListByFirmware`.
func (s *Server) ListByFirmware(ctx context.Context, request gearservice.ListByFirmwareRequestObject) (gearservice.ListByFirmwareResponseObject, error) {
	depot, err := pathUnescape(request.Depot)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	items, err := s.listByFirmware(ctx, depot, request.Channel)
	if err != nil {
		return gearservice.ListByFirmware500JSONResponse(gearError("INTERNAL_ERROR", err.Error())), nil
	}
	return gearservice.ListByFirmware200JSONResponse(toGearRegistrationList(items)), nil
}

// ResolveByIMEI implements `gearservice.StrictServerInterface.ResolveByIMEI`.
func (s *Server) ResolveByIMEI(ctx context.Context, request gearservice.ResolveByIMEIRequestObject) (gearservice.ResolveByIMEIResponseObject, error) {
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
		return gearservice.ResolveByIMEI404JSONResponse(gearError("GEAR_IMEI_NOT_FOUND", err.Error())), nil
	}
	return gearservice.ResolveByIMEI200JSONResponse(gearservice.PublicKeyResponse{PublicKey: publicKey}), nil
}

// ListByLabel implements `gearservice.StrictServerInterface.ListByLabel`.
func (s *Server) ListByLabel(ctx context.Context, request gearservice.ListByLabelRequestObject) (gearservice.ListByLabelResponseObject, error) {
	key, err := pathUnescape(request.Key)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	value, err := pathUnescape(request.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	items, err := s.listByLabel(ctx, key, value)
	if err != nil {
		return gearservice.ListByLabel500JSONResponse(gearError("INTERNAL_ERROR", err.Error())), nil
	}
	return gearservice.ListByLabel200JSONResponse(toGearRegistrationList(items)), nil
}

// ResolveBySN implements `gearservice.StrictServerInterface.ResolveBySN`.
func (s *Server) ResolveBySN(ctx context.Context, request gearservice.ResolveBySNRequestObject) (gearservice.ResolveBySNResponseObject, error) {
	sn, err := pathUnescape(request.Sn)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	publicKey, err := s.resolveBySN(ctx, sn)
	if err != nil {
		return gearservice.ResolveBySN404JSONResponse(gearError("GEAR_SN_NOT_FOUND", err.Error())), nil
	}
	return gearservice.ResolveBySN200JSONResponse(gearservice.PublicKeyResponse{PublicKey: publicKey}), nil
}

// DeleteGear implements `gearservice.StrictServerInterface.DeleteGear`.
func (s *Server) DeleteGear(ctx context.Context, request gearservice.DeleteGearRequestObject) (gearservice.DeleteGearResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.delete(ctx, publicKey)
	if err != nil {
		return gearservice.DeleteGear404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return gearservice.DeleteGear200JSONResponse(toGearRegistration(gear)), nil
}

// GetGear implements `gearservice.StrictServerInterface.GetGear`.
func (s *Server) GetGear(ctx context.Context, request gearservice.GetGearRequestObject) (gearservice.GetGearResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return gearservice.GetGear404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return gearservice.GetGear200JSONResponse(toGearRegistration(gear)), nil
}

// GetGearConfig implements `gearservice.StrictServerInterface.GetGearConfig`.
func (s *Server) GetGearConfig(ctx context.Context, request gearservice.GetGearConfigRequestObject) (gearservice.GetGearConfigResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return gearservice.GetGearConfig404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return gearservice.GetGearConfig200JSONResponse(gear.Configuration), nil
}

// PutGearConfig implements `gearservice.StrictServerInterface.PutGearConfig`.
func (s *Server) PutGearConfig(ctx context.Context, request gearservice.PutGearConfigRequestObject) (gearservice.PutGearConfigResponseObject, error) {
	if request.Body == nil {
		return gearservice.PutGearConfig400JSONResponse(gearError("INVALID_PARAMS", "request body required")), nil
	}
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return gearservice.PutGearConfig400JSONResponse(gearError("INVALID_PARAMS", err.Error())), nil
	}
	gear, err := s.putConfig(ctx, publicKey, *request.Body)
	if err != nil {
		if errors.Is(err, ErrGearNotFound) {
			return gearservice.PutGearConfig404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
		}
		return gearservice.PutGearConfig400JSONResponse(gearError("INVALID_PARAMS", err.Error())), nil
	}
	return gearservice.PutGearConfig200JSONResponse(gear.Configuration), nil
}

// GetGearInfo implements `gearservice.StrictServerInterface.GetGearInfo`.
func (s *Server) GetGearInfo(ctx context.Context, request gearservice.GetGearInfoRequestObject) (gearservice.GetGearInfoResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return gearservice.GetGearInfo404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return gearservice.GetGearInfo200JSONResponse(gear.Device), nil
}

// GetGearRuntime implements `gearservice.StrictServerInterface.GetGearRuntime`.
func (s *Server) GetGearRuntime(ctx context.Context, request gearservice.GetGearRuntimeRequestObject) (gearservice.GetGearRuntimeResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	return gearservice.GetGearRuntime200JSONResponse(s.peerRuntime(ctx, publicKey)), nil
}

// ApproveGear implements `gearservice.StrictServerInterface.ApproveGear`.
func (s *Server) ApproveGear(ctx context.Context, request gearservice.ApproveGearRequestObject) (gearservice.ApproveGearResponseObject, error) {
	if request.Body == nil {
		return gearservice.ApproveGear400JSONResponse(gearError("INVALID_ROLE", "request body required")), nil
	}
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return gearservice.ApproveGear400JSONResponse(gearError("INVALID_PARAMS", err.Error())), nil
	}
	gear, err := s.approve(ctx, publicKey, request.Body.Role)
	if err != nil {
		return gearservice.ApproveGear400JSONResponse(gearError("INVALID_ROLE", err.Error())), nil
	}
	return gearservice.ApproveGear200JSONResponse(toGearRegistration(gear)), nil
}

// BlockGear implements `gearservice.StrictServerInterface.BlockGear`.
func (s *Server) BlockGear(ctx context.Context, request gearservice.BlockGearRequestObject) (gearservice.BlockGearResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.block(ctx, publicKey)
	if err != nil {
		return gearservice.BlockGear404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return gearservice.BlockGear200JSONResponse(toGearRegistration(gear)), nil
}

// RefreshGear implements `gearservice.StrictServerInterface.RefreshGear`.
func (s *Server) RefreshGear(ctx context.Context, request gearservice.RefreshGearRequestObject) (gearservice.RefreshGearResponseObject, error) {
	if s.PeerManager == nil {
		return gearservice.RefreshGear502JSONResponse(gearError("DEVICE_REFRESH_FAILED", "refresh provider not configured")), nil
	}
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	result, online, err := s.PeerManager.RefreshGear(ctx, publicKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrGearNotFound):
			return gearservice.RefreshGear404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
		case !online:
			return gearservice.RefreshGear409JSONResponse(gearError("DEVICE_OFFLINE", err.Error())), nil
		default:
			return gearservice.RefreshGear502JSONResponse(gearError("DEVICE_REFRESH_FAILED", err.Error())), nil
		}
	}
	return gearservice.RefreshGear200JSONResponse(result), nil
}

// GetConfig implements `serverpublic.StrictServerInterface.GetConfig`.
func (s *Server) GetConfig(ctx context.Context, _ serverpublic.GetConfigRequestObject) (serverpublic.GetConfigResponseObject, error) {
	gear, err := s.get(ctx, serverpublic.CallerPublicKey(ctx))
	if err != nil {
		return serverpublic.GetConfig404JSONResponse(publicError("GEAR_NOT_FOUND", err.Error())), nil
	}
	cfg, err := toPublicConfiguration(gear.Configuration)
	if err != nil {
		return getConfig500JSONResponse(publicError("INTERNAL_ERROR", err.Error())), nil
	}
	return serverpublic.GetConfig200JSONResponse(cfg), nil
}

// GetInfo implements `serverpublic.StrictServerInterface.GetInfo`.
func (s *Server) GetInfo(ctx context.Context, _ serverpublic.GetInfoRequestObject) (serverpublic.GetInfoResponseObject, error) {
	gear, err := s.get(ctx, serverpublic.CallerPublicKey(ctx))
	if err != nil {
		return serverpublic.GetInfo404JSONResponse(publicError("GEAR_NOT_FOUND", err.Error())), nil
	}
	info, err := toPublicDeviceInfo(gear.Device)
	if err != nil {
		return getInfo500JSONResponse(publicError("INTERNAL_ERROR", err.Error())), nil
	}
	return serverpublic.GetInfo200JSONResponse(info), nil
}

// PutInfo implements `serverpublic.StrictServerInterface.PutInfo`.
func (s *Server) PutInfo(ctx context.Context, request serverpublic.PutInfoRequestObject) (serverpublic.PutInfoResponseObject, error) {
	if request.Body == nil {
		return serverpublic.PutInfo400JSONResponse(publicError("INVALID_DEVICE_INFO", "request body required")), nil
	}
	body, err := toGearDeviceInfo(*request.Body)
	if err != nil {
		return serverpublic.PutInfo400JSONResponse(publicError("INVALID_DEVICE_INFO", err.Error())), nil
	}
	gear, err := s.putInfo(ctx, serverpublic.CallerPublicKey(ctx), body)
	if err != nil {
		return serverpublic.PutInfo404JSONResponse(publicError("GEAR_NOT_FOUND", err.Error())), nil
	}
	info, err := toPublicDeviceInfo(gear.Device)
	if err != nil {
		return putInfo500JSONResponse(publicError("INTERNAL_ERROR", err.Error())), nil
	}
	return serverpublic.PutInfo200JSONResponse(info), nil
}

// RegisterGear implements `serverpublic.StrictServerInterface.RegisterGear`.
func (s *Server) RegisterGear(ctx context.Context, request serverpublic.RegisterGearRequestObject) (serverpublic.RegisterGearResponseObject, error) {
	if request.Body == nil {
		return serverpublic.RegisterGear400JSONResponse(publicError("INVALID_PARAMS", "request body required")), nil
	}
	body, err := toGearRegistrationRequest(*request.Body)
	if err != nil {
		return serverpublic.RegisterGear400JSONResponse(publicError("INVALID_PARAMS", err.Error())), nil
	}
	body.PublicKey = serverpublic.CallerPublicKey(ctx)
	result, err := s.register(ctx, body)
	if err != nil {
		if errors.Is(err, ErrGearAlreadyExists) {
			return serverpublic.RegisterGear409JSONResponse(publicError("GEAR_ALREADY_EXISTS", err.Error())), nil
		}
		return serverpublic.RegisterGear400JSONResponse(publicError("INVALID_PARAMS", err.Error())), nil
	}
	out, err := toPublicRegistrationResult(result)
	if err != nil {
		return registerGear500JSONResponse(publicError("INTERNAL_ERROR", err.Error())), nil
	}
	return serverpublic.RegisterGear200JSONResponse(out), nil
}

// GetRegistration implements `serverpublic.StrictServerInterface.GetRegistration`.
func (s *Server) GetRegistration(ctx context.Context, _ serverpublic.GetRegistrationRequestObject) (serverpublic.GetRegistrationResponseObject, error) {
	gear, err := s.get(ctx, serverpublic.CallerPublicKey(ctx))
	if err != nil {
		return serverpublic.GetRegistration404JSONResponse(publicError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return serverpublic.GetRegistration200JSONResponse(toPublicRegistration(gear)), nil
}

// GetRuntime implements `serverpublic.StrictServerInterface.GetRuntime`.
func (s *Server) GetRuntime(ctx context.Context, _ serverpublic.GetRuntimeRequestObject) (serverpublic.GetRuntimeResponseObject, error) {
	return serverpublic.GetRuntime200JSONResponse(toPublicRuntime(s.peerRuntime(ctx, serverpublic.CallerPublicKey(ctx)))), nil
}

// GetServerInfo implements `serverpublic.StrictServerInterface.GetServerInfo`.
func (s *Server) GetServerInfo(_ context.Context, _ serverpublic.GetServerInfoRequestObject) (serverpublic.GetServerInfoResponseObject, error) {
	return serverpublic.GetServerInfo200JSONResponse(serverpublic.ServerInfo{
		BuildCommit: s.BuildCommit,
		PublicKey:   s.ServerPublicKey,
		ServerTime:  time.Now().UnixMilli(),
	}), nil
}

func pathUnescape(value string) (string, error) {
	return url.PathUnescape(value)
}
