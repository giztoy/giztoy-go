package gears

import (
	"context"
	"errors"
)

var ErrNoRefreshData = errors.New("gears: no refresh data")

type DeviceProvider interface {
	GetInfo(ctx context.Context, publicKey string) (RefreshInfo, error)
	GetIdentifiers(ctx context.Context, publicKey string) (RefreshIdentifiers, error)
	GetVersion(ctx context.Context, publicKey string) (RefreshVersion, error)
}

func ApplyRefresh(gear Gear, patch RefreshPatch) (Gear, []string, error) {
	updated := make([]string, 0, 8)
	if patch.Info == nil && patch.Identifiers == nil && patch.Version == nil {
		return gear, nil, ErrNoRefreshData
	}

	if patch.Info != nil {
		if patch.Info.Name != "" && gear.Device.Name != patch.Info.Name {
			gear.Device.Name = patch.Info.Name
			updated = append(updated, "device.name")
		}
		if patch.Info.Manufacturer != "" && gear.Device.Hardware.Manufacturer != patch.Info.Manufacturer {
			gear.Device.Hardware.Manufacturer = patch.Info.Manufacturer
			updated = append(updated, "device.hardware.manufacturer")
		}
		if patch.Info.Model != "" && gear.Device.Hardware.Model != patch.Info.Model {
			gear.Device.Hardware.Model = patch.Info.Model
			updated = append(updated, "device.hardware.model")
		}
		if patch.Info.HardwareRevision != "" && gear.Device.Hardware.HardwareRevision != patch.Info.HardwareRevision {
			gear.Device.Hardware.HardwareRevision = patch.Info.HardwareRevision
			updated = append(updated, "device.hardware.hardware_revision")
		}
	}
	if patch.Identifiers != nil {
		if patch.Identifiers.SN != "" && gear.Device.SN != patch.Identifiers.SN {
			gear.Device.SN = patch.Identifiers.SN
			updated = append(updated, "device.sn")
		}
		if len(patch.Identifiers.IMEIs) > 0 {
			gear.Device.Hardware.IMEIs = dedupeIMEIs(patch.Identifiers.IMEIs)
			updated = append(updated, "device.hardware.imeis")
		}
		if len(patch.Identifiers.Labels) > 0 {
			gear.Device.Hardware.Labels = dedupeLabels(patch.Identifiers.Labels)
			updated = append(updated, "device.hardware.labels")
		}
	}
	if patch.Version != nil {
		if patch.Version.Depot != "" && gear.Device.Hardware.Depot != patch.Version.Depot {
			gear.Device.Hardware.Depot = patch.Version.Depot
			updated = append(updated, "device.hardware.depot")
		}
		if patch.Version.FirmwareSemVer != "" && gear.Device.Hardware.FirmwareSemVer != patch.Version.FirmwareSemVer {
			gear.Device.Hardware.FirmwareSemVer = patch.Version.FirmwareSemVer
			updated = append(updated, "device.hardware.firmware_semver")
		}
	}
	return gear, updated, nil
}
