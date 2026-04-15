package gear

import (
	"encoding/json"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
)

func toGearRegistrationList(items []gearservice.Gear) gearservice.RegistrationList {
	out := make([]gearservice.Registration, 0, len(items))
	for _, item := range items {
		out = append(out, toGearRegistration(item))
	}
	return gearservice.RegistrationList{Items: out}
}

func toGearRegistration(gear gearservice.Gear) gearservice.Registration {
	return gearservice.Registration{
		ApprovedAt:     gear.ApprovedAt,
		AutoRegistered: gear.AutoRegistered,
		CreatedAt:      gear.CreatedAt,
		PublicKey:      gear.PublicKey,
		Role:           gear.Role,
		Status:         gear.Status,
		UpdatedAt:      gear.UpdatedAt,
	}
}

func toPublicConfiguration(in gearservice.Configuration) (serverpublic.Configuration, error) {
	var out serverpublic.Configuration
	data, err := json.Marshal(in)
	if err != nil {
		return serverpublic.Configuration{}, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return serverpublic.Configuration{}, err
	}
	return out, nil
}

func toPublicDeviceInfo(in gearservice.DeviceInfo) (serverpublic.DeviceInfo, error) {
	var out serverpublic.DeviceInfo
	data, err := json.Marshal(in)
	if err != nil {
		return serverpublic.DeviceInfo{}, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return serverpublic.DeviceInfo{}, err
	}
	return out, nil
}

func toGearDeviceInfo(in serverpublic.DeviceInfo) (gearservice.DeviceInfo, error) {
	var out gearservice.DeviceInfo
	data, err := json.Marshal(in)
	if err != nil {
		return gearservice.DeviceInfo{}, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return gearservice.DeviceInfo{}, err
	}
	return out, nil
}

func toGearRegistrationRequest(in serverpublic.RegistrationRequest) (gearservice.RegistrationRequest, error) {
	var out gearservice.RegistrationRequest
	data, err := json.Marshal(in)
	if err != nil {
		return gearservice.RegistrationRequest{}, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return gearservice.RegistrationRequest{}, err
	}
	return out, nil
}

func toPublicRegistration(gear gearservice.Gear) serverpublic.Registration {
	return serverpublic.Registration{
		ApprovedAt:     gear.ApprovedAt,
		AutoRegistered: gear.AutoRegistered,
		CreatedAt:      gear.CreatedAt,
		PublicKey:      gear.PublicKey,
		Role:           serverpublic.GearRole(gear.Role),
		Status:         serverpublic.GearStatus(gear.Status),
		UpdatedAt:      gear.UpdatedAt,
	}
}

func toPublicRuntime(in gearservice.Runtime) serverpublic.Runtime {
	return serverpublic.Runtime{
		LastAddr:   in.LastAddr,
		LastSeenAt: in.LastSeenAt,
		Online:     in.Online,
	}
}

func toPublicGear(in gearservice.Gear) (serverpublic.Gear, error) {
	cfg, err := toPublicConfiguration(in.Configuration)
	if err != nil {
		return serverpublic.Gear{}, err
	}
	info, err := toPublicDeviceInfo(in.Device)
	if err != nil {
		return serverpublic.Gear{}, err
	}
	return serverpublic.Gear{
		ApprovedAt:     in.ApprovedAt,
		AutoRegistered: in.AutoRegistered,
		Configuration:  cfg,
		CreatedAt:      in.CreatedAt,
		Device:         info,
		PublicKey:      in.PublicKey,
		Role:           serverpublic.GearRole(in.Role),
		Status:         serverpublic.GearStatus(in.Status),
		UpdatedAt:      in.UpdatedAt,
	}, nil
}

func toPublicRegistrationResult(in gearservice.RegistrationResult) (serverpublic.RegistrationResult, error) {
	gear, err := toPublicGear(in.Gear)
	if err != nil {
		return serverpublic.RegistrationResult{}, err
	}
	return serverpublic.RegistrationResult{
		Gear:         gear,
		Registration: toPublicRegistration(in.Gear),
	}, nil
}
