package gears

type ServiceKind string

const (
	ServiceKindPublicDevice  ServiceKind = "public_device"
	ServiceKindAdmin         ServiceKind = "admin"
	ServiceKindPeer          ServiceKind = "peer"
	ServiceKindDeviceApp     ServiceKind = "device_app"
	ServiceKindDeviceReverse ServiceKind = "device_reverse"
)

func CanAccess(role GearRole, status GearStatus, service ServiceKind) bool {
	switch service {
	case ServiceKindPublicDevice:
		// Bootstrap/public API is available to any connected device, including
		// blocked ones, except register is handled by higher-level business logic.
		return true
	case ServiceKindAdmin:
		return role == GearRoleAdmin && status == GearStatusActive
	case ServiceKindPeer:
		return role == GearRolePeer && status == GearStatusActive
	case ServiceKindDeviceApp, ServiceKindDeviceReverse:
		return role == GearRoleDevice && status == GearStatusActive
	default:
		return false
	}
}
