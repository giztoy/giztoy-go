package firmware

import "errors"

type Channel string

const (
	ChannelRollback Channel = "rollback"
	ChannelStable   Channel = "stable"
	ChannelBeta     Channel = "beta"
	ChannelTesting  Channel = "testing"
)

var orderedChannels = []Channel{ChannelStable, ChannelBeta, ChannelTesting}

var (
	ErrDepotNotFound         = errors.New("firmware: depot not found")
	ErrChannelNotFound       = errors.New("firmware: channel not found")
	ErrFirmwareNotFound      = errors.New("firmware: firmware not found")
	ErrInvalidPath           = errors.New("firmware: invalid path")
	ErrVersionOrderViolation = errors.New("firmware: version order violation")
)

type Depot struct {
	Name     string       `json:"name" msgpack:"name"`
	Info     DepotInfo    `json:"info" msgpack:"info"`
	Rollback DepotRelease `json:"rollback" msgpack:"rollback"`
	Stable   DepotRelease `json:"stable" msgpack:"stable"`
	Beta     DepotRelease `json:"beta" msgpack:"beta"`
	Testing  DepotRelease `json:"testing" msgpack:"testing"`
}

type DepotInfo struct {
	Files []DepotInfoFile `json:"files,omitempty" msgpack:"files,omitempty"`
}

type DepotInfoFile struct {
	Path string `json:"path" msgpack:"path"`
}

type DepotRelease struct {
	FirmwareSemVer string      `json:"firmware_semver" msgpack:"firmware_semver"`
	Channel        string      `json:"channel,omitempty" msgpack:"channel,omitempty"`
	Files          []DepotFile `json:"files,omitempty" msgpack:"files,omitempty"`
}

type DepotFile struct {
	Path   string `json:"path" msgpack:"path"`
	SHA256 string `json:"sha256" msgpack:"sha256"`
	MD5    string `json:"md5" msgpack:"md5"`
}

type OTASummary struct {
	Depot          string      `json:"depot"`
	Channel        string      `json:"channel"`
	FirmwareSemVer string      `json:"firmware_semver"`
	Files          []DepotFile `json:"files"`
}

func IsValidChannel(channel Channel) bool {
	switch channel {
	case ChannelRollback, ChannelStable, ChannelBeta, ChannelTesting:
		return true
	default:
		return false
	}
}

func (d Depot) Release(channel Channel) (DepotRelease, bool) {
	switch channel {
	case ChannelRollback:
		return d.Rollback, d.Rollback.FirmwareSemVer != ""
	case ChannelStable:
		return d.Stable, d.Stable.FirmwareSemVer != ""
	case ChannelBeta:
		return d.Beta, d.Beta.FirmwareSemVer != ""
	case ChannelTesting:
		return d.Testing, d.Testing.FirmwareSemVer != ""
	default:
		return DepotRelease{}, false
	}
}

func (d *Depot) SetRelease(channel Channel, release DepotRelease) {
	switch channel {
	case ChannelRollback:
		d.Rollback = release
	case ChannelStable:
		d.Stable = release
	case ChannelBeta:
		d.Beta = release
	case ChannelTesting:
		d.Testing = release
	}
}
