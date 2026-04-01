package gears

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/giztoy/giztoy-go/pkg/kv"
	"github.com/vmihailenco/msgpack/v5"
)

var (
	ErrGearNotFound      = errors.New("gears: gear not found")
	ErrGearAlreadyExists = errors.New("gears: gear already exists")
)

type Store struct {
	kv  kv.Store
	now func() time.Time

	mu sync.Mutex
}

func NewStore(store kv.Store) *Store {
	return &Store{
		kv:  store,
		now: time.Now,
	}
}

func (s *Store) Get(ctx context.Context, publicKey string) (Gear, error) {
	publicKey = NormalizePublicKey(publicKey)
	data, err := s.kv.Get(ctx, gearKey(publicKey))
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return Gear{}, ErrGearNotFound
		}
		return Gear{}, fmt.Errorf("gears: get %s: %w", publicKey, err)
	}
	var gear Gear
	if err := msgpack.Unmarshal(data, &gear); err != nil {
		return Gear{}, fmt.Errorf("gears: decode %s: %w", publicKey, err)
	}
	return gear, nil
}

func (s *Store) Exists(ctx context.Context, publicKey string) (bool, error) {
	_, err := s.Get(ctx, publicKey)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrGearNotFound) {
		return false, nil
	}
	return false, err
}

func (s *Store) Put(ctx context.Context, gear Gear) error {
	gear.PublicKey = NormalizePublicKey(gear.PublicKey)
	if gear.PublicKey == "" {
		return fmt.Errorf("gears: empty public key")
	}
	if !IsValidRole(gear.Role) {
		return fmt.Errorf("gears: invalid role %q", gear.Role)
	}
	if !IsValidStatus(gear.Status) {
		return fmt.Errorf("gears: invalid status %q", gear.Status)
	}
	if !IsValidChannel(gear.Configuration.Firmware.Channel) {
		return fmt.Errorf("gears: invalid firmware channel %q", gear.Configuration.Firmware.Channel)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	old, err := s.Get(ctx, gear.PublicKey)
	if err != nil && !errors.Is(err, ErrGearNotFound) {
		return err
	}
	if gear.CreatedAt == 0 {
		if errors.Is(err, ErrGearNotFound) {
			gear.CreatedAt = s.now().UnixMilli()
		} else {
			gear.CreatedAt = old.CreatedAt
		}
	}
	gear.UpdatedAt = s.now().UnixMilli()
	return s.writeGearLocked(ctx, gear, optionalGear(old, err))
}

func (s *Store) Create(ctx context.Context, gear Gear) error {
	gear.PublicKey = NormalizePublicKey(gear.PublicKey)
	if gear.PublicKey == "" {
		return fmt.Errorf("gears: empty public key")
	}
	if !IsValidRole(gear.Role) {
		return fmt.Errorf("gears: invalid role %q", gear.Role)
	}
	if !IsValidStatus(gear.Status) {
		return fmt.Errorf("gears: invalid status %q", gear.Status)
	}
	if !IsValidChannel(gear.Configuration.Firmware.Channel) {
		return fmt.Errorf("gears: invalid firmware channel %q", gear.Configuration.Firmware.Channel)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.Get(ctx, gear.PublicKey); err == nil {
		return ErrGearAlreadyExists
	} else if !errors.Is(err, ErrGearNotFound) {
		return err
	}

	now := s.now().UnixMilli()
	gear.CreatedAt = now
	gear.UpdatedAt = now
	return s.writeGearLocked(ctx, gear, nil)
}

func (s *Store) List(ctx context.Context, opts ListOptions) ([]Gear, error) {
	items := make([]Gear, 0)
	for entry, err := range s.kv.List(ctx, gearsPrefix()) {
		if err != nil {
			return nil, fmt.Errorf("gears: list: %w", err)
		}
		var gear Gear
		if err := msgpack.Unmarshal(entry.Value, &gear); err != nil {
			return nil, fmt.Errorf("gears: decode list %s: %w", entry.Key.String(), err)
		}
		items = append(items, gear)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt == items[j].CreatedAt {
			return items[i].PublicKey < items[j].PublicKey
		}
		return items[i].CreatedAt < items[j].CreatedAt
	})
	if opts.Limit > 0 && len(items) > opts.Limit {
		items = items[:opts.Limit]
	}
	return items, nil
}

func (s *Store) ResolveBySN(ctx context.Context, sn string) (string, error) {
	return s.resolveSingle(ctx, snKey(sn), ErrGearNotFound)
}

func (s *Store) ResolveByIMEI(ctx context.Context, tac, serial string) (string, error) {
	return s.resolveSingle(ctx, imeiKey(tac, serial), ErrGearNotFound)
}

func (s *Store) ListByLabel(ctx context.Context, key, value string) ([]Gear, error) {
	return s.listByReferencePrefix(ctx, labelPrefix(GearLabel{Key: key, Value: value}))
}

func (s *Store) ListByCertification(ctx context.Context, certType GearCertificationType, authority GearCertificationAuthority, id string) ([]Gear, error) {
	return s.listByReferencePrefix(ctx, certificationPrefix(GearCertification{
		Type:      certType,
		Authority: authority,
		ID:        id,
	}))
}

func (s *Store) ListByFirmware(ctx context.Context, depot string, channel GearFirmwareChannel) ([]Gear, error) {
	return s.listByReferencePrefix(ctx, firmwarePrefix(depot, channel))
}

func (s *Store) ListByRole(ctx context.Context, role GearRole) ([]Gear, error) {
	return s.listByReferencePrefix(ctx, rolePrefix(role))
}

func (s *Store) ListByStatus(ctx context.Context, status GearStatus) ([]Gear, error) {
	return s.listByReferencePrefix(ctx, statusPrefix(status))
}

func (s *Store) Close() error {
	return s.kv.Close()
}

func (s *Store) writeGearLocked(ctx context.Context, gear Gear, previous *Gear) error {
	data, err := msgpack.Marshal(&gear)
	if err != nil {
		return fmt.Errorf("gears: encode %s: %w", gear.PublicKey, err)
	}

	var deletes []kv.Key
	if previous != nil {
		deletes = append(deletes, indexKeys(snapshotIndexes(*previous), previous.PublicKey)...)
	}

	entries := []kv.Entry{{Key: gearKey(gear.PublicKey), Value: data}}
	entries = append(entries, indexEntries(snapshotIndexes(gear), gear.PublicKey)...)

	if len(deletes) > 0 {
		if err := s.kv.BatchDelete(ctx, deletes); err != nil {
			return fmt.Errorf("gears: delete stale indexes %s: %w", gear.PublicKey, err)
		}
	}
	if err := s.kv.BatchSet(ctx, entries); err != nil {
		return fmt.Errorf("gears: write %s: %w", gear.PublicKey, err)
	}
	return nil
}

func (s *Store) resolveSingle(ctx context.Context, key kv.Key, notFound error) (string, error) {
	data, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return "", notFound
		}
		return "", err
	}
	return string(data), nil
}

func (s *Store) listByReferencePrefix(ctx context.Context, prefix kv.Key) ([]Gear, error) {
	publicKeys := make([]string, 0)
	for entry, err := range s.kv.List(ctx, prefix) {
		if err != nil {
			return nil, err
		}
		if len(entry.Key) == 0 {
			continue
		}
		publicKey := entry.Key[len(entry.Key)-1]
		publicKeys = append(publicKeys, publicKey)
	}
	sort.Strings(publicKeys)

	items := make([]Gear, 0, len(publicKeys))
	seen := make(map[string]struct{}, len(publicKeys))
	for _, publicKey := range publicKeys {
		if _, ok := seen[publicKey]; ok {
			continue
		}
		seen[publicKey] = struct{}{}
		gear, err := s.Get(ctx, publicKey)
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

func indexEntries(snapshot indexSnapshot, publicKey string) []kv.Entry {
	entries := make([]kv.Entry, 0, 2+len(snapshot.imeis)+len(snapshot.certifications)+len(snapshot.labels))
	if snapshot.sn != "" {
		entries = append(entries, kv.Entry{Key: snKey(snapshot.sn), Value: []byte(publicKey)})
	}
	for _, item := range snapshot.imeis {
		entries = append(entries,
			kv.Entry{Key: imeiKey(item.TAC, item.Serial), Value: []byte(publicKey)},
			kv.Entry{Key: imeiTACKey(item.TAC, publicKey), Value: []byte{1}},
		)
	}
	for _, cert := range snapshot.certifications {
		entries = append(entries, kv.Entry{Key: certificationKey(cert, publicKey), Value: []byte{1}})
	}
	for _, label := range snapshot.labels {
		entries = append(entries, kv.Entry{Key: labelKey(label, publicKey), Value: []byte{1}})
	}
	if snapshot.depot != "" && snapshot.channel != "" {
		entries = append(entries, kv.Entry{Key: firmwareKey(snapshot.depot, snapshot.channel, publicKey), Value: []byte{1}})
	}
	if snapshot.role != "" {
		entries = append(entries, kv.Entry{Key: roleKey(snapshot.role, publicKey), Value: []byte{1}})
	}
	if snapshot.status != "" {
		entries = append(entries, kv.Entry{Key: statusKey(snapshot.status, publicKey), Value: []byte{1}})
	}
	return entries
}

func indexKeys(snapshot indexSnapshot, publicKey string) []kv.Key {
	keys := make([]kv.Key, 0, 2+len(snapshot.imeis)+len(snapshot.certifications)+len(snapshot.labels))
	if snapshot.sn != "" {
		keys = append(keys, snKey(snapshot.sn))
	}
	for _, item := range snapshot.imeis {
		keys = append(keys, imeiKey(item.TAC, item.Serial), imeiTACKey(item.TAC, publicKey))
	}
	for _, cert := range snapshot.certifications {
		keys = append(keys, certificationKey(cert, publicKey))
	}
	for _, label := range snapshot.labels {
		keys = append(keys, labelKey(label, publicKey))
	}
	if snapshot.depot != "" && snapshot.channel != "" {
		keys = append(keys, firmwareKey(snapshot.depot, snapshot.channel, publicKey))
	}
	if snapshot.role != "" {
		keys = append(keys, roleKey(snapshot.role, publicKey))
	}
	if snapshot.status != "" {
		keys = append(keys, statusKey(snapshot.status, publicKey))
	}
	return keys
}

func optionalGear(gear Gear, err error) *Gear {
	if err != nil {
		return nil
	}
	cp := gear
	return &cp
}
