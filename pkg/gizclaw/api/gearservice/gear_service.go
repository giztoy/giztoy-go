package gearservice

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/giztoy/giztoy-go/pkg/gears"
)

type PeerManager interface {
	PeerRuntime(context.Context, string) gears.Runtime
	RefreshDevice(context.Context, string) (gears.RefreshResult, bool, error)
}

type GearServer struct {
	Gears       *gears.Service
	FirmwareOTA *firmware.OTAService
	Manager     PeerManager
}

var _ StrictServerInterface = (*GearServer)(nil)

func (s *GearServer) peerRuntime(ctx context.Context, publicKey string) gears.Runtime {
	if s.Manager == nil {
		return gears.Runtime{}
	}
	return s.Manager.PeerRuntime(ctx, publicKey)
}

func gearError(code, message string) ErrorResponse {
	return ErrorResponse{
		Error: ErrorPayload{
			Code:    code,
			Message: message,
		},
	}
}

func internalGearError(message string) ErrorResponse {
	return gearError("INTERNAL_ERROR", message)
}

func (s *GearServer) ListGears(ctx context.Context, _ ListGearsRequestObject) (ListGearsResponseObject, error) {
	items, err := s.Gears.List(ctx, gears.ListOptions{})
	if err != nil {
		return ListGears500JSONResponse(gearError("INTERNAL_ERROR", err.Error())), nil
	}
	return ListGears200JSONResponse(toGearRegistrationList(items)), nil
}

func (s *GearServer) ListByCertification(ctx context.Context, request ListByCertificationRequestObject) (ListByCertificationResponseObject, error) {
	id, err := pathUnescape(request.Id)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	items, err := s.Gears.ListByCertification(ctx, gears.GearCertificationType(request.Type), gears.GearCertificationAuthority(request.Authority), id)
	if err != nil {
		return ListByCertification500JSONResponse(gearError("INTERNAL_ERROR", err.Error())), nil
	}
	return ListByCertification200JSONResponse(toGearRegistrationList(items)), nil
}

func (s *GearServer) ListByFirmware(ctx context.Context, request ListByFirmwareRequestObject) (ListByFirmwareResponseObject, error) {
	depot, err := pathUnescape(request.Depot)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	items, err := s.Gears.ListByFirmware(ctx, depot, gears.GearFirmwareChannel(request.Channel))
	if err != nil {
		return ListByFirmware500JSONResponse(gearError("INTERNAL_ERROR", err.Error())), nil
	}
	return ListByFirmware200JSONResponse(toGearRegistrationList(items)), nil
}

func (s *GearServer) ResolveByIMEI(ctx context.Context, request ResolveByIMEIRequestObject) (ResolveByIMEIResponseObject, error) {
	tac, err := pathUnescape(request.Tac)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	serial, err := pathUnescape(request.Serial)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.Gears.ResolveByIMEI(ctx, tac, serial)
	if err != nil {
		return ResolveByIMEI404JSONResponse(gearError("GEAR_IMEI_NOT_FOUND", err.Error())), nil
	}
	return ResolveByIMEI200JSONResponse(toGearPublicKeyResponse(gear.PublicKey)), nil
}

func (s *GearServer) ListByLabel(ctx context.Context, request ListByLabelRequestObject) (ListByLabelResponseObject, error) {
	key, err := pathUnescape(request.Key)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	value, err := pathUnescape(request.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	items, err := s.Gears.ListByLabel(ctx, key, value)
	if err != nil {
		return ListByLabel500JSONResponse(gearError("INTERNAL_ERROR", err.Error())), nil
	}
	return ListByLabel200JSONResponse(toGearRegistrationList(items)), nil
}

func (s *GearServer) ResolveBySN(ctx context.Context, request ResolveBySNRequestObject) (ResolveBySNResponseObject, error) {
	sn, err := pathUnescape(request.Sn)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.Gears.ResolveBySN(ctx, sn)
	if err != nil {
		return ResolveBySN404JSONResponse(gearError("GEAR_SN_NOT_FOUND", err.Error())), nil
	}
	return ResolveBySN200JSONResponse(toGearPublicKeyResponse(gear.PublicKey)), nil
}

func (s *GearServer) DeleteGear(ctx context.Context, request DeleteGearRequestObject) (DeleteGearResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.Gears.Delete(ctx, publicKey)
	if err != nil {
		return DeleteGear404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return DeleteGear200JSONResponse(toGearRegistration(gear.Registration())), nil
}

func (s *GearServer) GetGear(ctx context.Context, request GetGearRequestObject) (GetGearResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.Gears.Get(ctx, publicKey)
	if err != nil {
		return GetGear404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return GetGear200JSONResponse(toGearRegistration(gear.Registration())), nil
}

func (s *GearServer) GetGearConfig(ctx context.Context, request GetGearConfigRequestObject) (GetGearConfigResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.Gears.Get(ctx, publicKey)
	if err != nil {
		return GetGearConfig404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
	}
	cfg, err := reencode[Configuration](gear.Configuration)
	if err != nil {
		return getGearConfig500JSONResponse(internalGearError(err.Error())), nil
	}
	return GetGearConfig200JSONResponse(cfg), nil
}

func (s *GearServer) PutGearConfig(ctx context.Context, request PutGearConfigRequestObject) (PutGearConfigResponseObject, error) {
	if request.Body == nil {
		return PutGearConfig400JSONResponse(gearError("INVALID_PARAMS", "request body required")), nil
	}
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return PutGearConfig400JSONResponse(gearError("INVALID_PARAMS", err.Error())), nil
	}
	cfg, err := reencode[gears.Configuration](*request.Body)
	if err != nil {
		return PutGearConfig400JSONResponse(gearError("INVALID_PARAMS", err.Error())), nil
	}
	gear, err := s.Gears.PutConfig(ctx, publicKey, cfg)
	if err != nil {
		if errors.Is(err, gears.ErrGearNotFound) {
			return PutGearConfig404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
		}
		return PutGearConfig400JSONResponse(gearError("INVALID_PARAMS", err.Error())), nil
	}
	out, err := reencode[Configuration](gear.Configuration)
	if err != nil {
		return putGearConfig500JSONResponse(internalGearError(err.Error())), nil
	}
	return PutGearConfig200JSONResponse(out), nil
}

func (s *GearServer) GetGearInfo(ctx context.Context, request GetGearInfoRequestObject) (GetGearInfoResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.Gears.Get(ctx, publicKey)
	if err != nil {
		return GetGearInfo404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
	}
	device, err := reencode[DeviceInfo](gear.Device)
	if err != nil {
		return getGearInfo500JSONResponse(internalGearError(err.Error())), nil
	}
	return GetGearInfo200JSONResponse(device), nil
}

func (s *GearServer) GetGearOTA(ctx context.Context, request GetGearOTARequestObject) (GetGearOTAResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.Gears.Get(ctx, publicKey)
	if err != nil {
		return GetGearOTA404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
	}
	ota, err := s.FirmwareOTA.Resolve(gear.Device.Hardware.Depot, firmware.Channel(gear.Configuration.Firmware.Channel))
	if err != nil {
		return GetGearOTA404JSONResponse(gearError("FIRMWARE_NOT_FOUND", err.Error())), nil
	}
	out, err := reencode[OTASummary](ota)
	if err != nil {
		return getGearOTA500JSONResponse(internalGearError(err.Error())), nil
	}
	return GetGearOTA200JSONResponse(out), nil
}

func (s *GearServer) GetGearRuntime(ctx context.Context, request GetGearRuntimeRequestObject) (GetGearRuntimeResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	return GetGearRuntime200JSONResponse(toGearRuntime(s.peerRuntime(ctx, publicKey))), nil
}

func (s *GearServer) ApproveGear(ctx context.Context, request ApproveGearRequestObject) (ApproveGearResponseObject, error) {
	if request.Body == nil {
		return ApproveGear400JSONResponse(gearError("INVALID_ROLE", "request body required")), nil
	}
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return ApproveGear400JSONResponse(gearError("INVALID_PARAMS", err.Error())), nil
	}
	gear, err := s.Gears.Approve(ctx, publicKey, gears.GearRole(request.Body.Role))
	if err != nil {
		return ApproveGear400JSONResponse(gearError("INVALID_ROLE", err.Error())), nil
	}
	return ApproveGear200JSONResponse(toGearRegistration(gear.Registration())), nil
}

func (s *GearServer) BlockGear(ctx context.Context, request BlockGearRequestObject) (BlockGearResponseObject, error) {
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	gear, err := s.Gears.Block(ctx, publicKey)
	if err != nil {
		return BlockGear404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return BlockGear200JSONResponse(toGearRegistration(gear.Registration())), nil
}

func (s *GearServer) RefreshGear(ctx context.Context, request RefreshGearRequestObject) (RefreshGearResponseObject, error) {
	if s.Manager == nil {
		return RefreshGear502JSONResponse(gearError("DEVICE_REFRESH_FAILED", "refresh provider not configured")), nil
	}
	publicKey, err := pathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	result, online, err := s.Manager.RefreshDevice(ctx, publicKey)
	if err != nil {
		switch {
		case errors.Is(err, gears.ErrGearNotFound):
			return RefreshGear404JSONResponse(gearError("GEAR_NOT_FOUND", err.Error())), nil
		case !online:
			return RefreshGear409JSONResponse(gearError("DEVICE_OFFLINE", err.Error())), nil
		default:
			return RefreshGear502JSONResponse(gearError("DEVICE_REFRESH_FAILED", err.Error())), nil
		}
	}
	out, err := toGearRefreshResult(result)
	if err != nil {
		return refreshGear500JSONResponse(internalGearError(err.Error())), nil
	}
	return RefreshGear200JSONResponse(out), nil
}

func pathUnescape(value string) (string, error) {
	return url.PathUnescape(value)
}
