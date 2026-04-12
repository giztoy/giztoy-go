package serverpublic

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"time"

	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/giztoy/giztoy-go/pkg/gears"
)

type callerPublicKeyContextKey string

const callerPublicKeyKey callerPublicKeyContextKey = "caller_public_key"

func WithCallerPublicKey(ctx context.Context, publicKey string) context.Context {
	return context.WithValue(ctx, callerPublicKeyKey, publicKey)
}

func CallerPublicKey(ctx context.Context) string {
	value, _ := ctx.Value(callerPublicKeyKey).(string)
	return value
}

type PeerRuntimeProvider interface {
	PeerRuntime(context.Context, string) gears.Runtime
}

type PublicServer struct {
	BuildCommit     string
	ServerPublicKey string
	Gears           *gears.Service
	FirmwareOTA     *firmware.OTAService
	PeerServer      PeerRuntimeProvider
	CallerPublicKey func(context.Context) string
	Now             func() time.Time
}

var _ StrictServerInterface = (*PublicServer)(nil)

func (s *PublicServer) callerPublicKey(ctx context.Context) string {
	if s.CallerPublicKey == nil {
		return CallerPublicKey(ctx)
	}
	return s.CallerPublicKey(ctx)
}

func (s *PublicServer) peerRuntime(ctx context.Context, publicKey string) gears.Runtime {
	if s.PeerServer == nil {
		return gears.Runtime{}
	}
	return s.PeerServer.PeerRuntime(ctx, publicKey)
}

func (s *PublicServer) now() time.Time {
	if s.Now == nil {
		return time.Now()
	}
	return s.Now()
}

func publicError(code, message string) ErrorResponse {
	return ErrorResponse{
		Error: ErrorPayload{
			Code:    code,
			Message: message,
		},
	}
}

func internalPublicError(message string) ErrorResponse {
	return publicError("INTERNAL_ERROR", message)
}

func toPublicRegistration(in gears.Registration) Registration {
	return Registration{
		ApprovedAt:     millisPtr(in.ApprovedAt),
		AutoRegistered: boolPtr(in.AutoRegistered),
		CreatedAt:      millisTime(in.CreatedAt),
		PublicKey:      in.PublicKey,
		Role:           GearRole(in.Role),
		Status:         GearStatus(in.Status),
		UpdatedAt:      millisTime(in.UpdatedAt),
	}
}

func toPublicGear(in gears.Gear) (Gear, error) {
	device, err := reencode[DeviceInfo](in.Device)
	if err != nil {
		return Gear{}, err
	}
	cfg, err := reencode[Configuration](in.Configuration)
	if err != nil {
		return Gear{}, err
	}
	return Gear{
		ApprovedAt:     millisPtr(in.ApprovedAt),
		AutoRegistered: boolPtr(in.AutoRegistered),
		Configuration:  cfg,
		CreatedAt:      millisTime(in.CreatedAt),
		Device:         device,
		PublicKey:      in.PublicKey,
		Role:           GearRole(in.Role),
		Status:         GearStatus(in.Status),
		UpdatedAt:      millisTime(in.UpdatedAt),
	}, nil
}

func toPublicRegistrationResult(in gears.RegistrationResult) (RegistrationResult, error) {
	gear, err := toPublicGear(in.Gear)
	if err != nil {
		return RegistrationResult{}, err
	}
	return RegistrationResult{
		Gear:         gear,
		Registration: toPublicRegistration(in.Registered),
	}, nil
}

func toPublicRuntime(in gears.Runtime) Runtime {
	return Runtime{
		LastAddr:   stringPtr(in.LastAddr),
		LastSeenAt: millisTime(in.LastSeenAt),
		Online:     in.Online,
	}
}

func (s *PublicServer) GetConfig(ctx context.Context, _ GetConfigRequestObject) (GetConfigResponseObject, error) {
	gear, err := s.Gears.Get(ctx, s.callerPublicKey(ctx))
	if err != nil {
		return GetConfig404JSONResponse(publicError("GEAR_NOT_FOUND", err.Error())), nil
	}
	cfg, err := reencode[Configuration](gear.Configuration)
	if err != nil {
		return getConfig500JSONResponse(internalPublicError(err.Error())), nil
	}
	return GetConfig200JSONResponse(cfg), nil
}

func (s *PublicServer) DownloadFirmware(ctx context.Context, request DownloadFirmwareRequestObject) (DownloadFirmwareResponseObject, error) {
	gear, err := s.Gears.Get(ctx, s.callerPublicKey(ctx))
	if err != nil {
		return DownloadFirmware404JSONResponse(publicError("GEAR_NOT_FOUND", err.Error())), nil
	}
	if gear.Device.Hardware.Depot == "" || gear.Configuration.Firmware.Channel == "" {
		return DownloadFirmware404JSONResponse(publicError("OTA_NOT_AVAILABLE", "missing depot or channel")), nil
	}
	path, err := url.PathUnescape(request.Path)
	if err != nil {
		return DownloadFirmware400JSONResponse(publicError("INVALID_PARAMS", err.Error())), nil
	}
	fullPath, fileMeta, err := s.FirmwareOTA.ResolveFile(gear.Device.Hardware.Depot, firmware.Channel(gear.Configuration.Firmware.Channel), path)
	if err != nil {
		switch {
		case errors.Is(err, firmware.ErrInvalidPath):
			return DownloadFirmware400JSONResponse(publicError("INVALID_PARAMS", err.Error())), nil
		default:
			return DownloadFirmware404JSONResponse(publicError("FIRMWARE_FILE_NOT_FOUND", err.Error())), nil
		}
	}
	file, err := os.Open(fullPath)
	if err != nil {
		return downloadFirmware500JSONResponse(internalPublicError(err.Error())), nil
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return downloadFirmware500JSONResponse(internalPublicError(err.Error())), nil
	}
	return DownloadFirmware200ApplicationoctetStreamResponse{
		Body: file,
		Headers: DownloadFirmware200ResponseHeaders{
			XChecksumMD5:    fileMeta.MD5,
			XChecksumSHA256: fileMeta.SHA256,
		},
		ContentLength: info.Size(),
	}, nil
}

func (s *PublicServer) GetInfo(ctx context.Context, _ GetInfoRequestObject) (GetInfoResponseObject, error) {
	gear, err := s.Gears.Get(ctx, s.callerPublicKey(ctx))
	if err != nil {
		return GetInfo404JSONResponse(publicError("GEAR_NOT_FOUND", err.Error())), nil
	}
	device, err := reencode[DeviceInfo](gear.Device)
	if err != nil {
		return getInfo500JSONResponse(internalPublicError(err.Error())), nil
	}
	return GetInfo200JSONResponse(device), nil
}

func (s *PublicServer) PutInfo(ctx context.Context, request PutInfoRequestObject) (PutInfoResponseObject, error) {
	if request.Body == nil {
		return PutInfo400JSONResponse(publicError("INVALID_DEVICE_INFO", "request body required")), nil
	}
	body, err := reencode[gears.DeviceInfo](*request.Body)
	if err != nil {
		return PutInfo400JSONResponse(publicError("INVALID_DEVICE_INFO", err.Error())), nil
	}
	gear, err := s.Gears.PutInfo(ctx, s.callerPublicKey(ctx), body)
	if err != nil {
		return PutInfo404JSONResponse(publicError("GEAR_NOT_FOUND", err.Error())), nil
	}
	device, err := reencode[DeviceInfo](gear.Device)
	if err != nil {
		return putInfo500JSONResponse(internalPublicError(err.Error())), nil
	}
	return PutInfo200JSONResponse(device), nil
}

func (s *PublicServer) GetOTA(ctx context.Context, _ GetOTARequestObject) (GetOTAResponseObject, error) {
	gear, err := s.Gears.Get(ctx, s.callerPublicKey(ctx))
	if err != nil {
		return GetOTA404JSONResponse(publicError("GEAR_NOT_FOUND", err.Error())), nil
	}
	if gear.Device.Hardware.Depot == "" || gear.Configuration.Firmware.Channel == "" {
		return GetOTA404JSONResponse(publicError("OTA_NOT_AVAILABLE", "missing depot or channel")), nil
	}
	ota, err := s.FirmwareOTA.Resolve(gear.Device.Hardware.Depot, firmware.Channel(gear.Configuration.Firmware.Channel))
	if err != nil {
		return GetOTA404JSONResponse(publicError("FIRMWARE_NOT_FOUND", err.Error())), nil
	}
	out, err := reencode[OTASummary](ota)
	if err != nil {
		return getOTA500JSONResponse(internalPublicError(err.Error())), nil
	}
	return GetOTA200JSONResponse(out), nil
}

func (s *PublicServer) RegisterGear(ctx context.Context, request RegisterGearRequestObject) (RegisterGearResponseObject, error) {
	if request.Body == nil {
		return RegisterGear400JSONResponse(publicError("INVALID_PARAMS", "request body required")), nil
	}
	body, err := reencode[gears.RegistrationRequest](*request.Body)
	if err != nil {
		return RegisterGear400JSONResponse(publicError("INVALID_PARAMS", err.Error())), nil
	}
	body.PublicKey = s.callerPublicKey(ctx)
	result, err := s.Gears.Register(ctx, body)
	if err != nil {
		if errors.Is(err, gears.ErrGearAlreadyExists) {
			return RegisterGear409JSONResponse(publicError("GEAR_ALREADY_EXISTS", err.Error())), nil
		}
		return RegisterGear400JSONResponse(publicError("INVALID_PARAMS", err.Error())), nil
	}
	out, err := toPublicRegistrationResult(result)
	if err != nil {
		return registerGear500JSONResponse(internalPublicError(err.Error())), nil
	}
	return RegisterGear200JSONResponse(out), nil
}

func (s *PublicServer) GetRegistration(ctx context.Context, _ GetRegistrationRequestObject) (GetRegistrationResponseObject, error) {
	gear, err := s.Gears.Get(ctx, s.callerPublicKey(ctx))
	if err != nil {
		return GetRegistration404JSONResponse(publicError("GEAR_NOT_FOUND", err.Error())), nil
	}
	return GetRegistration200JSONResponse(toPublicRegistration(gear.Registration())), nil
}

func (s *PublicServer) GetRuntime(ctx context.Context, _ GetRuntimeRequestObject) (GetRuntimeResponseObject, error) {
	runtime := s.peerRuntime(ctx, s.callerPublicKey(ctx))
	return GetRuntime200JSONResponse(toPublicRuntime(runtime)), nil
}

func (s *PublicServer) GetServerInfo(_ context.Context, _ GetServerInfoRequestObject) (GetServerInfoResponseObject, error) {
	return GetServerInfo200JSONResponse(ServerInfo{
		BuildCommit: s.BuildCommit,
		PublicKey:   s.ServerPublicKey,
		ServerTime:  s.now().UnixMilli(),
	}), nil
}

var _ io.Reader = (*os.File)(nil)
