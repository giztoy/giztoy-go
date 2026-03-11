package firmware

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Switcher struct {
	store   *Store
	scanner *Scanner
}

func NewSwitcher(store *Store, scanner *Scanner) *Switcher {
	return &Switcher{store: store, scanner: scanner}
}

func (s *Switcher) Rollback(depot string) (Depot, error) {
	unlock := s.store.LockDepot(depot)
	defer unlock()

	if err := s.store.ValidateDepot(depot); err != nil {
		return Depot{}, err
	}
	rollbackPath := s.store.ChannelPath(depot, ChannelRollback)
	stablePath := s.store.ChannelPath(depot, ChannelStable)
	if _, err := os.Stat(rollbackPath); err != nil {
		if os.IsNotExist(err) {
			return Depot{}, fmt.Errorf("%w: %s", ErrChannelNotFound, ChannelRollback)
		}
		return Depot{}, err
	}
	paths := map[Channel]string{
		ChannelStable:   stablePath,
		ChannelRollback: rollbackPath,
	}
	backups, restore, err := s.prepareSwitch(depot, paths, map[Channel]Channel{
		ChannelStable:   ChannelRollback,
		ChannelRollback: ChannelStable,
	})
	if err != nil {
		return Depot{}, err
	}
	commit := false
	defer func() {
		if !commit {
			_ = restore()
		}
	}()
	if err := s.rewriteManifestChannel(depot, ChannelStable); err != nil {
		return Depot{}, err
	}
	if err := s.rewriteManifestChannel(depot, ChannelRollback); err != nil && !errors.Is(err, ErrChannelNotFound) {
		return Depot{}, err
	}
	snapshot, err := s.scanner.ScanDepot(depot)
	if err != nil {
		return Depot{}, err
	}
	commit = true
	s.cleanupBackups(backups)
	return snapshot, nil
}

func (s *Switcher) Release(depot string) (Depot, error) {
	unlock := s.store.LockDepot(depot)
	defer unlock()

	if err := s.store.ValidateDepot(depot); err != nil {
		return Depot{}, err
	}
	paths := map[Channel]string{
		ChannelRollback: s.store.ChannelPath(depot, ChannelRollback),
		ChannelStable:   s.store.ChannelPath(depot, ChannelStable),
		ChannelBeta:     s.store.ChannelPath(depot, ChannelBeta),
		ChannelTesting:  s.store.ChannelPath(depot, ChannelTesting),
	}
	if _, err := os.Stat(paths[ChannelBeta]); err != nil {
		if os.IsNotExist(err) {
			return Depot{}, fmt.Errorf("%w: %s", ErrChannelNotFound, ChannelBeta)
		}
		return Depot{}, err
	}
	if _, err := os.Stat(paths[ChannelTesting]); err != nil {
		if os.IsNotExist(err) {
			return Depot{}, fmt.Errorf("%w: %s", ErrChannelNotFound, ChannelTesting)
		}
		return Depot{}, err
	}
	backups, restore, err := s.prepareSwitch(depot, paths, map[Channel]Channel{
		ChannelStable:   ChannelBeta,
		ChannelBeta:     ChannelTesting,
		ChannelRollback: ChannelStable,
	})
	if err != nil {
		return Depot{}, err
	}
	commit := false
	defer func() {
		if !commit {
			_ = restore()
		}
	}()
	for _, channel := range []Channel{ChannelRollback, ChannelStable, ChannelBeta} {
		if err := s.rewriteManifestChannel(depot, channel); err != nil && !errors.Is(err, ErrChannelNotFound) {
			return Depot{}, err
		}
	}
	snapshot, err := s.scanner.ScanDepot(depot)
	if err != nil {
		return Depot{}, err
	}
	commit = true
	s.cleanupBackups(backups)
	return snapshot, nil
}

func (s *Switcher) prepareSwitch(depot string, paths map[Channel]string, layout map[Channel]Channel) (map[Channel]string, func() error, error) {
	backups := make(map[Channel]string, len(paths))
	applied := make(map[Channel]Channel, len(layout))
	for channel, currentPath := range paths {
		backupPath := filepath.Join(s.store.DepotPath(depot), ".bak-"+string(channel))
		_ = os.RemoveAll(backupPath)
		if _, err := os.Stat(currentPath); err == nil {
			if err := os.Rename(currentPath, backupPath); err != nil {
				return nil, nil, err
			}
			backups[channel] = backupPath
			continue
		} else if !os.IsNotExist(err) {
			return nil, nil, err
		}
	}

	restore := func() error {
		var firstErr error
		for target, source := range applied {
			if err := os.Rename(paths[target], backups[source]); err != nil && !os.IsNotExist(err) && firstErr == nil {
				firstErr = err
			}
		}
		for channel, backupPath := range backups {
			if err := os.Rename(backupPath, paths[channel]); err != nil && !os.IsNotExist(err) && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}

	for target, source := range layout {
		sourcePath, ok := backups[source]
		if !ok {
			continue
		}
		if err := os.Rename(sourcePath, paths[target]); err != nil {
			_ = restore()
			return nil, nil, err
		}
		applied[target] = source
	}
	return backups, restore, nil
}

func (s *Switcher) cleanupBackups(backups map[Channel]string) {
	for _, backupPath := range backups {
		_ = os.RemoveAll(backupPath)
	}
}

func (s *Switcher) rewriteManifestChannel(depot string, channel Channel) error {
	path := s.store.ManifestPath(depot, channel)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrChannelNotFound
		}
		return err
	}
	release, err := ParseManifest(data)
	if err != nil {
		return err
	}
	release.Channel = string(channel)
	return WriteManifest(path, release)
}
