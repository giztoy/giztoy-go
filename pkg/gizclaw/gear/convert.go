package gear

import (
	"encoding/json"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
)

func convertViaJSON[T any](in any) (T, error) {
	var out T
	data, err := json.Marshal(in)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

func toAdminRegistrationList(items []apitypes.Gear, hasNext bool, nextCursor *string) adminservice.RegistrationList {
	out := make([]apitypes.Registration, 0, len(items))
	for _, item := range items {
		out = append(out, toAdminRegistration(item))
	}
	return adminservice.RegistrationList{
		HasNext:    hasNext,
		Items:      out,
		NextCursor: nextCursor,
	}
}

func toAdminRegistration(gear apitypes.Gear) apitypes.Registration {
	return apitypes.Registration{
		ApprovedAt:     gear.ApprovedAt,
		AutoRegistered: gear.AutoRegistered,
		CreatedAt:      gear.CreatedAt,
		PublicKey:      gear.PublicKey,
		Role:           apitypes.GearRole(gear.Role),
		Status:         apitypes.GearStatus(gear.Status),
		UpdatedAt:      gear.UpdatedAt,
	}
}

func toAdminRuntime(in apitypes.Runtime) apitypes.Runtime {
	return apitypes.Runtime{
		LastAddr:   in.LastAddr,
		LastSeenAt: in.LastSeenAt,
		Online:     in.Online,
	}
}

func toAdminOTASummary(in apitypes.OTASummary) (apitypes.OTASummary, error) {
	return convertViaJSON[apitypes.OTASummary](in)
}

func toGearRegistration(gear apitypes.Gear) apitypes.Registration {
	return apitypes.Registration{
		ApprovedAt:     gear.ApprovedAt,
		AutoRegistered: gear.AutoRegistered,
		CreatedAt:      gear.CreatedAt,
		PublicKey:      gear.PublicKey,
		Role:           apitypes.GearRole(gear.Role),
		Status:         apitypes.GearStatus(gear.Status),
		UpdatedAt:      gear.UpdatedAt,
	}
}

func toGearConfiguration(in apitypes.Configuration) (apitypes.Configuration, error) {
	return convertViaJSON[apitypes.Configuration](in)
}

func toPublicConfiguration(in apitypes.Configuration) (apitypes.Configuration, error) {
	return convertViaJSON[apitypes.Configuration](in)
}

func toGearDeviceInfo(in apitypes.DeviceInfo) (apitypes.DeviceInfo, error) {
	return convertViaJSON[apitypes.DeviceInfo](in)
}

func toAdminDeviceInfo(in apitypes.DeviceInfo) (apitypes.DeviceInfo, error) {
	return convertViaJSON[apitypes.DeviceInfo](in)
}

func toPublicDeviceInfo(in apitypes.DeviceInfo) (apitypes.DeviceInfo, error) {
	return convertViaJSON[apitypes.DeviceInfo](in)
}

func toPublicRegistration(gear apitypes.Gear) apitypes.Registration {
	return apitypes.Registration{
		ApprovedAt:     gear.ApprovedAt,
		AutoRegistered: gear.AutoRegistered,
		CreatedAt:      gear.CreatedAt,
		PublicKey:      gear.PublicKey,
		Role:           apitypes.GearRole(gear.Role),
		Status:         apitypes.GearStatus(gear.Status),
		UpdatedAt:      gear.UpdatedAt,
	}
}

func toPublicGear(in apitypes.Gear) (apitypes.Gear, error) {
	cfg, err := toPublicConfiguration(in.Configuration)
	if err != nil {
		return apitypes.Gear{}, err
	}
	info, err := toPublicDeviceInfo(in.Device)
	if err != nil {
		return apitypes.Gear{}, err
	}
	return apitypes.Gear{
		ApprovedAt:     in.ApprovedAt,
		AutoRegistered: in.AutoRegistered,
		Configuration:  cfg,
		CreatedAt:      in.CreatedAt,
		Device:         info,
		PublicKey:      in.PublicKey,
		Role:           apitypes.GearRole(in.Role),
		Status:         apitypes.GearStatus(in.Status),
		UpdatedAt:      in.UpdatedAt,
	}, nil
}

func toPublicRegistrationResult(gear apitypes.Gear) (serverpublic.RegistrationResult, error) {
	publicGear, err := toPublicGear(gear)
	if err != nil {
		return serverpublic.RegistrationResult{}, err
	}
	return serverpublic.RegistrationResult{
		Gear:         publicGear,
		Registration: toPublicRegistration(gear),
	}, nil
}
