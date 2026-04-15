package gear

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

func (s *Server) register(ctx context.Context, request gearservice.RegistrationRequest) (gearservice.RegistrationResult, error) {
	publicKey := normalizePublicKey(request.PublicKey)
	if publicKey == "" {
		return gearservice.RegistrationResult{}, fmt.Errorf("gear: empty public key")
	}
	exists, err := s.exists(ctx, publicKey)
	if err != nil {
		return gearservice.RegistrationResult{}, err
	}
	if exists {
		return gearservice.RegistrationResult{}, ErrGearAlreadyExists
	}

	autoRegistered := true
	gear := gearservice.Gear{
		PublicKey:      publicKey,
		Role:           gearservice.GearRoleUnspecified,
		Status:         gearservice.GearStatusUnspecified,
		Device:         request.Device,
		Configuration:  gearservice.Configuration{},
		AutoRegistered: &autoRegistered,
	}

	if request.RegistrationToken != nil {
		tokenName := strings.TrimSpace(*request.RegistrationToken)
		if tokenName != "" {
			role, ok := s.RegistrationTokens[tokenName]
			if !ok {
				return gearservice.RegistrationResult{}, fmt.Errorf("gear: unknown registration token")
			}
			approvedAt := time.Now()
			gear.Role = role
			gear.Status = gearservice.GearStatusActive
			gear.ApprovedAt = &approvedAt
		}
	}

	created, err := s.create(ctx, gear)
	if err != nil {
		return gearservice.RegistrationResult{}, err
	}
	return gearservice.RegistrationResult{
		Gear:         created,
		Registration: toGearRegistration(created),
	}, nil
}

func (s *Server) putInfo(ctx context.Context, publicKey string, info gearservice.DeviceInfo) (gearservice.Gear, error) {
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return gearservice.Gear{}, err
	}
	gear.Device = info
	return s.put(ctx, gear)
}

// LoadGear returns the stored gear record for a public key.
func (s *Server) LoadGear(ctx context.Context, publicKey string) (gearservice.Gear, error) {
	return s.get(ctx, publicKey)
}

// SaveGear stores a full gear record and returns the persisted value.
func (s *Server) SaveGear(ctx context.Context, gear gearservice.Gear) (gearservice.Gear, error) {
	return s.put(ctx, gear)
}

func (s *Server) putConfig(ctx context.Context, publicKey string, cfg gearservice.Configuration) (gearservice.Gear, error) {
	if err := validateConfiguration(cfg); err != nil {
		return gearservice.Gear{}, err
	}
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return gearservice.Gear{}, err
	}
	gear.Configuration = cfg
	return s.put(ctx, gear)
}

func (s *Server) approve(ctx context.Context, publicKey string, role gearservice.GearRole) (gearservice.Gear, error) {
	if role == gearservice.GearRoleUnspecified || !role.Valid() {
		return gearservice.Gear{}, fmt.Errorf("gear: invalid role %q", role)
	}
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return gearservice.Gear{}, err
	}
	approvedAt := time.Now()
	gear.Role = role
	gear.Status = gearservice.GearStatusActive
	gear.ApprovedAt = &approvedAt
	return s.put(ctx, gear)
}

func (s *Server) block(ctx context.Context, publicKey string) (gearservice.Gear, error) {
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return gearservice.Gear{}, err
	}
	gear.Status = gearservice.GearStatusBlocked
	return s.put(ctx, gear)
}

func (s *Server) delete(ctx context.Context, publicKey string) (gearservice.Gear, error) {
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return gearservice.Gear{}, err
	}
	gear.Role = gearservice.GearRoleUnspecified
	gear.Status = gearservice.GearStatusUnspecified
	gear.ApprovedAt = nil
	return s.put(ctx, gear)
}

func (s *Server) get(ctx context.Context, publicKey string) (gearservice.Gear, error) {
	store, err := s.store()
	if err != nil {
		return gearservice.Gear{}, err
	}
	publicKey = normalizePublicKey(publicKey)
	data, err := store.Get(ctx, gearKey(publicKey))
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return gearservice.Gear{}, ErrGearNotFound
		}
		return gearservice.Gear{}, fmt.Errorf("gear: get %s: %w", publicKey, err)
	}
	var gear gearservice.Gear
	if err := json.Unmarshal(data, &gear); err != nil {
		return gearservice.Gear{}, fmt.Errorf("gear: decode %s: %w", publicKey, err)
	}
	return gear, nil
}

func (s *Server) exists(ctx context.Context, publicKey string) (bool, error) {
	_, err := s.get(ctx, publicKey)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrGearNotFound) {
		return false, nil
	}
	return false, err
}

func (s *Server) create(ctx context.Context, gear gearservice.Gear) (gearservice.Gear, error) {
	if err := validateGear(gear); err != nil {
		return gearservice.Gear{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.get(ctx, gear.PublicKey); err == nil {
		return gearservice.Gear{}, ErrGearAlreadyExists
	} else if !errors.Is(err, ErrGearNotFound) {
		return gearservice.Gear{}, err
	}

	now := time.Now()
	gear.CreatedAt = now
	gear.UpdatedAt = now
	if err := s.writeGearLocked(ctx, gear, nil); err != nil {
		return gearservice.Gear{}, err
	}
	return s.get(ctx, gear.PublicKey)
}

func (s *Server) put(ctx context.Context, gear gearservice.Gear) (gearservice.Gear, error) {
	if err := validateGear(gear); err != nil {
		return gearservice.Gear{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	old, err := s.get(ctx, gear.PublicKey)
	if err != nil && !errors.Is(err, ErrGearNotFound) {
		return gearservice.Gear{}, err
	}
	if gear.CreatedAt.IsZero() {
		if errors.Is(err, ErrGearNotFound) {
			gear.CreatedAt = time.Now()
		} else {
			gear.CreatedAt = old.CreatedAt
		}
	}
	gear.UpdatedAt = time.Now()
	if err := s.writeGearLocked(ctx, gear, optionalGear(old, err)); err != nil {
		return gearservice.Gear{}, err
	}
	return s.get(ctx, gear.PublicKey)
}

func (s *Server) list(ctx context.Context) ([]gearservice.Gear, error) {
	store, err := s.store()
	if err != nil {
		return nil, err
	}
	items := make([]gearservice.Gear, 0)
	for entry, err := range store.List(ctx, gearsPrefix()) {
		if err != nil {
			return nil, fmt.Errorf("gear: list: %w", err)
		}
		var gear gearservice.Gear
		if err := json.Unmarshal(entry.Value, &gear); err != nil {
			return nil, fmt.Errorf("gear: decode list %s: %w", entry.Key.String(), err)
		}
		items = append(items, gear)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].PublicKey < items[j].PublicKey
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items, nil
}

func (s *Server) resolveBySN(ctx context.Context, sn string) (string, error) {
	return s.resolveSingle(ctx, snKey(sn), ErrGearNotFound)
}

func (s *Server) resolveByIMEI(ctx context.Context, tac, serial string) (string, error) {
	return s.resolveSingle(ctx, imeiKey(tac, serial), ErrGearNotFound)
}

func (s *Server) listByLabel(ctx context.Context, key, value string) ([]gearservice.Gear, error) {
	return s.listByReferencePrefix(ctx, labelPrefix(key, value))
}

func (s *Server) listByCertification(ctx context.Context, certType gearservice.GearCertificationType, authority gearservice.GearCertificationAuthority, id string) ([]gearservice.Gear, error) {
	return s.listByReferencePrefix(ctx, certificationPrefix(certType, authority, id))
}

func (s *Server) listByFirmware(ctx context.Context, depot string, channel gearservice.GearFirmwareChannel) ([]gearservice.Gear, error) {
	return s.listByReferencePrefix(ctx, firmwarePrefix(depot, channel))
}

func (s *Server) writeGearLocked(ctx context.Context, gear gearservice.Gear, previous *gearservice.Gear) error {
	store, err := s.store()
	if err != nil {
		return err
	}
	data, err := json.Marshal(gear)
	if err != nil {
		return fmt.Errorf("gear: encode %s: %w", gear.PublicKey, err)
	}

	var deletes []kv.Key
	if previous != nil {
		deletes = append(deletes, indexKeys(*previous)...)
	}

	entries := []kv.Entry{{Key: gearKey(gear.PublicKey), Value: data}}
	entries = append(entries, indexEntries(gear)...)

	if len(deletes) > 0 {
		if err := store.BatchDelete(ctx, deletes); err != nil {
			return fmt.Errorf("gear: delete stale indexes %s: %w", gear.PublicKey, err)
		}
	}
	if err := store.BatchSet(ctx, entries); err != nil {
		return fmt.Errorf("gear: write %s: %w", gear.PublicKey, err)
	}
	return nil
}

func (s *Server) resolveSingle(ctx context.Context, key kv.Key, notFound error) (string, error) {
	store, err := s.store()
	if err != nil {
		return "", err
	}
	data, err := store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return "", notFound
		}
		return "", err
	}
	return string(data), nil
}

func (s *Server) listByReferencePrefix(ctx context.Context, prefix kv.Key) ([]gearservice.Gear, error) {
	store, err := s.store()
	if err != nil {
		return nil, err
	}
	publicKeys := make([]string, 0)
	for entry, err := range store.List(ctx, prefix) {
		if err != nil {
			return nil, err
		}
		if len(entry.Key) == 0 {
			continue
		}
		publicKeys = append(publicKeys, entry.Key[len(entry.Key)-1])
	}
	sort.Strings(publicKeys)

	items := make([]gearservice.Gear, 0, len(publicKeys))
	seen := make(map[string]struct{}, len(publicKeys))
	for _, publicKey := range publicKeys {
		if _, ok := seen[publicKey]; ok {
			continue
		}
		seen[publicKey] = struct{}{}
		gear, err := s.get(ctx, publicKey)
		if err != nil {
			if errors.Is(err, ErrGearNotFound) {
				continue
			}
			return nil, err
		}
		items = append(items, gear)
	}
	return items, nil
}

func (s *Server) store() (kv.Store, error) {
	if s.Store == nil {
		return nil, errors.New("gear: store not configured")
	}
	return s.Store, nil
}

func (s *Server) peerRuntime(ctx context.Context, publicKey string) gearservice.Runtime {
	if s.PeerManager == nil {
		return gearservice.Runtime{}
	}
	return s.PeerManager.PeerRuntime(ctx, publicKey)
}

func optionalGear(gear gearservice.Gear, err error) *gearservice.Gear {
	if err != nil {
		return nil
	}
	cp := gear
	return &cp
}
