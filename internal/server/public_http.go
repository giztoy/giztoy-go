package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/giztoy/giztoy-go/api/publicgen"
	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/go-chi/chi/v5"
)

var BuildCommit = "dev"

var _ publicgen.ServerInterface = (*Server)(nil)

func (s *Server) publicHandler(publicKey string) http.Handler {
	r := chi.NewRouter()
	publicgen.HandlerFromMux(s, r)
	return s.withCaller(publicKey, r.ServeHTTP)
}

func (s *Server) withCaller(publicKey string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next(w, r.WithContext(withCallerPublicKey(r.Context(), publicKey)))
	}
}

func (s *Server) GetServerInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"public_key":   s.keyPair.Public.String(),
		"server_time":  time.Now().UnixMilli(),
		"build_commit": BuildCommit,
	})
}

func (s *Server) GetInfo(w http.ResponseWriter, r *http.Request) {
	gear, err := s.gears.Get(r.Context(), callerPublicKey(r.Context()))
	if err != nil {
		writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, gear.Device)
}

func (s *Server) PutInfo(w http.ResponseWriter, r *http.Request) {
	publicKey := callerPublicKey(r.Context())
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
}

func (s *Server) GetRegistration(w http.ResponseWriter, r *http.Request) {
	gear, err := s.gears.Get(r.Context(), callerPublicKey(r.Context()))
	if err != nil {
		writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, gear.Registration())
}

func (s *Server) GetRuntime(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.peerRuntime(callerPublicKey(r.Context())))
}

func (s *Server) GetConfig(w http.ResponseWriter, r *http.Request) {
	gear, err := s.gears.Get(r.Context(), callerPublicKey(r.Context()))
	if err != nil {
		writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, gear.Configuration)
}

func (s *Server) RegisterGear(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) GetOTA(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) DownloadFirmware(w http.ResponseWriter, r *http.Request, path string) {
	gear, err := s.gears.Get(r.Context(), callerPublicKey(r.Context()))
	if err != nil {
		writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
		return
	}
	if gear.Device.Hardware.Depot == "" || gear.Configuration.Firmware.Channel == "" {
		writeError(w, http.StatusNotFound, "OTA_NOT_AVAILABLE", "missing depot or channel")
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
