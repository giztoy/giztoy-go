package firmware

import (
	"errors"
	"fmt"
	"io/fs"
	"path"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
)

func (s *Server) releaseDepot(depot string) (adminservice.Depot, error) {
	unlock := s.lockDepot(depot)
	defer unlock()
	if err := s.validateDepot(depot); err != nil {
		return adminservice.Depot{}, err
	}
	paths := map[string]string{
		string(Rollback): s.channelPath(depot, string(Rollback)),
		string(Stable):   s.channelPath(depot, string(Stable)),
		string(Beta):     s.channelPath(depot, string(Beta)),
		string(Testing):  s.channelPath(depot, string(Testing)),
	}
	if _, err := s.store().Stat(paths[string(Beta)]); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return adminservice.Depot{}, fmt.Errorf("%w: %s", errChannelNotFound, Beta)
		}
		return adminservice.Depot{}, err
	}
	if _, err := s.store().Stat(paths[string(Testing)]); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return adminservice.Depot{}, fmt.Errorf("%w: %s", errChannelNotFound, Testing)
		}
		return adminservice.Depot{}, err
	}
	backups, restore, err := s.prepareSwitch(
		depot,
		paths,
		map[string]string{
			string(Stable):   string(Beta),
			string(Beta):     string(Testing),
			string(Rollback): string(Stable),
		},
	)
	if err != nil {
		return adminservice.Depot{}, err
	}
	commit := false
	defer func() {
		if !commit {
			_ = restore()
		}
	}()
	for _, channel := range []Channel{Rollback, Stable, Beta} {
		if err := s.rewriteManifestChannel(depot, channel); err != nil && !errors.Is(err, errChannelNotFound) {
			return adminservice.Depot{}, err
		}
	}
	snapshot, err := s.scanDepot(depot)
	if err != nil {
		return adminservice.Depot{}, err
	}
	commit = true
	s.cleanupBackups(backups)
	return snapshot, nil
}

func (s *Server) rollbackDepot(depot string) (adminservice.Depot, error) {
	unlock := s.lockDepot(depot)
	defer unlock()
	if err := s.validateDepot(depot); err != nil {
		return adminservice.Depot{}, err
	}
	rollbackPath := s.channelPath(depot, string(Rollback))
	stablePath := s.channelPath(depot, string(Stable))
	if _, err := s.store().Stat(rollbackPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return adminservice.Depot{}, fmt.Errorf("%w: %s", errChannelNotFound, Rollback)
		}
		return adminservice.Depot{}, err
	}
	backups, restore, err := s.prepareSwitch(
		depot,
		map[string]string{
			string(Stable):   stablePath,
			string(Rollback): rollbackPath,
		},
		map[string]string{
			string(Stable):   string(Rollback),
			string(Rollback): string(Stable),
		},
	)
	if err != nil {
		return adminservice.Depot{}, err
	}
	commit := false
	defer func() {
		if !commit {
			_ = restore()
		}
	}()
	if err := s.rewriteManifestChannel(depot, Stable); err != nil {
		return adminservice.Depot{}, err
	}
	if err := s.rewriteManifestChannel(depot, Rollback); err != nil && !errors.Is(err, errChannelNotFound) {
		return adminservice.Depot{}, err
	}
	snapshot, err := s.scanDepot(depot)
	if err != nil {
		return adminservice.Depot{}, err
	}
	commit = true
	s.cleanupBackups(backups)
	return snapshot, nil
}

func (s *Server) prepareSwitch(depot string, paths map[string]string, layout map[string]string) (map[string]string, func() error, error) {
	backups := make(map[string]string, len(paths))
	applied := make(map[string]string, len(layout))
	for channel, currentPath := range paths {
		backupPath := path.Join(s.depotPath(depot), ".bak-"+channel)
		_ = s.store().RemoveAll(backupPath)
		if _, err := s.store().Stat(currentPath); err == nil {
			if err := s.store().Rename(currentPath, backupPath); err != nil {
				return nil, nil, err
			}
			backups[channel] = backupPath
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, nil, err
		}
	}
	restore := func() error {
		var firstErr error
		for target, source := range applied {
			if err := s.store().Rename(paths[target], backups[source]); err != nil && !errors.Is(err, fs.ErrNotExist) && firstErr == nil {
				firstErr = err
			}
		}
		for channel, backupPath := range backups {
			if err := s.store().Rename(backupPath, paths[channel]); err != nil && !errors.Is(err, fs.ErrNotExist) && firstErr == nil {
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
		if err := s.store().Rename(sourcePath, paths[target]); err != nil {
			_ = restore()
			return nil, nil, err
		}
		applied[target] = source
	}
	return backups, restore, nil
}

func (s *Server) rewriteManifestChannel(depot string, channel Channel) error {
	data, err := s.store().ReadFile(s.manifestPath(depot, string(channel)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return errChannelNotFound
		}
		return err
	}
	release, err := parseManifest(data)
	if err != nil {
		return err
	}
	release.Channel = stringPtr(string(channel))
	return writeManifest(s.store(), s.manifestPath(depot, string(channel)), release)
}

func (s *Server) cleanupBackups(backups map[string]string) {
	for _, backupPath := range backups {
		_ = s.store().RemoveAll(backupPath)
	}
}
