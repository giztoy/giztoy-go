package portaudio

import (
	"fmt"
	"runtime"
	"sync"
)

// Driver manages PortAudio lifecycle and stream creation.
type Driver struct {
	backend backend

	mu          sync.Mutex
	initialized bool
	refs        int
}

// NewDriver returns a driver backed by compile-time selected backend.
func NewDriver() *Driver {
	return newDriverWithBackend(newBackend())
}

func newDriverWithBackend(b backend) *Driver {
	if b == nil {
		b = newBackend()
	}
	return &Driver{backend: b}
}

var defaultDriver = NewDriver()

// NativeRuntimeSupported reports whether native backend can be enabled at runtime.
func NativeRuntimeSupported() bool {
	return nativeCGOEnabled && isSupportedPlatform(runtime.GOOS, runtime.GOARCH)
}

// BackendName returns backend marker string.
func BackendName() string {
	if defaultDriver == nil || defaultDriver.backend == nil {
		return "unknown"
	}
	return defaultDriver.backend.Name()
}

func (d *Driver) acquire() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.refs == 0 {
		if err := d.backend.Init(); err != nil {
			return err
		}
		d.initialized = true
	}
	d.refs++
	return nil
}

func (d *Driver) release() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.refs == 0 {
		return nil
	}

	d.refs--
	if d.refs > 0 || !d.initialized {
		return nil
	}

	d.initialized = false
	if err := d.backend.Terminate(); err != nil {
		return fmt.Errorf("portaudio: terminate failed: %w", err)
	}
	return nil
}

func (d *Driver) withLifecycle(fn func() error) error {
	if err := d.acquire(); err != nil {
		return err
	}
	defer func() {
		_ = d.release()
	}()
	return fn()
}

// ListDevices returns currently discovered devices.
func (d *Driver) ListDevices() ([]DeviceInfo, error) {
	var devices []DeviceInfo
	err := d.withLifecycle(func() error {
		items, err := d.backend.ListDevices()
		if err != nil {
			return err
		}
		devices = copyDevices(items)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return devices, nil
}

// DefaultInputDevice returns backend default input device info.
func (d *Driver) DefaultInputDevice() (*DeviceInfo, error) {
	var (
		id      int
		devices []DeviceInfo
	)
	err := d.withLifecycle(func() error {
		var err error
		id, err = d.backend.DefaultInputDevice()
		if err != nil {
			return err
		}
		devices, err = d.backend.ListDevices()
		return err
	})
	if err != nil {
		return nil, err
	}
	return findDeviceByID(devices, id)
}

// DefaultOutputDevice returns backend default output device info.
func (d *Driver) DefaultOutputDevice() (*DeviceInfo, error) {
	var (
		id      int
		devices []DeviceInfo
	)
	err := d.withLifecycle(func() error {
		var err error
		id, err = d.backend.DefaultOutputDevice()
		if err != nil {
			return err
		}
		devices, err = d.backend.ListDevices()
		return err
	})
	if err != nil {
		return nil, err
	}
	return findDeviceByID(devices, id)
}

// ListDevices returns currently discovered devices via default driver.
func ListDevices() ([]DeviceInfo, error) {
	return defaultDriver.ListDevices()
}

// DefaultInputDevice returns backend default input device info via default driver.
func DefaultInputDevice() (*DeviceInfo, error) {
	return defaultDriver.DefaultInputDevice()
}

// DefaultOutputDevice returns backend default output device info via default driver.
func DefaultOutputDevice() (*DeviceInfo, error) {
	return defaultDriver.DefaultOutputDevice()
}
