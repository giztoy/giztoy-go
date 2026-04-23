package gear

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"

	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

func (s *Server) register(ctx context.Context, request serverpublic.RegistrationRequest) (apitypes.Gear, error) {
	publicKey := normalizePublicKey(request.PublicKey)
	if publicKey == "" {
		return apitypes.Gear{}, fmt.Errorf("gear: empty public key")
	}
	exists, err := s.exists(ctx, publicKey)
	if err != nil {
		return apitypes.Gear{}, err
	}
	if exists {
		return apitypes.Gear{}, ErrGearAlreadyExists
	}

	autoRegistered := true
	device, err := convertViaJSON[apitypes.DeviceInfo](request.Device)
	if err != nil {
		return apitypes.Gear{}, err
	}
	gear := apitypes.Gear{
		PublicKey:      publicKey,
		Role:           apitypes.GearRoleUnspecified,
		Status:         apitypes.GearStatusUnspecified,
		Device:         device,
		Configuration:  apitypes.Configuration{},
		AutoRegistered: &autoRegistered,
	}

	if request.RegistrationToken != nil {
		tokenName := strings.TrimSpace(*request.RegistrationToken)
		if tokenName != "" {
			role, ok := s.RegistrationTokens[tokenName]
			if !ok {
				return apitypes.Gear{}, fmt.Errorf("gear: unknown registration token")
			}
			approvedAt := time.Now()
			gear.Role = apitypes.GearRole(role)
			gear.Status = apitypes.GearStatusActive
			gear.ApprovedAt = &approvedAt
		}
	}

	created, err := s.create(ctx, gear)
	if err != nil {
		return apitypes.Gear{}, err
	}
	return created, nil
}

func (s *Server) putInfo(ctx context.Context, publicKey string, info apitypes.DeviceInfo) (apitypes.Gear, error) {
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return apitypes.Gear{}, err
	}
	gear.Device = info
	return s.put(ctx, gear)
}

// LoadGear returns the stored gear record for a public key.
func (s *Server) LoadGear(ctx context.Context, publicKey string) (apitypes.Gear, error) {
	return s.get(ctx, publicKey)
}

// SaveGear stores a full gear record and returns the persisted value.
func (s *Server) SaveGear(ctx context.Context, gear apitypes.Gear) (apitypes.Gear, error) {
	return s.put(ctx, gear)
}

func (s *Server) putConfig(ctx context.Context, publicKey string, cfg apitypes.Configuration) (apitypes.Gear, error) {
	if err := validateConfiguration(cfg); err != nil {
		return apitypes.Gear{}, err
	}
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return apitypes.Gear{}, err
	}
	gear.Configuration = cfg
	return s.put(ctx, gear)
}

func (s *Server) approve(ctx context.Context, publicKey string, role apitypes.GearRole) (apitypes.Gear, error) {
	if role == apitypes.GearRoleUnspecified || !role.Valid() {
		return apitypes.Gear{}, fmt.Errorf("gear: invalid role %q", role)
	}
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return apitypes.Gear{}, err
	}
	approvedAt := time.Now()
	gear.Role = role
	gear.Status = apitypes.GearStatusActive
	gear.ApprovedAt = &approvedAt
	return s.put(ctx, gear)
}

func (s *Server) block(ctx context.Context, publicKey string) (apitypes.Gear, error) {
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return apitypes.Gear{}, err
	}
	gear.Status = apitypes.GearStatusBlocked
	return s.put(ctx, gear)
}

func (s *Server) delete(ctx context.Context, publicKey string) (apitypes.Gear, error) {
	gear, err := s.get(ctx, publicKey)
	if err != nil {
		return apitypes.Gear{}, err
	}
	gear.Role = apitypes.GearRoleUnspecified
	gear.Status = apitypes.GearStatusUnspecified
	gear.ApprovedAt = nil
	return s.put(ctx, gear)
}

func (s *Server) get(ctx context.Context, publicKey string) (apitypes.Gear, error) {
	store, err := s.store()
	if err != nil {
		return apitypes.Gear{}, err
	}
	publicKey = normalizePublicKey(publicKey)
	data, err := store.Get(ctx, gearKey(publicKey))
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return apitypes.Gear{}, ErrGearNotFound
		}
		return apitypes.Gear{}, fmt.Errorf("gear: get %s: %w", publicKey, err)
	}
	var gear apitypes.Gear
	if err := json.Unmarshal(data, &gear); err != nil {
		return apitypes.Gear{}, fmt.Errorf("gear: decode %s: %w", publicKey, err)
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

func (s *Server) create(ctx context.Context, gear apitypes.Gear) (apitypes.Gear, error) {
	if err := validateGear(gear); err != nil {
		return apitypes.Gear{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.get(ctx, gear.PublicKey); err == nil {
		return apitypes.Gear{}, ErrGearAlreadyExists
	} else if !errors.Is(err, ErrGearNotFound) {
		return apitypes.Gear{}, err
	}

	now := time.Now()
	gear.CreatedAt = now
	gear.UpdatedAt = now
	if err := s.writeGearLocked(ctx, gear, nil); err != nil {
		return apitypes.Gear{}, err
	}
	return s.get(ctx, gear.PublicKey)
}

func (s *Server) put(ctx context.Context, gear apitypes.Gear) (apitypes.Gear, error) {
	if err := validateGear(gear); err != nil {
		return apitypes.Gear{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	old, err := s.get(ctx, gear.PublicKey)
	if err != nil && !errors.Is(err, ErrGearNotFound) {
		return apitypes.Gear{}, err
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
		return apitypes.Gear{}, err
	}
	return s.get(ctx, gear.PublicKey)
}

func (s *Server) list(ctx context.Context) ([]apitypes.Gear, error) {
	store, err := s.store()
	if err != nil {
		return nil, err
	}
	items := make([]apitypes.Gear, 0)
	for entry, err := range store.List(ctx, gearsPrefix()) {
		if err != nil {
			return nil, fmt.Errorf("gear: list: %w", err)
		}
		var gear apitypes.Gear
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

func (s *Server) listPage(ctx context.Context, cursor string, limit int) ([]apitypes.Gear, bool, *string, error) {
	items, err := s.list(ctx)
	if err != nil {
		return nil, false, nil, err
	}
	start := 0
	if cursor != "" {
		start = len(items)
		for index, gear := range items {
			if gear.PublicKey == cursor {
				start = index + 1
				break
			}
		}
	}
	if start >= len(items) {
		return nil, false, nil, nil
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	page := items[start:end]
	if end >= len(items) {
		return page, false, nil, nil
	}
	nextCursor := page[len(page)-1].PublicKey
	return page, true, &nextCursor, nil
}

func (s *Server) resolveBySN(ctx context.Context, sn string) (string, error) {
	return s.resolveSingle(ctx, snKey(sn), ErrGearNotFound)
}

func (s *Server) resolveByIMEI(ctx context.Context, tac, serial string) (string, error) {
	return s.resolveSingle(ctx, imeiKey(tac, serial), ErrGearNotFound)
}

func (s *Server) listByLabel(ctx context.Context, key, value, cursor string, limit int) ([]apitypes.Gear, bool, *string, error) {
	return s.listByReferencePrefixPage(ctx, labelPrefix(key, value), cursor, limit)
}

func (s *Server) listByCertification(ctx context.Context, certType apitypes.GearCertificationType, authority apitypes.GearCertificationAuthority, id, cursor string, limit int) ([]apitypes.Gear, bool, *string, error) {
	return s.listByReferencePrefixPage(ctx, certificationPrefix(certType, authority, id), cursor, limit)
}

func (s *Server) listByFirmware(ctx context.Context, depot string, channel apitypes.GearFirmwareChannel, cursor string, limit int) ([]apitypes.Gear, bool, *string, error) {
	return s.listByReferencePrefixPage(ctx, firmwarePrefix(depot, channel), cursor, limit)
}

func (s *Server) writeGearLocked(ctx context.Context, gear apitypes.Gear, previous *apitypes.Gear) error {
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

func (s *Server) listByReferencePrefixPage(ctx context.Context, prefix kv.Key, cursor string, limit int) ([]apitypes.Gear, bool, *string, error) {
	store, err := s.store()
	if err != nil {
		return nil, false, nil, err
	}
	entries, err := kv.ListAfter(ctx, store, prefix, cursorAfterKey(prefix, cursor), limit+1)
	if err != nil {
		return nil, false, nil, err
	}
	pageEntries, hasNext, nextCursor := paginateEntries(entries, limit)

	items := make([]apitypes.Gear, 0, len(pageEntries))
	for _, entry := range pageEntries {
		if len(entry.Key) == 0 {
			continue
		}
		publicKey := entry.Key[len(entry.Key)-1]
		gear, err := s.get(ctx, publicKey)
		if err != nil {
			if errors.Is(err, ErrGearNotFound) {
				continue
			}
			return nil, false, nil, err
		}
		items = append(items, gear)
	}
	return items, hasNext, nextCursor, nil
}

func cursorAfterKey(prefix kv.Key, cursor string) kv.Key {
	if cursor == "" {
		return nil
	}
	after := append(kv.Key{}, prefix...)
	return append(after, cursor)
}

func paginateEntries(entries []kv.Entry, limit int) ([]kv.Entry, bool, *string) {
	if len(entries) == 0 {
		return nil, false, nil
	}

	hasNext := len(entries) > limit
	if !hasNext {
		return entries, false, nil
	}

	page := entries[:limit]
	if len(page) == 0 || len(page[len(page)-1].Key) == 0 {
		return page, true, nil
	}

	nextCursor := page[len(page)-1].Key[len(page[len(page)-1].Key)-1]
	return page, true, &nextCursor
}

func (s *Server) store() (kv.Store, error) {
	if s.Store == nil {
		return nil, errors.New("gear: store not configured")
	}
	return s.Store, nil
}

func (s *Server) peerRuntime(ctx context.Context, publicKey string) apitypes.Runtime {
	if s.PeerManager == nil {
		return apitypes.Runtime{}
	}
	return s.PeerManager.PeerRuntime(ctx, publicKey)
}

func optionalGear(gear apitypes.Gear, err error) *apitypes.Gear {
	if err != nil {
		return nil
	}
	cp := gear
	return &cp
}
