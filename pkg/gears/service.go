package gears

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	store              *Store
	registrationTokens map[string]RegistrationToken
	now                func() time.Time
}

func NewService(store *Store, registrationTokens map[string]RegistrationToken) *Service {
	return &Service{
		store:              store,
		registrationTokens: registrationTokens,
		now:                time.Now,
	}
}

func (s *Service) Get(ctx context.Context, publicKey string) (Gear, error) {
	publicKey = NormalizePublicKey(publicKey)
	return s.store.Get(ctx, publicKey)
}

func (s *Service) List(ctx context.Context, opts ListOptions) ([]Gear, error) {
	return s.store.List(ctx, opts)
}

func (s *Service) ResolveBySN(ctx context.Context, sn string) (Gear, error) {
	publicKey, err := s.store.ResolveBySN(ctx, sn)
	if err != nil {
		return Gear{}, err
	}
	return s.Get(ctx, publicKey)
}

func (s *Service) ResolveByIMEI(ctx context.Context, tac, serial string) (Gear, error) {
	publicKey, err := s.store.ResolveByIMEI(ctx, tac, serial)
	if err != nil {
		return Gear{}, err
	}
	return s.Get(ctx, publicKey)
}

func (s *Service) ListByLabel(ctx context.Context, key, value string) ([]Gear, error) {
	return s.store.ListByLabel(ctx, key, value)
}

func (s *Service) ListByCertification(ctx context.Context, certType GearCertificationType, authority GearCertificationAuthority, id string) ([]Gear, error) {
	return s.store.ListByCertification(ctx, certType, authority, id)
}

func (s *Service) ListByFirmware(ctx context.Context, depot string, channel GearFirmwareChannel) ([]Gear, error) {
	return s.store.ListByFirmware(ctx, depot, channel)
}

func (s *Service) Register(ctx context.Context, req RegistrationRequest) (RegistrationResult, error) {
	publicKey := NormalizePublicKey(req.PublicKey)
	if publicKey == "" {
		return RegistrationResult{}, fmt.Errorf("gears: empty public key")
	}

	if exists, err := s.store.Exists(ctx, publicKey); err != nil {
		return RegistrationResult{}, err
	} else if exists {
		return RegistrationResult{}, ErrGearAlreadyExists
	}

	gear := Gear{
		PublicKey:      publicKey,
		Role:           GearRoleUnspecified,
		Status:         GearStatusUnspecified,
		Device:         req.Device,
		Configuration:  Configuration{},
		AutoRegistered: true,
	}

	if tokenName := strings.TrimSpace(req.RegistrationToken); tokenName != "" {
		token, ok := s.registrationTokens[tokenName]
		if !ok {
			return RegistrationResult{}, fmt.Errorf("gears: unknown registration token")
		}
		gear.Role = token.Role
		gear.Status = GearStatusActive
		gear.ApprovedAt = s.now().UnixMilli()
	}

	if err := s.store.Create(ctx, gear); err != nil {
		return RegistrationResult{}, err
	}
	created, err := s.store.Get(ctx, publicKey)
	if err != nil {
		return RegistrationResult{}, err
	}
	return RegistrationResult{Gear: created, Registered: created.Registration()}, nil
}

func (s *Service) PutInfo(ctx context.Context, publicKey string, info DeviceInfo) (Gear, error) {
	gear, err := s.Get(ctx, publicKey)
	if err != nil {
		return Gear{}, err
	}
	gear.Device = info
	if err := s.store.Put(ctx, gear); err != nil {
		return Gear{}, err
	}
	updated, err := s.store.Get(ctx, publicKey)
	if err != nil {
		return Gear{}, err
	}
	return updated, nil
}

func (s *Service) PutConfig(ctx context.Context, publicKey string, cfg Configuration) (Gear, error) {
	if !IsValidChannel(cfg.Firmware.Channel) {
		return Gear{}, fmt.Errorf("gears: invalid firmware channel %q", cfg.Firmware.Channel)
	}
	gear, err := s.Get(ctx, publicKey)
	if err != nil {
		return Gear{}, err
	}
	gear.Configuration = cfg
	if err := s.store.Put(ctx, gear); err != nil {
		return Gear{}, err
	}
	updated, err := s.store.Get(ctx, publicKey)
	if err != nil {
		return Gear{}, err
	}
	return updated, nil
}

func (s *Service) Approve(ctx context.Context, publicKey string, role GearRole) (Gear, error) {
	if role == GearRoleUnspecified || !IsValidRole(role) {
		return Gear{}, fmt.Errorf("gears: invalid role %q", role)
	}
	gear, err := s.Get(ctx, publicKey)
	if err != nil {
		return Gear{}, err
	}
	gear.Role = role
	gear.Status = GearStatusActive
	gear.ApprovedAt = s.now().UnixMilli()
	if err := s.store.Put(ctx, gear); err != nil {
		return Gear{}, err
	}
	updated, err := s.store.Get(ctx, publicKey)
	if err != nil {
		return Gear{}, err
	}
	return updated, nil
}

func (s *Service) Block(ctx context.Context, publicKey string) (Gear, error) {
	gear, err := s.Get(ctx, publicKey)
	if err != nil {
		return Gear{}, err
	}
	gear.Status = GearStatusBlocked
	if err := s.store.Put(ctx, gear); err != nil {
		return Gear{}, err
	}
	updated, err := s.store.Get(ctx, publicKey)
	if err != nil {
		return Gear{}, err
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, publicKey string) (Gear, error) {
	gear, err := s.Get(ctx, publicKey)
	if err != nil {
		return Gear{}, err
	}
	gear.Role = GearRoleUnspecified
	gear.Status = GearStatusUnspecified
	gear.ApprovedAt = 0
	if err := s.store.Put(ctx, gear); err != nil {
		return Gear{}, err
	}
	updated, err := s.store.Get(ctx, publicKey)
	if err != nil {
		return Gear{}, err
	}
	return updated, nil
}

func (s *Service) Refresh(ctx context.Context, publicKey string, patch RefreshPatch) (RefreshResult, error) {
	gear, err := s.Get(ctx, publicKey)
	if err != nil {
		return RefreshResult{}, err
	}
	gear, updatedFields, err := ApplyRefresh(gear, patch)
	if err != nil {
		return RefreshResult{}, err
	}
	if len(updatedFields) == 0 {
		return RefreshResult{Gear: gear}, nil
	}
	if err := s.store.Put(ctx, gear); err != nil {
		return RefreshResult{}, err
	}
	updatedGear, err := s.store.Get(ctx, publicKey)
	if err != nil {
		return RefreshResult{}, err
	}
	return RefreshResult{Gear: updatedGear, UpdatedFields: updatedFields}, nil
}

func (s *Service) RefreshFromProvider(ctx context.Context, publicKey string, provider DeviceProvider) (RefreshResult, error) {
	if provider == nil {
		return RefreshResult{}, errors.New("gears: nil device provider")
	}
	var patch RefreshPatch
	errs := make(map[string]string)

	info, err := provider.GetInfo(ctx, publicKey)
	if err != nil {
		errs["info"] = err.Error()
	} else {
		patch.Info = &info
	}

	identifiers, err := provider.GetIdentifiers(ctx, publicKey)
	if err != nil {
		errs["identifiers"] = err.Error()
	} else {
		patch.Identifiers = &identifiers
	}

	version, err := provider.GetVersion(ctx, publicKey)
	if err != nil {
		errs["version"] = err.Error()
	} else {
		patch.Version = &version
	}

	result, err := s.Refresh(ctx, publicKey, patch)
	if err != nil {
		if len(errs) > 0 && errors.Is(err, ErrNoRefreshData) {
			return RefreshResult{Errors: errs}, err
		}
		return RefreshResult{Errors: errs}, err
	}
	if len(errs) > 0 {
		result.Errors = errs
	}
	return result, nil
}
