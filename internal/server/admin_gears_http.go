package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/giztoy/giztoy-go/api/admingen"
	"github.com/giztoy/giztoy-go/api/geargen"
	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/go-chi/chi/v5"
)

var _ geargen.ServerInterface = (*Server)(nil)

func (s *Server) adminHandler(publicKey string) http.Handler {
	r := chi.NewRouter()
	admingen.HandlerFromMux(s, r)
	geargen.HandlerFromMux(s, r)
	return s.withCaller(publicKey, s.requireAdmin(r.ServeHTTP))
}

func (s *Server) ListGears(w http.ResponseWriter, r *http.Request) {
	items, err := s.gears.List(r.Context(), gears.ListOptions{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	s.writeGearRegistrations(w, items)
}

func (s *Server) ResolveBySN(w http.ResponseWriter, r *http.Request, sn string) {
	gear, err := s.gears.ResolveBySN(r.Context(), sn)
	if err != nil {
		writeError(w, http.StatusNotFound, "GEAR_SN_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"public_key": gear.PublicKey})
}

func (s *Server) ResolveByIMEI(w http.ResponseWriter, r *http.Request, tac string, serial string) {
	gear, err := s.gears.ResolveByIMEI(r.Context(), tac, serial)
	if err != nil {
		writeError(w, http.StatusNotFound, "GEAR_IMEI_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"public_key": gear.PublicKey})
}

func (s *Server) ListByLabel(w http.ResponseWriter, r *http.Request, key string, value string) {
	items, err := s.gears.ListByLabel(r.Context(), key, value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	s.writeGearRegistrations(w, items)
}

func (s *Server) ListByCertification(w http.ResponseWriter, r *http.Request, pType geargen.GearCertificationType, authority geargen.GearCertificationAuthority, id string) {
	items, err := s.gears.ListByCertification(r.Context(), gears.GearCertificationType(pType), gears.GearCertificationAuthority(authority), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	s.writeGearRegistrations(w, items)
}

func (s *Server) ListByFirmware(w http.ResponseWriter, r *http.Request, depot string, channel geargen.GearFirmwareChannel) {
	items, err := s.gears.ListByFirmware(r.Context(), depot, gears.GearFirmwareChannel(channel))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	s.writeGearRegistrations(w, items)
}

func (s *Server) GetGear(w http.ResponseWriter, r *http.Request, publicKey geargen.PublicKey) {
	gear, err := s.gears.Get(r.Context(), publicKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, gear.Registration())
}

func (s *Server) DeleteGear(w http.ResponseWriter, r *http.Request, publicKey geargen.PublicKey) {
	gear, err := s.gears.Delete(r.Context(), publicKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, gear.Registration())
}

func (s *Server) GetGearInfo(w http.ResponseWriter, r *http.Request, publicKey geargen.PublicKey) {
	gear, err := s.gears.Get(r.Context(), publicKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, gear.Device)
}

func (s *Server) GetGearConfig(w http.ResponseWriter, r *http.Request, publicKey geargen.PublicKey) {
	gear, err := s.gears.Get(r.Context(), publicKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, gear.Configuration)
}

func (s *Server) PutGearConfig(w http.ResponseWriter, r *http.Request, publicKey geargen.PublicKey) {
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
}

func (s *Server) GetGearRuntime(w http.ResponseWriter, r *http.Request, publicKey geargen.PublicKey) {
	writeJSON(w, http.StatusOK, s.peerRuntime(publicKey))
}

func (s *Server) GetGearOTA(w http.ResponseWriter, r *http.Request, publicKey geargen.PublicKey) {
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
}

func (s *Server) ApproveGear(w http.ResponseWriter, r *http.Request, publicKey geargen.PublicKey) {
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
}

func (s *Server) BlockGear(w http.ResponseWriter, r *http.Request, publicKey geargen.PublicKey) {
	gear, err := s.gears.Block(r.Context(), publicKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, gear.Registration())
}

func (s *Server) RefreshGear(w http.ResponseWriter, r *http.Request, publicKey geargen.PublicKey) {
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
}

func (s *Server) writeGearRegistrations(w http.ResponseWriter, items []gears.Gear) {
	registrations := make([]gears.Registration, 0, len(items))
	for _, item := range items {
		registrations = append(registrations, item.Registration())
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": registrations})
}
