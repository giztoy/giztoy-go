package portaudio

import "fmt"

type streamDirection uint8

const (
	directionInput streamDirection = iota + 1
	directionOutput
)

type streamHandle interface {
	Start() error
	Stop() error
	Close() error
	Read([]byte) (int, error)
	Write([]byte) (int, error)
}

type backend interface {
	Name() string
	Init() error
	Terminate() error
	ListDevices() ([]DeviceInfo, error)
	DefaultInputDevice() (int, error)
	DefaultOutputDevice() (int, error)
	IsFormatSupported(direction streamDirection, cfg StreamConfig) error
	OpenStream(direction streamDirection, cfg StreamConfig) (streamHandle, error)
}

func copyDevices(devices []DeviceInfo) []DeviceInfo {
	if len(devices) == 0 {
		return nil
	}
	out := make([]DeviceInfo, len(devices))
	copy(out, devices)
	return out
}

func findDeviceByID(devices []DeviceInfo, id int) (*DeviceInfo, error) {
	for i := range devices {
		if devices[i].ID == id {
			device := devices[i]
			return &device, nil
		}
	}
	return nil, fmt.Errorf("%w: id=%d", ErrDeviceNotFound, id)
}
