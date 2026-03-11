package gears

import "strings"

type GearRole string

const (
	GearRoleUnspecified GearRole = "unspecified"
	GearRoleAdmin       GearRole = "admin"
	GearRolePeer        GearRole = "peer"
	GearRoleDevice      GearRole = "device"
)

type GearStatus string

const (
	GearStatusUnspecified GearStatus = "unspecified"
	GearStatusActive      GearStatus = "active"
	GearStatusBlocked     GearStatus = "blocked"
)

type GearFirmwareChannel string

const (
	GearFirmwareChannelRollback GearFirmwareChannel = "rollback"
	GearFirmwareChannelStable   GearFirmwareChannel = "stable"
	GearFirmwareChannelBeta     GearFirmwareChannel = "beta"
	GearFirmwareChannelTesting  GearFirmwareChannel = "testing"
)

type GearCertificationType string

const (
	GearCertificationTypeLicense       GearCertificationType = "license"
	GearCertificationTypeCertification GearCertificationType = "certification"
)

type GearCertificationAuthority string

const (
	GearCertificationAuthorityUnknown  GearCertificationAuthority = "unknown"
	GearCertificationAuthorityCCC      GearCertificationAuthority = "ccc"
	GearCertificationAuthorityCE       GearCertificationAuthority = "ce"
	GearCertificationAuthorityFCC      GearCertificationAuthority = "fcc"
	GearCertificationAuthorityMIIT     GearCertificationAuthority = "miit"
	GearCertificationAuthoritySRRC     GearCertificationAuthority = "srrc"
	GearCertificationAuthorityRoHS     GearCertificationAuthority = "rohs"
	GearCertificationAuthorityInternal GearCertificationAuthority = "internal"
)

type Gear struct {
	PublicKey      string        `json:"public_key" msgpack:"public_key"`
	Role           GearRole      `json:"role" msgpack:"role"`
	Status         GearStatus    `json:"status" msgpack:"status"`
	Device         DeviceInfo    `json:"device" msgpack:"device"`
	Configuration  Configuration `json:"configuration" msgpack:"configuration"`
	AutoRegistered bool          `json:"auto_registered,omitempty" msgpack:"auto_registered,omitempty"`
	CreatedAt      int64         `json:"created_at" msgpack:"created_at"`
	UpdatedAt      int64         `json:"updated_at" msgpack:"updated_at"`
	ApprovedAt     int64         `json:"approved_at,omitempty" msgpack:"approved_at,omitempty"`
}

type DeviceInfo struct {
	Name     string       `json:"name,omitempty" msgpack:"name,omitempty"`
	SN       string       `json:"sn,omitempty" msgpack:"sn,omitempty"`
	Hardware HardwareInfo `json:"hardware" msgpack:"hardware"`
}

type HardwareInfo struct {
	Manufacturer     string      `json:"manufacturer,omitempty" msgpack:"manufacturer,omitempty"`
	Model            string      `json:"model,omitempty" msgpack:"model,omitempty"`
	HardwareRevision string      `json:"hardware_revision,omitempty" msgpack:"hardware_revision,omitempty"`
	Depot            string      `json:"depot,omitempty" msgpack:"depot,omitempty"`
	FirmwareSemVer   string      `json:"firmware_semver,omitempty" msgpack:"firmware_semver,omitempty"`
	IMEIs            []GearIMEI  `json:"imeis,omitempty" msgpack:"imeis,omitempty"`
	Labels           []GearLabel `json:"labels,omitempty" msgpack:"labels,omitempty"`
}

type Configuration struct {
	Certifications []GearCertification `json:"certifications,omitempty" msgpack:"certifications,omitempty"`
	Firmware       FirmwareConfig      `json:"firmware" msgpack:"firmware"`
}

type FirmwareConfig struct {
	Channel GearFirmwareChannel `json:"channel,omitempty" msgpack:"channel,omitempty"`
}

type GearIMEI struct {
	Name   string `json:"name,omitempty" msgpack:"name,omitempty"`
	TAC    string `json:"tac" msgpack:"tac"`
	Serial string `json:"serial" msgpack:"serial"`
}

type GearLabel struct {
	Key   string `json:"key" msgpack:"key"`
	Value string `json:"value" msgpack:"value"`
}

type GearCertification struct {
	Type          GearCertificationType      `json:"type" msgpack:"type"`
	Authority     GearCertificationAuthority `json:"authority" msgpack:"authority"`
	ID            string                     `json:"id" msgpack:"id"`
	AuthorityName string                     `json:"authority_name,omitempty" msgpack:"authority_name,omitempty"`
}

type Runtime struct {
	Online     bool   `json:"online" msgpack:"online"`
	LastSeenAt int64  `json:"last_seen_at" msgpack:"last_seen_at"`
	LastAddr   string `json:"last_addr,omitempty" msgpack:"last_addr,omitempty"`
}

type Registration struct {
	PublicKey      string     `json:"public_key"`
	Role           GearRole   `json:"role"`
	Status         GearStatus `json:"status"`
	AutoRegistered bool       `json:"auto_registered,omitempty"`
	CreatedAt      int64      `json:"created_at"`
	UpdatedAt      int64      `json:"updated_at"`
	ApprovedAt     int64      `json:"approved_at,omitempty"`
}

type RegistrationToken struct {
	Role GearRole `json:"role" yaml:"role"`
}

type RegistrationRequest struct {
	PublicKey         string     `json:"public_key"`
	Device            DeviceInfo `json:"device"`
	RegistrationToken string     `json:"registration_token,omitempty"`
}

type RegistrationResult struct {
	Gear       Gear         `json:"gear"`
	Registered Registration `json:"registration"`
}

type ListOptions struct {
	Limit int
}

type RefreshPatch struct {
	Info        *RefreshInfo
	Identifiers *RefreshIdentifiers
	Version     *RefreshVersion
}

type RefreshInfo struct {
	Name             string `json:"name,omitempty"`
	Manufacturer     string `json:"manufacturer,omitempty"`
	Model            string `json:"model,omitempty"`
	HardwareRevision string `json:"hardware_revision,omitempty"`
}

type RefreshIdentifiers struct {
	SN     string      `json:"sn,omitempty"`
	IMEIs  []GearIMEI  `json:"imeis,omitempty"`
	Labels []GearLabel `json:"labels,omitempty"`
}

type RefreshVersion struct {
	Depot          string `json:"depot,omitempty"`
	FirmwareSemVer string `json:"firmware_semver,omitempty"`
}

type RefreshResult struct {
	Gear          Gear              `json:"gear"`
	UpdatedFields []string          `json:"updated_fields,omitempty"`
	Errors        map[string]string `json:"errors,omitempty"`
}

func (g Gear) Registration() Registration {
	return Registration{
		PublicKey:      g.PublicKey,
		Role:           g.Role,
		Status:         g.Status,
		AutoRegistered: g.AutoRegistered,
		CreatedAt:      g.CreatedAt,
		UpdatedAt:      g.UpdatedAt,
		ApprovedAt:     g.ApprovedAt,
	}
}

func NormalizePublicKey(publicKey string) string {
	return strings.TrimSpace(publicKey)
}

func IsValidRole(role GearRole) bool {
	switch role {
	case GearRoleUnspecified, GearRoleAdmin, GearRolePeer, GearRoleDevice:
		return true
	default:
		return false
	}
}

func IsValidStatus(status GearStatus) bool {
	switch status {
	case GearStatusUnspecified, GearStatusActive, GearStatusBlocked:
		return true
	default:
		return false
	}
}

func IsValidChannel(channel GearFirmwareChannel) bool {
	switch channel {
	case "", GearFirmwareChannelRollback, GearFirmwareChannelStable, GearFirmwareChannelBeta, GearFirmwareChannelTesting:
		return true
	default:
		return false
	}
}
