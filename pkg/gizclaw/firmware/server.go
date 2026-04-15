package firmware

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"path"
	"sync"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
	"github.com/GizClaw/gizclaw-go/pkg/store/depotstore"
	"github.com/gofiber/fiber/v2"
)

type Server struct {
	Store depotstore.Store

	// ResolveGearTarget maps a gear public key to the OTA depot/channel it should read from.
	ResolveGearTarget func(ctx context.Context, publicKey string) (depot string, channel Channel, err error)

	mu      sync.Mutex
	depotMu map[string]*sync.Mutex
}

type FirmwareAdminService interface {
	ListDepots(context.Context, adminservice.ListDepotsRequestObject) (adminservice.ListDepotsResponseObject, error)
	GetDepot(context.Context, adminservice.GetDepotRequestObject) (adminservice.GetDepotResponseObject, error)
	PutDepotInfo(context.Context, adminservice.PutDepotInfoRequestObject) (adminservice.PutDepotInfoResponseObject, error)
	GetChannel(context.Context, adminservice.GetChannelRequestObject) (adminservice.GetChannelResponseObject, error)
	PutChannel(context.Context, adminservice.PutChannelRequestObject) (adminservice.PutChannelResponseObject, error)
	ReleaseDepot(context.Context, adminservice.ReleaseDepotRequestObject) (adminservice.ReleaseDepotResponseObject, error)
	RollbackDepot(context.Context, adminservice.RollbackDepotRequestObject) (adminservice.RollbackDepotResponseObject, error)
}

type FirmwareGearService interface {
	GetGearOTA(context.Context, gearservice.GetGearOTARequestObject) (gearservice.GetGearOTAResponseObject, error)
}

type FirmwareServerPublic interface {
	GetOTA(context.Context, serverpublic.GetOTARequestObject) (serverpublic.GetOTAResponseObject, error)
	DownloadFirmware(context.Context, serverpublic.DownloadFirmwareRequestObject) (serverpublic.DownloadFirmwareResponseObject, error)
}

var _ FirmwareAdminService = (*Server)(nil)
var _ FirmwareGearService = (*Server)(nil)
var _ FirmwareServerPublic = (*Server)(nil)

// ListDepots implements `adminservice.StrictServerInterface.ListDepots`.
func (s *Server) ListDepots(_ context.Context, _ adminservice.ListDepotsRequestObject) (adminservice.ListDepotsResponseObject, error) {
	names, err := s.scanDepotNames()
	if err != nil {
		return adminservice.ListDepots500JSONResponse(adminError("DIRECTORY_SCAN_FAILED", err.Error())), nil
	}
	items := make([]adminservice.Depot, 0, len(names))
	for _, name := range names {
		depot, err := s.scanDepot(name)
		if err != nil {
			return adminservice.ListDepots500JSONResponse(adminError("DIRECTORY_SCAN_FAILED", err.Error())), nil
		}
		items = append(items, depot)
	}
	return adminservice.ListDepots200JSONResponse(adminservice.DepotList{Items: items}), nil
}

// GetDepot implements `adminservice.StrictServerInterface.GetDepot`.
func (s *Server) GetDepot(_ context.Context, request adminservice.GetDepotRequestObject) (adminservice.GetDepotResponseObject, error) {
	depotName, err := url.PathUnescape(request.Depot)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	depot, err := s.scanDepot(depotName)
	if err != nil {
		return adminservice.GetDepot404JSONResponse(adminError("DEPOT_NOT_FOUND", err.Error())), nil
	}
	return adminservice.GetDepot200JSONResponse(depot), nil
}

// PutDepotInfo implements `adminservice.StrictServerInterface.PutDepotInfo`.
func (s *Server) PutDepotInfo(_ context.Context, request adminservice.PutDepotInfoRequestObject) (adminservice.PutDepotInfoResponseObject, error) {
	if request.Body == nil {
		return adminservice.PutDepotInfo400JSONResponse(adminError("INVALID_JSON", "request body required")), nil
	}
	depotName, err := url.PathUnescape(request.Depot)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	info := normalizeDepotInfo(*request.Body)
	unlock := s.lockDepot(depotName)
	defer unlock()
	if err := s.ensureDepot(depotName); err != nil {
		return nil, err
	}
	if err := validateDepotInfo(info); err != nil {
		return adminservice.PutDepotInfo400JSONResponse(adminError("INVALID_JSON", err.Error())), nil
	}
	if current, err := s.scanDepot(depotName); err == nil {
		for _, channel := range allChannels() {
			release, ok := depotRelease(current, channel)
			if !ok {
				continue
			}
			if !sameInfoFiles(info, release) {
				return adminservice.PutDepotInfo409JSONResponse(adminError("INFO_FILES_MISMATCH", "firmware files mismatch")), nil
			}
		}
	}
	if err := writeInfo(s.store(), s.infoPath(depotName), info); err != nil {
		return adminservice.PutDepotInfo400JSONResponse(adminError("INVALID_JSON", err.Error())), nil
	}
	depot, err := s.scanDepot(depotName)
	if err != nil {
		return adminservice.PutDepotInfo500JSONResponse(adminError("DIRECTORY_SCAN_FAILED", err.Error())), nil
	}
	return adminservice.PutDepotInfo200JSONResponse(depot), nil
}

// GetChannel implements `adminservice.StrictServerInterface.GetChannel`.
func (s *Server) GetChannel(_ context.Context, request adminservice.GetChannelRequestObject) (adminservice.GetChannelResponseObject, error) {
	depotName, err := url.PathUnescape(request.Depot)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	depot, err := s.scanDepot(depotName)
	if err != nil {
		return adminservice.GetChannel404JSONResponse(adminError("DEPOT_NOT_FOUND", err.Error())), nil
	}
	release, ok := depotRelease(depot, Channel(request.Channel))
	if !ok {
		return adminservice.GetChannel404JSONResponse(adminError("CHANNEL_NOT_FOUND", "channel not found")), nil
	}
	return adminservice.GetChannel200JSONResponse(release), nil
}

// PutChannel implements `adminservice.StrictServerInterface.PutChannel`.
func (s *Server) PutChannel(_ context.Context, request adminservice.PutChannelRequestObject) (adminservice.PutChannelResponseObject, error) {
	depotName, err := url.PathUnescape(request.Depot)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	release, err := s.uploadTar(depotName, Channel(request.Channel), request.Body)
	if err != nil {
		return adminservice.PutChannel409JSONResponse(adminError("MANIFEST_INVALID", err.Error())), nil
	}
	return adminservice.PutChannel200JSONResponse(release), nil
}

// ReleaseDepot implements `adminservice.StrictServerInterface.ReleaseDepot`.
func (s *Server) ReleaseDepot(_ context.Context, request adminservice.ReleaseDepotRequestObject) (adminservice.ReleaseDepotResponseObject, error) {
	depotName, err := url.PathUnescape(request.Depot)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	depot, err := s.releaseDepot(depotName)
	if err != nil {
		switch {
		case errors.Is(err, errDepotNotFound):
			return adminservice.ReleaseDepot404JSONResponse(adminError("DEPOT_NOT_FOUND", err.Error())), nil
		case errors.Is(err, errChannelNotFound):
			return adminservice.ReleaseDepot409JSONResponse(adminError("RELEASE_NOT_READY", err.Error())), nil
		default:
			return adminservice.ReleaseDepot500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
		}
	}
	return adminservice.ReleaseDepot200JSONResponse(depot), nil
}

// RollbackDepot implements `adminservice.StrictServerInterface.RollbackDepot`.
func (s *Server) RollbackDepot(_ context.Context, request adminservice.RollbackDepotRequestObject) (adminservice.RollbackDepotResponseObject, error) {
	depotName, err := url.PathUnescape(request.Depot)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	depot, err := s.rollbackDepot(depotName)
	if err != nil {
		switch {
		case errors.Is(err, errDepotNotFound):
			return adminservice.RollbackDepot404JSONResponse(adminError("DEPOT_NOT_FOUND", err.Error())), nil
		case errors.Is(err, errChannelNotFound):
			return adminservice.RollbackDepot409JSONResponse(adminError("ROLLBACK_NOT_AVAILABLE", err.Error())), nil
		default:
			return adminservice.RollbackDepot500JSONResponse(adminError("INTERNAL_ERROR", err.Error())), nil
		}
	}
	return adminservice.RollbackDepot200JSONResponse(depot), nil
}

// GetGearOTA implements `gearservice.StrictServerInterface.GetGearOTA`.
// OTA is colocated here because firmware storage owns the underlying data.
func (s *Server) GetGearOTA(ctx context.Context, request gearservice.GetGearOTARequestObject) (gearservice.GetGearOTAResponseObject, error) {
	if s.ResolveGearTarget == nil {
		return gearservice.GetGearOTA404JSONResponse(gearError("OTA_NOT_AVAILABLE", "gear target resolver not configured")), nil
	}
	publicKey, err := url.PathUnescape(string(request.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	depotName, channel, err := s.ResolveGearTarget(ctx, publicKey)
	if err != nil || depotName == "" || channel == "" {
		if err == nil {
			err = fmt.Errorf("missing depot or channel")
		}
		return gearservice.GetGearOTA404JSONResponse(gearError("OTA_NOT_AVAILABLE", err.Error())), nil
	}
	ota, err := s.resolveOTA(depotName, channel)
	if err != nil {
		return gearservice.GetGearOTA404JSONResponse(gearError("FIRMWARE_NOT_FOUND", err.Error())), nil
	}
	return gearservice.GetGearOTA200JSONResponse(ota), nil
}

// GetOTA implements `serverpublic.StrictServerInterface.GetOTA`.
func (s *Server) GetOTA(ctx context.Context, _ serverpublic.GetOTARequestObject) (serverpublic.GetOTAResponseObject, error) {
	depotName, channel, err := s.resolveCallerTarget(ctx)
	if err != nil {
		return serverpublic.GetOTA404JSONResponse(publicError("OTA_NOT_AVAILABLE", err.Error())), nil
	}
	ota, err := s.resolveOTA(depotName, channel)
	if err != nil {
		return serverpublic.GetOTA404JSONResponse(publicError("FIRMWARE_NOT_FOUND", err.Error())), nil
	}
	return serverpublic.GetOTA200JSONResponse(toPublicOTASummary(ota)), nil
}

// DownloadFirmware implements `serverpublic.StrictServerInterface.DownloadFirmware`.
func (s *Server) DownloadFirmware(ctx context.Context, request serverpublic.DownloadFirmwareRequestObject) (serverpublic.DownloadFirmwareResponseObject, error) {
	depotName, channel, err := s.resolveCallerTarget(ctx)
	if err != nil {
		return serverpublic.DownloadFirmware404JSONResponse(publicError("OTA_NOT_AVAILABLE", err.Error())), nil
	}
	filePath, err := url.PathUnescape(request.Path)
	if err != nil {
		return serverpublic.DownloadFirmware400JSONResponse(publicError("INVALID_PARAMS", err.Error())), nil
	}
	body, contentLength, headers, err := s.resolveOTAFile(depotName, channel, filePath)
	if err != nil {
		switch {
		case errors.Is(err, errInvalidPath):
			return serverpublic.DownloadFirmware400JSONResponse(publicError("INVALID_PARAMS", err.Error())), nil
		case errors.Is(err, errFirmwareNotFound), errors.Is(err, errDepotNotFound), errors.Is(err, errChannelNotFound):
			return serverpublic.DownloadFirmware404JSONResponse(publicError("FIRMWARE_FILE_NOT_FOUND", err.Error())), nil
		default:
			return downloadFirmware500JSONResponse(publicError("INTERNAL_ERROR", err.Error())), nil
		}
	}
	return serverpublic.DownloadFirmware200ApplicationoctetStreamResponse{
		Body:          body,
		Headers:       headers,
		ContentLength: contentLength,
	}, nil
}

func (s *Server) resolveCallerTarget(ctx context.Context) (string, Channel, error) {
	if s.ResolveGearTarget == nil {
		return "", "", fmt.Errorf("gear target resolver not configured")
	}
	publicKey := serverpublic.CallerPublicKey(ctx)
	if publicKey == "" {
		return "", "", fmt.Errorf("caller public key not configured")
	}
	depotName, channel, err := s.ResolveGearTarget(ctx, publicKey)
	if err != nil {
		return "", "", err
	}
	if depotName == "" || channel == "" {
		return "", "", fmt.Errorf("missing depot or channel")
	}
	return depotName, channel, nil
}

func (s *Server) resolveOTAFile(depotName string, channel Channel, relativePath string) (io.Reader, int64, serverpublic.DownloadFirmware200ResponseHeaders, error) {
	if err := validateRelativePath(relativePath); err != nil {
		return nil, 0, serverpublic.DownloadFirmware200ResponseHeaders{}, err
	}
	depot, err := s.scanDepot(depotName)
	if err != nil {
		return nil, 0, serverpublic.DownloadFirmware200ResponseHeaders{}, errFirmwareNotFound
	}
	release, ok := depotRelease(depot, channel)
	if !ok {
		return nil, 0, serverpublic.DownloadFirmware200ResponseHeaders{}, errFirmwareNotFound
	}
	var headers serverpublic.DownloadFirmware200ResponseHeaders
	found := false
	for _, file := range releaseFiles(release) {
		if file.Path != relativePath {
			continue
		}
		headers = serverpublic.DownloadFirmware200ResponseHeaders{
			XChecksumMD5:    file.Md5,
			XChecksumSHA256: file.Sha256,
		}
		found = true
		break
	}
	if !found {
		return nil, 0, serverpublic.DownloadFirmware200ResponseHeaders{}, fmt.Errorf("%w: %s", errFirmwareNotFound, relativePath)
	}

	fullPath := path.Join(s.channelPath(depotName, string(channel)), relativePath)
	file, err := s.store().Open(fullPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, 0, serverpublic.DownloadFirmware200ResponseHeaders{}, errFirmwareNotFound
		}
		return nil, 0, serverpublic.DownloadFirmware200ResponseHeaders{}, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, serverpublic.DownloadFirmware200ResponseHeaders{}, err
	}
	return file, info.Size(), headers, nil
}

func toPublicOTASummary(in gearservice.OTASummary) serverpublic.OTASummary {
	files := make([]serverpublic.DepotFile, 0, len(in.Files))
	for _, file := range in.Files {
		files = append(files, serverpublic.DepotFile{
			Md5:    file.Md5,
			Path:   file.Path,
			Sha256: file.Sha256,
		})
	}
	return serverpublic.OTASummary{
		Channel:        in.Channel,
		Depot:          in.Depot,
		Files:          files,
		FirmwareSemver: in.FirmwareSemver,
	}
}

func adminError(code, message string) adminservice.ErrorResponse {
	return adminservice.ErrorResponse{Error: adminservice.ErrorPayload{Code: code, Message: message}}
}

func gearError(code, message string) gearservice.ErrorResponse {
	return gearservice.ErrorResponse{Error: gearservice.ErrorPayload{Code: code, Message: message}}
}

func publicError(code, message string) serverpublic.ErrorResponse {
	return serverpublic.ErrorResponse{Error: serverpublic.ErrorPayload{Code: code, Message: message}}
}

type downloadFirmware500JSONResponse serverpublic.ErrorResponse

func (response downloadFirmware500JSONResponse) VisitDownloadFirmwareResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(500)
	return ctx.JSON(&response)
}
