package firmware

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
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

func (s *Server) scanDepot(name string) (apitypes.Depot, error) {
	if err := s.validateDepot(name); err != nil {
		return apitypes.Depot{}, err
	}
	depot := apitypes.Depot{Name: name}
	if data, err := s.store().ReadFile(s.infoPath(name)); err == nil {
		info, err := parseInfo(data)
		if err != nil {
			return apitypes.Depot{}, fmt.Errorf("scan depot info %s: %w", name, err)
		}
		depot.Info = normalizeDepotInfo(info)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return apitypes.Depot{}, err
	}
	for _, channel := range allChannels() {
		release, err := s.scanRelease(name, channel)
		if err != nil {
			if errors.Is(err, errChannelNotFound) {
				continue
			}
			return apitypes.Depot{}, err
		}
		setDepotRelease(&depot, channel, release)
	}
	if err := validateVersionOrder(depot); err != nil {
		return apitypes.Depot{}, err
	}
	return depot, nil
}

func (s *Server) scanRelease(depot string, channel Channel) (apitypes.DepotRelease, error) {
	data, err := s.store().ReadFile(s.manifestPath(depot, string(channel)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return apitypes.DepotRelease{}, errChannelNotFound
		}
		return apitypes.DepotRelease{}, err
	}
	release, err := parseManifest(data)
	if err != nil {
		return apitypes.DepotRelease{}, fmt.Errorf("scan manifest %s/%s: %w", depot, channel, err)
	}
	if releaseChannel(release) != channel {
		return apitypes.DepotRelease{}, fmt.Errorf("scan manifest %s/%s: channel mismatch", depot, channel)
	}
	if err := validateReleaseAgainstFiles(s.store(), s.channelPath(depot, string(channel)), release); err != nil {
		return apitypes.DepotRelease{}, err
	}
	return normalizeDepotRelease(release), nil
}

func (s *Server) resolveOTA(depotName string, channel Channel) (apitypes.OTASummary, error) {
	depot, err := s.scanDepot(depotName)
	if err != nil {
		return apitypes.OTASummary{}, err
	}
	release, ok := depotRelease(depot, channel)
	if !ok {
		return apitypes.OTASummary{}, errFirmwareNotFound
	}
	files := make([]apitypes.DepotFile, 0, len(releaseFiles(release)))
	for _, file := range releaseFiles(release) {
		files = append(files, apitypes.DepotFile{
			Md5:    file.Md5,
			Path:   file.Path,
			Sha256: file.Sha256,
		})
	}
	return apitypes.OTASummary{
		Channel:        string(channel),
		Depot:          depotName,
		Files:          files,
		FirmwareSemver: release.FirmwareSemver,
	}, nil
}
