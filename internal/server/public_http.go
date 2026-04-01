package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/giztoy/giztoy-go/pkg/gears"
)

var BuildCommit = "dev"

func (s *Server) publicHandler(publicKey string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/server-info", s.withCaller(publicKey, s.handleServerInfo))
	mux.HandleFunc("/info", s.withCaller(publicKey, s.handlePublicInfo))
	mux.HandleFunc("/registration", s.withCaller(publicKey, s.handleRegistration))
	mux.HandleFunc("/runtime", s.withCaller(publicKey, s.handleRuntime))
	mux.HandleFunc("/config", s.withCaller(publicKey, s.handleConfig))
	mux.HandleFunc("/register", s.withCaller(publicKey, s.handleRegister))
	mux.HandleFunc("/ota", s.withCaller(publicKey, s.handleOTA))
	mux.HandleFunc("/download/firmware/", s.withCaller(publicKey, s.handleFirmwareDownload))
	return mux
}

func (s *Server) withCaller(publicKey string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next(w, r.WithContext(withCallerPublicKey(r.Context(), publicKey)))
	}
}

func (s *Server) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"public_key":   s.keyPair.Public.String(),
		"server_time":  time.Now().UnixMilli(),
		"build_commit": BuildCommit,
	})
}

func (s *Server) handlePublicInfo(w http.ResponseWriter, r *http.Request) {
	publicKey := callerPublicKey(r.Context())
	switch r.Method {
	case http.MethodGet:
		gear, err := s.gears.Get(r.Context(), publicKey)
		if err != nil {
			writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, gear.Device)
	case http.MethodPut:
		var info gears.DeviceInfo
		if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_DEVICE_INFO", err.Error())
			return
		}
		gear, err := s.gears.PutInfo(r.Context(), publicKey, info)
		if err != nil {
			writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, gear.Device)
	default:
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "method not allowed")
	}
}

func (s *Server) handleRegistration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "method not allowed")
		return
	}
	gear, err := s.gears.Get(r.Context(), callerPublicKey(r.Context()))
	if err != nil {
		writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, gear.Registration())
}

func (s *Server) handleRuntime(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, s.peerRuntime(callerPublicKey(r.Context())))
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "method not allowed")
		return
	}
	gear, err := s.gears.Get(r.Context(), callerPublicKey(r.Context()))
	if err != nil {
		writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, gear.Configuration)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "method not allowed")
		return
	}
	var req gears.RegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
		return
	}
	req.PublicKey = callerPublicKey(r.Context())
	result, err := s.gears.Register(r.Context(), req)
	if err != nil {
		if err == gears.ErrGearAlreadyExists {
			writeError(w, http.StatusConflict, "GEAR_ALREADY_EXISTS", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleOTA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "method not allowed")
		return
	}
	gear, err := s.gears.Get(r.Context(), callerPublicKey(r.Context()))
	if err != nil {
		writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
		return
	}
	if gear.Device.Hardware.Depot == "" || gear.Configuration.Firmware.Channel == "" {
		writeError(w, http.StatusNotFound, "OTA_NOT_AVAILABLE", "missing depot or channel")
		return
	}
	ota, err := s.firmwareOTA.Resolve(gear.Device.Hardware.Depot, firmware.Channel(gear.Configuration.Firmware.Channel))
	if err != nil {
		writeError(w, http.StatusNotFound, "FIRMWARE_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ota)
}

func (s *Server) handleFirmwareDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", "method not allowed")
		return
	}
	gear, err := s.gears.Get(r.Context(), callerPublicKey(r.Context()))
	if err != nil {
		writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
		return
	}
	if gear.Device.Hardware.Depot == "" || gear.Configuration.Firmware.Channel == "" {
		writeError(w, http.StatusNotFound, "OTA_NOT_AVAILABLE", "missing depot or channel")
		return
	}
	path, err := decodeEscapedPath(escapedSubpath(r, "/download/firmware/"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
		return
	}
	fullPath, file, err := s.firmwareOTA.ResolveFile(gear.Device.Hardware.Depot, firmware.Channel(gear.Configuration.Firmware.Channel), path)
	if err != nil {
		writeError(w, http.StatusNotFound, "FIRMWARE_FILE_NOT_FOUND", err.Error())
		return
	}
	w.Header().Set("X-Checksum-SHA256", file.SHA256)
	w.Header().Set("X-Checksum-MD5", file.MD5)
	http.ServeFile(w, r, fullPath)
}
