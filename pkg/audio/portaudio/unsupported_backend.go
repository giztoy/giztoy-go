//go:build !(cgo && ((linux && (amd64 || arm64)) || (darwin && (amd64 || arm64))))

package portaudio

import (
	"fmt"
	"runtime"
)

func unsupportedErr() error {
	return fmt.Errorf(
		"portaudio: unsupported platform %s/%s: requires cgo + one of %s",
		runtime.GOOS,
		runtime.GOARCH,
		supportedPlatformDescription,
	)
}

type unsupportedBackend struct{}

func newBackend() backend {
	return unsupportedBackend{}
}

func (unsupportedBackend) Name() string {
	return "unsupported"
}

func (unsupportedBackend) Init() error {
	return unsupportedErr()
}

func (unsupportedBackend) Terminate() error {
	return nil
}

func (unsupportedBackend) ListDevices() ([]DeviceInfo, error) {
	return nil, unsupportedErr()
}

func (unsupportedBackend) DefaultInputDevice() (int, error) {
	return 0, unsupportedErr()
}

func (unsupportedBackend) DefaultOutputDevice() (int, error) {
	return 0, unsupportedErr()
}

func (unsupportedBackend) IsFormatSupported(direction streamDirection, cfg StreamConfig) error {
	_ = direction
	if err := cfg.Validate(); err != nil {
		return err
	}
	return unsupportedErr()
}

func (unsupportedBackend) OpenStream(direction streamDirection, cfg StreamConfig) (streamHandle, error) {
	_ = direction
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return nil, unsupportedErr()
}
