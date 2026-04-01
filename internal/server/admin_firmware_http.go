package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/giztoy/giztoy-go/pkg/firmware"
)

func (s *Server) handleAdminFirmwares(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "method not allowed")
		return
	}
	depots, err := s.firmwareScanner.Scan()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DIRECTORY_SCAN_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": depots})
}

func (s *Server) handleAdminFirmwareByPath(w http.ResponseWriter, r *http.Request) {
	path := escapedSubpath(r, "/firmwares/")
	switch {
	case strings.HasSuffix(path, ":rollback") && r.Method == http.MethodPut:
		depot, err := decodeEscapedPath(strings.TrimSuffix(path, ":rollback"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
			return
		}
		snapshot, err := s.firmwareSwitcher.Rollback(depot)
		if err != nil {
			writeFirmwareSwitchError(w, "rollback", err)
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	case strings.HasSuffix(path, ":release") && r.Method == http.MethodPut:
		depot, err := decodeEscapedPath(strings.TrimSuffix(path, ":release"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
			return
		}
		snapshot, err := s.firmwareSwitcher.Release(depot)
		if err != nil {
			writeFirmwareSwitchError(w, "release", err)
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	default:
		parts, err := decodeEscapedSegments(path, 2)
		if err == nil {
			s.handleAdminChannel(w, r, parts[0], firmware.Channel(parts[1]))
			return
		}
		depot, depotErr := decodeEscapedPath(path)
		if depotErr == nil {
			s.handleAdminDepot(w, r, depot)
			return
		}
		switch len(strings.Split(path, "/")) {
		case 1:
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", depotErr.Error())
		default:
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "unsupported route")
		}
	}
}

func writeFirmwareSwitchError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, firmware.ErrDepotNotFound):
		writeError(w, http.StatusNotFound, "DEPOT_NOT_FOUND", err.Error())
	case errors.Is(err, firmware.ErrChannelNotFound):
		if op == "rollback" {
			writeError(w, http.StatusConflict, "ROLLBACK_NOT_AVAILABLE", err.Error())
			return
		}
		writeError(w, http.StatusConflict, "RELEASE_NOT_READY", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}

func (s *Server) handleAdminDepot(w http.ResponseWriter, r *http.Request, depot string) {
	switch r.Method {
	case http.MethodGet:
		snapshot, err := s.firmwareScanner.ScanDepot(depot)
		if err != nil {
			writeError(w, http.StatusNotFound, "DEPOT_NOT_FOUND", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	case http.MethodPut:
		var info firmware.DepotInfo
		if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
			return
		}
		if err := s.firmwareUploader.PutInfo(depot, info); err != nil {
			writeError(w, http.StatusConflict, "INFO_FILES_MISMATCH", err.Error())
			return
		}
		snapshot, err := s.firmwareScanner.ScanDepot(depot)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "DIRECTORY_SCAN_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	default:
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "method not allowed")
	}
}

func (s *Server) handleAdminChannel(w http.ResponseWriter, r *http.Request, depot string, channel firmware.Channel) {
	switch r.Method {
	case http.MethodGet:
		snapshot, err := s.firmwareScanner.ScanDepot(depot)
		if err != nil {
			writeError(w, http.StatusNotFound, "DEPOT_NOT_FOUND", err.Error())
			return
		}
		release, ok := snapshot.Release(channel)
		if !ok {
			writeError(w, http.StatusNotFound, "CHANNEL_NOT_FOUND", "channel not found")
			return
		}
		writeJSON(w, http.StatusOK, release)
	case http.MethodPut:
		release, err := s.firmwareUploader.UploadTar(depot, channel, r.Body)
		if err != nil {
			writeError(w, http.StatusConflict, "MANIFEST_INVALID", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, release)
	default:
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "method not allowed")
	}
}
