package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/giztoy/giztoy-go/api/admingen"
	"github.com/giztoy/giztoy-go/pkg/firmware"
)

var _ admingen.ServerInterface = (*Server)(nil)

func (s *Server) ListDepots(w http.ResponseWriter, r *http.Request) {
	depots, err := s.firmwareScanner.Scan()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DIRECTORY_SCAN_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": depots})
}

func (s *Server) GetDepot(w http.ResponseWriter, r *http.Request, depot admingen.DepotName) {
	snapshot, err := s.firmwareScanner.ScanDepot(depot)
	if err != nil {
		writeError(w, http.StatusNotFound, "DEPOT_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) PutDepotInfo(w http.ResponseWriter, r *http.Request, depot admingen.DepotName) {
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
}

func (s *Server) GetChannel(w http.ResponseWriter, r *http.Request, depot admingen.DepotName, channel admingen.Channel) {
	snapshot, err := s.firmwareScanner.ScanDepot(depot)
	if err != nil {
		writeError(w, http.StatusNotFound, "DEPOT_NOT_FOUND", err.Error())
		return
	}
	release, ok := snapshot.Release(firmware.Channel(channel))
	if !ok {
		writeError(w, http.StatusNotFound, "CHANNEL_NOT_FOUND", "channel not found")
		return
	}
	writeJSON(w, http.StatusOK, release)
}

func (s *Server) PutChannel(w http.ResponseWriter, r *http.Request, depot admingen.DepotName, channel admingen.Channel) {
	release, err := s.firmwareUploader.UploadTar(depot, firmware.Channel(channel), r.Body)
	if err != nil {
		writeError(w, http.StatusConflict, "MANIFEST_INVALID", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, release)
}

func (s *Server) ReleaseDepot(w http.ResponseWriter, r *http.Request, depot admingen.DepotName) {
	snapshot, err := s.firmwareSwitcher.Release(depot)
	if err != nil {
		writeFirmwareSwitchError(w, "release", err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) RollbackDepot(w http.ResponseWriter, r *http.Request, depot admingen.DepotName) {
	snapshot, err := s.firmwareSwitcher.Rollback(depot)
	if err != nil {
		writeFirmwareSwitchError(w, "rollback", err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
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
