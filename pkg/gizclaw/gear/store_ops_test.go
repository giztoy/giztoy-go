package gear

import (
	"context"
	"errors"
	"strings"
	"testing"

	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
)

func TestStoreOpsHelpers(t *testing.T) {
	server := &Server{}
	if _, err := server.store(); err == nil {
		t.Fatal("store should fail when store is nil")
	}
	if (&Server{}).peerRuntime(context.Background(), "pk").Online {
		t.Fatal("zero peerRuntime should be offline")
	}
	if optionalGear(apitypes.Gear{PublicKey: "x"}, nil) == nil {
		t.Fatal("optionalGear should keep value")
	}
	if optionalGear(apitypes.Gear{}, errors.New("boom")) != nil {
		t.Fatal("optionalGear should drop error case")
	}
}

func TestStoreOpsRegisterValidation(t *testing.T) {
	server := &Server{
		Store:              mustBadgerInMemory(t, nil),
		RegistrationTokens: map[string]apitypes.GearRole{},
	}

	_, err := server.register(context.Background(), serverpublic.RegistrationRequest{})
	if err == nil || !strings.Contains(err.Error(), "empty public key") {
		t.Fatalf("empty public key err = %v", err)
	}

	token := "missing"
	_, err = server.register(context.Background(), serverpublic.RegistrationRequest{
		PublicKey:         "peer",
		RegistrationToken: &token,
	})
	if err == nil || !strings.Contains(err.Error(), "unknown registration token") {
		t.Fatalf("unknown token err = %v", err)
	}
}

func TestStoreOpsLoadAndSaveGear(t *testing.T) {
	server := &Server{Store: mustBadgerInMemory(t, nil)}
	want := apitypes.Gear{
		PublicKey: "peer",
		Role:      apitypes.GearRoleDevice,
		Status:    apitypes.GearStatusActive,
		Device: apitypes.DeviceInfo{
			Name: func() *string {
				value := "demo"
				return &value
			}(),
		},
		Configuration: apitypes.Configuration{},
	}

	got, err := server.SaveGear(context.Background(), want)
	if err != nil {
		t.Fatalf("SaveGear error = %v", err)
	}
	if got.PublicKey != want.PublicKey {
		t.Fatalf("SaveGear public key = %q, want %q", got.PublicKey, want.PublicKey)
	}

	loaded, err := server.LoadGear(context.Background(), want.PublicKey)
	if err != nil {
		t.Fatalf("LoadGear error = %v", err)
	}
	if loaded.PublicKey != want.PublicKey || loaded.Role != want.Role || loaded.Status != want.Status {
		t.Fatalf("LoadGear = %+v", loaded)
	}
	if loaded.Device.Name == nil || *loaded.Device.Name != "demo" {
		t.Fatalf("LoadGear device name = %+v", loaded.Device.Name)
	}
}

func TestStoreOpsLoadGearMissing(t *testing.T) {
	server := &Server{Store: mustBadgerInMemory(t, nil)}

	_, err := server.LoadGear(context.Background(), "missing")
	if !errors.Is(err, ErrGearNotFound) {
		t.Fatalf("LoadGear missing err = %v", err)
	}
}

func TestStoreOpsSaveGearRejectsInvalidGear(t *testing.T) {
	server := &Server{Store: mustBadgerInMemory(t, nil)}

	_, err := server.SaveGear(context.Background(), apitypes.Gear{})
	if err == nil || !strings.Contains(err.Error(), "empty public key") {
		t.Fatalf("SaveGear invalid err = %v", err)
	}
}

func TestStoreOpsExists(t *testing.T) {
	server := &Server{Store: mustBadgerInMemory(t, nil)}

	if exists, err := server.exists(context.Background(), "missing"); err != nil || exists {
		t.Fatalf("exists(missing) = %v, %v", exists, err)
	}

	if _, err := server.SaveGear(context.Background(), apitypes.Gear{
		PublicKey:     "peer",
		Role:          apitypes.GearRoleDevice,
		Status:        apitypes.GearStatusActive,
		Configuration: apitypes.Configuration{},
	}); err != nil {
		t.Fatalf("SaveGear error = %v", err)
	}

	if exists, err := server.exists(context.Background(), "peer"); err != nil || !exists {
		t.Fatalf("exists(peer) = %v, %v", exists, err)
	}
}
