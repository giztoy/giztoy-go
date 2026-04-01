package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/giztoy/giztoy-go/pkg/gears"
)

func (s *Server) adminHandler(publicKey string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/gears", s.withCaller(publicKey, s.requireAdmin(s.handleAdminGears)))
	mux.HandleFunc("/gears/", s.withCaller(publicKey, s.requireAdmin(s.handleAdminGearByPath)))
	mux.HandleFunc("/firmwares", s.withCaller(publicKey, s.requireAdmin(s.handleAdminFirmwares)))
	mux.HandleFunc("/firmwares/", s.withCaller(publicKey, s.requireAdmin(s.handleAdminFirmwareByPath)))
	return mux
}

func (s *Server) handleAdminGears(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "method not allowed")
		return
	}
	items, err := s.gears.List(r.Context(), gears.ListOptions{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	s.writeGearRegistrations(w, items)
}

func (s *Server) handleAdminGearByPath(w http.ResponseWriter, r *http.Request) {
	path := escapedSubpath(r, "/gears/")
	switch {
	case strings.HasPrefix(path, "sn/") && r.Method == http.MethodGet:
		sn, err := decodeEscapedPath(strings.TrimPrefix(path, "sn/"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
			return
		}
		gear, err := s.gears.ResolveBySN(r.Context(), sn)
		if err != nil {
			writeError(w, http.StatusNotFound, "GEAR_SN_NOT_FOUND", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"public_key": gear.PublicKey})
	case strings.HasPrefix(path, "imei/") && r.Method == http.MethodGet:
		parts, err := decodeEscapedSegments(strings.TrimPrefix(path, "imei/"), 2)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "want /gears/imei/{tac}/{serial}")
			return
		}
		gear, err := s.gears.ResolveByIMEI(r.Context(), parts[0], parts[1])
		if err != nil {
			writeError(w, http.StatusNotFound, "GEAR_IMEI_NOT_FOUND", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"public_key": gear.PublicKey})
	case strings.HasPrefix(path, "label/") && r.Method == http.MethodGet:
		parts, err := decodeEscapedSegments(strings.TrimPrefix(path, "label/"), 2)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "want /gears/label/{key}/{value}")
			return
		}
		items, err := s.gears.ListByLabel(r.Context(), parts[0], parts[1])
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		s.writeGearRegistrations(w, items)
	case strings.HasPrefix(path, "certification/") && r.Method == http.MethodGet:
		parts, err := decodeEscapedSegments(strings.TrimPrefix(path, "certification/"), 3)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "want /gears/certification/{type}/{authority}/{id}")
			return
		}
		items, err := s.gears.ListByCertification(r.Context(), gears.GearCertificationType(parts[0]), gears.GearCertificationAuthority(parts[1]), parts[2])
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		s.writeGearRegistrations(w, items)
	case strings.HasPrefix(path, "firmware/") && r.Method == http.MethodGet:
		parts, err := decodeEscapedSegments(strings.TrimPrefix(path, "firmware/"), 2)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "want /gears/firmware/{depot}/{channel}")
			return
		}
		items, err := s.gears.ListByFirmware(r.Context(), parts[0], gears.GearFirmwareChannel(parts[1]))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		s.writeGearRegistrations(w, items)
	default:
		s.handleAdminGearResource(w, r, path)
	}
}

func (s *Server) writeGearRegistrations(w http.ResponseWriter, items []gears.Gear) {
	registrations := make([]gears.Registration, 0, len(items))
	for _, item := range items {
		registrations = append(registrations, item.Registration())
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": registrations})
}

func (s *Server) handleAdminGearResource(w http.ResponseWriter, r *http.Request, path string) {
	switch {
	case strings.HasSuffix(path, ":approve") && r.Method == http.MethodPost:
		publicKey, err := decodeEscapedPath(strings.TrimSuffix(path, ":approve"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
			return
		}
		var body struct {
			Role gears.GearRole `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ROLE", err.Error())
			return
		}
		gear, err := s.gears.Approve(r.Context(), publicKey, body.Role)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ROLE", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, gear.Registration())
	case strings.HasSuffix(path, ":block") && r.Method == http.MethodPost:
		publicKey, err := decodeEscapedPath(strings.TrimSuffix(path, ":block"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
			return
		}
		gear, err := s.gears.Block(r.Context(), publicKey)
		if err != nil {
			writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, gear.Registration())
	case strings.HasSuffix(path, ":refresh") && r.Method == http.MethodPost:
		publicKey, err := decodeEscapedPath(strings.TrimSuffix(path, ":refresh"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
			return
		}
		result, err := s.refreshGearFromDevice(r.Context(), publicKey)
		if err != nil {
			switch {
			case errors.Is(err, ErrDeviceOffline):
				writeError(w, http.StatusConflict, "DEVICE_OFFLINE", err.Error())
			case errors.Is(err, gears.ErrGearNotFound):
				writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
			default:
				writeError(w, http.StatusBadGateway, "DEVICE_REFRESH_FAILED", err.Error())
			}
			return
		}
		writeJSON(w, http.StatusOK, result)
	case strings.HasSuffix(path, "/info") && r.Method == http.MethodGet:
		publicKey, err := decodeEscapedPath(strings.TrimSuffix(path, "/info"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
			return
		}
		gear, err := s.gears.Get(r.Context(), publicKey)
		if err != nil {
			writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, gear.Device)
	case strings.HasSuffix(path, "/config") && r.Method == http.MethodGet:
		publicKey, err := decodeEscapedPath(strings.TrimSuffix(path, "/config"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
			return
		}
		gear, err := s.gears.Get(r.Context(), publicKey)
		if err != nil {
			writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, gear.Configuration)
	case strings.HasSuffix(path, "/config") && r.Method == http.MethodPut:
		publicKey, err := decodeEscapedPath(strings.TrimSuffix(path, "/config"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
			return
		}
		var cfg gears.Configuration
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
			return
		}
		gear, err := s.gears.PutConfig(r.Context(), publicKey, cfg)
		if err != nil {
			if errors.Is(err, gears.ErrGearNotFound) {
				writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, gear.Configuration)
	case strings.HasSuffix(path, "/runtime") && r.Method == http.MethodGet:
		publicKey, err := decodeEscapedPath(strings.TrimSuffix(path, "/runtime"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, s.peerRuntime(publicKey))
	case strings.HasSuffix(path, "/ota") && r.Method == http.MethodGet:
		publicKey, err := decodeEscapedPath(strings.TrimSuffix(path, "/ota"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
			return
		}
		gear, err := s.gears.Get(r.Context(), publicKey)
		if err != nil {
			writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
			return
		}
		ota, err := s.firmwareOTA.Resolve(gear.Device.Hardware.Depot, firmware.Channel(gear.Configuration.Firmware.Channel))
		if err != nil {
			writeError(w, http.StatusNotFound, "FIRMWARE_NOT_FOUND", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ota)
	case r.Method == http.MethodDelete:
		publicKey, err := decodeEscapedPath(path)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
			return
		}
		gear, err := s.gears.Delete(r.Context(), publicKey)
		if err != nil {
			writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, gear.Registration())
	case r.Method == http.MethodGet:
		publicKey, err := decodeEscapedPath(path)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
			return
		}
		gear, err := s.gears.Get(r.Context(), publicKey)
		if err != nil {
			writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, gear.Registration())
	default:
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "unsupported route")
	}
}
