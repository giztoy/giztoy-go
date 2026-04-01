package server

import (
	"context"
	"net/http"

	"github.com/giztoy/giztoy-go/pkg/gears"
)

type contextKey string

const callerPublicKeyKey contextKey = "caller_public_key"

func withCallerPublicKey(ctx context.Context, publicKey string) context.Context {
	return context.WithValue(ctx, callerPublicKeyKey, publicKey)
}

func callerPublicKey(ctx context.Context) string {
	value, _ := ctx.Value(callerPublicKeyKey).(string)
	return value
}

func (s *Server) authorize(publicKey string, kind gears.ServiceKind) (gears.Gear, bool, error) {
	gear, err := s.gears.Get(context.Background(), publicKey)
	if err != nil {
		if kind == gears.ServiceKindPublicDevice {
			return gears.Gear{}, false, nil
		}
		return gears.Gear{}, false, err
	}
	return gear, gears.CanAccess(gear.Role, gear.Status, kind), nil
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		publicKey := callerPublicKey(r.Context())
		gear, allowed, err := s.authorize(publicKey, gears.ServiceKindAdmin)
		if err != nil {
			writeError(w, http.StatusNotFound, "GEAR_NOT_FOUND", err.Error())
			return
		}
		if gear.PublicKey == "" || gear.Role != gears.GearRoleAdmin {
			writeError(w, http.StatusForbidden, "PERMISSION_DENIED", "admin role required")
			return
		}
		if gear.Status != gears.GearStatusActive || !allowed {
			writeError(w, http.StatusForbidden, "CALLER_NOT_ACTIVE", "caller must be active")
			return
		}
		next(w, r)
	}
}
