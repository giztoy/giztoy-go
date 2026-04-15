package firmware

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
)

func (s *Server) scanDepotNames() ([]string, error) {
	seen := make(map[string]struct{})
	err := s.store().WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if p == "." {
			return nil
		}
		base := path.Base(p)
		if d.IsDir() && strings.HasPrefix(base, ".") {
			return fs.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		var depotPath string
		switch base {
		case "info.json":
			depotPath = path.Dir(p)
		case "manifest.json":
			channelDir := path.Dir(p)
			if !isValidChannel(Channel(path.Base(channelDir))) {
				return nil
			}
			depotPath = path.Dir(channelDir)
		default:
			return nil
		}
		rel := path.Clean(depotPath)
		if rel == "." {
			return nil
		}
		if err := validateDepotName(rel); err != nil {
			return nil
		}
		seen[rel] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (s *Server) scanDepot(name string) (adminservice.Depot, error) {
	if err := s.validateDepot(name); err != nil {
		return adminservice.Depot{}, err
	}
	depot := adminservice.Depot{Name: name}
	if data, err := s.store().ReadFile(s.infoPath(name)); err == nil {
		info, err := parseInfo(data)
		if err != nil {
			return adminservice.Depot{}, fmt.Errorf("scan depot info %s: %w", name, err)
		}
		depot.Info = normalizeDepotInfo(info)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return adminservice.Depot{}, err
	}
	for _, channel := range allChannels() {
		release, err := s.scanRelease(name, channel)
		if err != nil {
			if errors.Is(err, errChannelNotFound) {
				continue
			}
			return adminservice.Depot{}, err
		}
		setDepotRelease(&depot, channel, release)
	}
	if err := validateVersionOrder(depot); err != nil {
		return adminservice.Depot{}, err
	}
	return depot, nil
}

func (s *Server) scanRelease(depot string, channel Channel) (adminservice.DepotRelease, error) {
	data, err := s.store().ReadFile(s.manifestPath(depot, string(channel)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return adminservice.DepotRelease{}, errChannelNotFound
		}
		return adminservice.DepotRelease{}, err
	}
	release, err := parseManifest(data)
	if err != nil {
		return adminservice.DepotRelease{}, fmt.Errorf("scan manifest %s/%s: %w", depot, channel, err)
	}
	if releaseChannel(release) != channel {
		return adminservice.DepotRelease{}, fmt.Errorf("scan manifest %s/%s: channel mismatch", depot, channel)
	}
	if err := validateReleaseAgainstFiles(s.store(), s.channelPath(depot, string(channel)), release); err != nil {
		return adminservice.DepotRelease{}, err
	}
	return normalizeDepotRelease(release), nil
}

func (s *Server) resolveOTA(depotName string, channel Channel) (gearservice.OTASummary, error) {
	depot, err := s.scanDepot(depotName)
	if err != nil {
		return gearservice.OTASummary{}, err
	}
	release, ok := depotRelease(depot, channel)
	if !ok {
		return gearservice.OTASummary{}, errFirmwareNotFound
	}
	files := make([]gearservice.DepotFile, 0, len(releaseFiles(release)))
	for _, file := range releaseFiles(release) {
		files = append(files, gearservice.DepotFile{
			Md5:    file.Md5,
			Path:   file.Path,
			Sha256: file.Sha256,
		})
	}
	return gearservice.OTASummary{
		Channel:        string(channel),
		Depot:          depotName,
		Files:          files,
		FirmwareSemver: release.FirmwareSemver,
	}, nil
}
