//go:build cgo && portaudio_device && ((linux && (amd64 || arm64)) || (darwin && (amd64 || arm64)))

package portaudio

import "testing"

func TestListDevicesLive(t *testing.T) {
	devices, err := ListDevices()
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(devices) == 0 {
		t.Skip("no audio devices detected in current runtime")
	}
}
