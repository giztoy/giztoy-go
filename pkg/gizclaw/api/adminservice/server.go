package adminservice

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/giztoy/giztoy-go/pkg/firmware"
)

type Server struct {
	FirmwareScanner  *firmware.Scanner
	FirmwareUploader *firmware.Uploader
	FirmwareSwitcher *firmware.Switcher
}

var _ StrictServerInterface = (*Server)(nil)

func adminError(code, message string) ErrorResponse {
	return ErrorResponse{
		Error: ErrorPayload{
			Code:    code,
			Message: message,
		},
	}
}

func (s *Server) ListDepots(_ context.Context, _ ListDepotsRequestObject) (ListDepotsResponseObject, error) {
	depots, err := s.FirmwareScanner.Scan()
	if err != nil {
		return ListDepots500JSONResponse(adminError("DIRECTORY_SCAN_FAILED", err.Error())), nil
	}
	items := make([]Depot, 0, len(depots))
	for _, depot := range depots {
		items = append(items, toAdminDepot(depot))
	}
	return ListDepots200JSONResponse(DepotList{Items: items}), nil
}

func (s *Server) GetDepot(_ context.Context, request GetDepotRequestObject) (GetDepotResponseObject, error) {
	depotName, err := adminPathUnescape(request.Depot)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	depot, err := s.FirmwareScanner.ScanDepot(depotName)
	if err != nil {
		return GetDepot404JSONResponse(adminError("DEPOT_NOT_FOUND", err.Error())), nil
	}
	return GetDepot200JSONResponse(toAdminDepot(depot)), nil
}

func (s *Server) PutDepotInfo(_ context.Context, request PutDepotInfoRequestObject) (PutDepotInfoResponseObject, error) {
	if request.Body == nil {
		return PutDepotInfo400JSONResponse(adminError("INVALID_JSON", "request body required")), nil
	}
	depotName, err := adminPathUnescape(request.Depot)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	info, err := reencode[firmware.DepotInfo](*request.Body)
	if err != nil {
		return PutDepotInfo400JSONResponse(adminError("INVALID_JSON", err.Error())), nil
	}
	if err := s.FirmwareUploader.PutInfo(depotName, info); err != nil {
		return PutDepotInfo409JSONResponse(adminError("INFO_FILES_MISMATCH", err.Error())), nil
	}
	depot, err := s.FirmwareScanner.ScanDepot(depotName)
	if err != nil {
		return PutDepotInfo500JSONResponse(adminError("DIRECTORY_SCAN_FAILED", err.Error())), nil
	}
	return PutDepotInfo200JSONResponse(toAdminDepot(depot)), nil
}

func (s *Server) GetChannel(_ context.Context, request GetChannelRequestObject) (GetChannelResponseObject, error) {
	depotName, err := adminPathUnescape(request.Depot)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	depot, err := s.FirmwareScanner.ScanDepot(depotName)
	if err != nil {
		return GetChannel404JSONResponse(adminError("DEPOT_NOT_FOUND", err.Error())), nil
	}
	release, ok := depot.Release(firmware.Channel(request.Channel))
	if !ok {
		return GetChannel404JSONResponse(adminError("CHANNEL_NOT_FOUND", "channel not found")), nil
	}
	return GetChannel200JSONResponse(toAdminDepotRelease(release)), nil
}

func (s *Server) PutChannel(_ context.Context, request PutChannelRequestObject) (PutChannelResponseObject, error) {
	depotName, err := adminPathUnescape(request.Depot)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	release, err := s.FirmwareUploader.UploadTar(depotName, firmware.Channel(request.Channel), request.Body)
	if err != nil {
		return PutChannel409JSONResponse(adminError("MANIFEST_INVALID", err.Error())), nil
	}
	return PutChannel200JSONResponse(toAdminDepotRelease(release)), nil
}

func writeAdminSwitchErrorRelease(err error) ReleaseDepotResponseObject {
	switch {
	case errors.Is(err, firmware.ErrDepotNotFound):
		return ReleaseDepot404JSONResponse(adminError("DEPOT_NOT_FOUND", err.Error()))
	case errors.Is(err, firmware.ErrChannelNotFound):
		return ReleaseDepot409JSONResponse(adminError("RELEASE_NOT_READY", err.Error()))
	default:
		return ReleaseDepot500JSONResponse(adminError("INTERNAL_ERROR", err.Error()))
	}
}

func writeAdminSwitchErrorRollback(err error) RollbackDepotResponseObject {
	switch {
	case errors.Is(err, firmware.ErrDepotNotFound):
		return RollbackDepot404JSONResponse(adminError("DEPOT_NOT_FOUND", err.Error()))
	case errors.Is(err, firmware.ErrChannelNotFound):
		return RollbackDepot409JSONResponse(adminError("ROLLBACK_NOT_AVAILABLE", err.Error()))
	default:
		return RollbackDepot500JSONResponse(adminError("INTERNAL_ERROR", err.Error()))
	}
}

func (s *Server) ReleaseDepot(_ context.Context, request ReleaseDepotRequestObject) (ReleaseDepotResponseObject, error) {
	depotName, err := adminPathUnescape(request.Depot)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	depot, err := s.FirmwareSwitcher.Release(depotName)
	if err != nil {
		return writeAdminSwitchErrorRelease(err), nil
	}
	return ReleaseDepot200JSONResponse(toAdminDepot(depot)), nil
}

func (s *Server) RollbackDepot(_ context.Context, request RollbackDepotRequestObject) (RollbackDepotResponseObject, error) {
	depotName, err := adminPathUnescape(request.Depot)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	depot, err := s.FirmwareSwitcher.Rollback(depotName)
	if err != nil {
		return writeAdminSwitchErrorRollback(err), nil
	}
	return RollbackDepot200JSONResponse(toAdminDepot(depot)), nil
}

func adminPathUnescape(value string) (string, error) {
	return url.PathUnescape(value)
}
