package gearservice

import (
	"encoding/json"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"sort"
	"time"
)

func reencode[T any](v any) (T, error) {
	var out T
	data, err := json.Marshal(v)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	out := v
	return &out
}

func boolPtr(v bool) *bool {
	if !v {
		return nil
	}
	out := v
	return &out
}

func millisPtr(ms int64) *time.Time {
	if ms == 0 {
		return nil
	}
	t := time.UnixMilli(ms).UTC()
	return &t
}

func millisTime(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

func toGearRegistration(in gears.Registration) Registration {
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

func toGearRegistrationList(items []gears.Gear) RegistrationList {
	out := make([]Registration, 0, len(items))
	for _, item := range items {
		out = append(out, toGearRegistration(item.Registration()))
	}
	return RegistrationList{Items: out}
}

func toGearPublicKeyResponse(publicKey string) PublicKeyResponse {
	return PublicKeyResponse{PublicKey: publicKey}
}

func toGearRuntime(in gears.Runtime) Runtime {
	return Runtime{
		LastAddr:   stringPtr(in.LastAddr),
		LastSeenAt: millisTime(in.LastSeenAt),
		Online:     in.Online,
	}
}

func toGearRefreshErrors(errs map[string]string) *[]string {
	if len(errs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(errs))
	for key := range errs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+": "+errs[key])
	}
	return &out
}

func toGearRefreshResult(in gears.RefreshResult) (RefreshResult, error) {
	gear, err := toGearGear(in.Gear)
	if err != nil {
		return RefreshResult{}, err
	}
	var updated *[]string
	if len(in.UpdatedFields) > 0 {
		fields := append([]string(nil), in.UpdatedFields...)
		updated = &fields
	}
	return RefreshResult{
		Errors:        toGearRefreshErrors(in.Errors),
		Gear:          gear,
		UpdatedFields: updated,
	}, nil
}

func toGearGear(in gears.Gear) (Gear, error) {
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
