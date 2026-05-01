package gear

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
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
		Store: mustBadgerInMemory(t, nil),
	}

	_, err := server.register(context.Background(), "", gearservice.RegistrationRequest{})
	if err == nil || !strings.Contains(err.Error(), "empty public key") {
		t.Fatalf("empty public key err = %v", err)
	}

	registered, err := server.register(context.Background(), "server", gearservice.RegistrationRequest{
		Device: apitypes.DeviceInfo{},
	})
	if err != nil {
		t.Fatalf("register without token error = %v", err)
	}
	if registered.Status != apitypes.GearStatusActive || registered.Role != apitypes.GearRoleUnspecified {
		t.Fatalf("registered gear = %+v", registered)
	}
}

func TestStoreOpsEnsureConnectedGearAndRegister(t *testing.T) {
	server := &Server{Store: mustBadgerInMemory(t, nil)}
	ctx := context.Background()

	connected, err := server.EnsureConnectedGear(ctx, "server")
	if err != nil {
		t.Fatalf("EnsureConnectedGear error = %v", err)
	}
	if connected.Role != apitypes.GearRoleUnspecified || connected.Status != apitypes.GearStatusActive {
		t.Fatalf("connected gear = %+v", connected)
	}

	name := "demo"
	registered, err := server.register(ctx, "server", gearservice.RegistrationRequest{
		Device: apitypes.DeviceInfo{Name: &name},
	})
	if err != nil {
		t.Fatalf("register existing connected gear error = %v", err)
	}
	if registered.Role != apitypes.GearRoleUnspecified || registered.Status != apitypes.GearStatusActive {
		t.Fatalf("registered gear = %+v", registered)
	}
	if registered.Device.Name == nil || *registered.Device.Name != "demo" {
		t.Fatalf("registered device = %+v", registered.Device)
	}
}

func TestStoreOpsEnsureConnectedGearPreservesExisting(t *testing.T) {
	server := &Server{Store: mustBadgerInMemory(t, nil)}
	ctx := context.Background()
	if _, err := server.SaveGear(ctx, apitypes.Gear{
		PublicKey:     "server",
		Role:          apitypes.GearRoleAdmin,
		Status:        apitypes.GearStatusBlocked,
		Configuration: apitypes.Configuration{},
	}); err != nil {
		t.Fatalf("SaveGear error = %v", err)
	}

	got, err := server.EnsureConnectedGear(ctx, "server")
	if err != nil {
		t.Fatalf("EnsureConnectedGear error = %v", err)
	}
	if got.Role != apitypes.GearRoleAdmin || got.Status != apitypes.GearStatusBlocked {
		t.Fatalf("EnsureConnectedGear overwrote existing gear: %+v", got)
	}
}

func TestStoreOpsLoadAndSaveGear(t *testing.T) {
	server := &Server{Store: mustBadgerInMemory(t, nil)}
	want := apitypes.Gear{
		PublicKey: "server",
		Role:      apitypes.GearRoleGear,
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
		PublicKey:     "server",
		Role:          apitypes.GearRoleGear,
		Status:        apitypes.GearStatusActive,
		Configuration: apitypes.Configuration{},
	}); err != nil {
		t.Fatalf("SaveGear error = %v", err)
	}

	if exists, err := server.exists(context.Background(), "server"); err != nil || !exists {
		t.Fatalf("exists(peer) = %v, %v", exists, err)
	}
}
