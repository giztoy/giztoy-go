package firmware

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Scanner struct {
	store *Store
}

func NewScanner(store *Store) *Scanner {
	return &Scanner{store: store}
}

func (s *Scanner) Scan() ([]Depot, error) {
	depotNames, err := s.scanDepotNames()
	if err != nil {
		return nil, err
	}
	depots := make([]Depot, 0, len(depotNames))
	for _, name := range depotNames {
		depot, err := s.ScanDepot(name)
		if err != nil {
			return nil, err
		}
		depots = append(depots, depot)
	}
	sort.Slice(depots, func(i, j int) bool { return depots[i].Name < depots[j].Name })
	return depots, nil
}

func (s *Scanner) scanDepotNames() ([]string, error) {
	root := s.store.Root()
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	seen := make(map[string]struct{})
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		base := filepath.Base(path)
		if d.IsDir() && strings.HasPrefix(base, ".") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		var depotPath string
		switch base {
		case "info.json":
			depotPath = filepath.Dir(path)
		case "manifest.json":
			channelDir := filepath.Dir(path)
			channel := Channel(filepath.Base(channelDir))
			if !IsValidChannel(channel) || channel == "" {
				return nil
			}
			depotPath = filepath.Dir(channelDir)
		default:
			return nil
		}

		rel, err := filepath.Rel(root, depotPath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
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

func (s *Scanner) ScanDepot(name string) (Depot, error) {
	if err := s.store.ValidateDepot(name); err != nil {
		return Depot{}, err
	}

	depot := Depot{Name: name}
	if data, err := os.ReadFile(s.store.InfoPath(name)); err == nil {
		info, err := ParseInfo(data)
		if err != nil {
			return Depot{}, fmt.Errorf("scan depot info %s: %w", name, err)
		}
		depot.Info = DepotInfo{Files: normalizeInfoPaths(info.Files)}
	} else if !os.IsNotExist(err) {
		return Depot{}, err
	}

	for _, channel := range []Channel{ChannelRollback, ChannelStable, ChannelBeta, ChannelTesting} {
		release, err := s.scanRelease(name, channel)
		if err != nil {
			if err == ErrChannelNotFound {
				continue
			}
			return Depot{}, err
		}
		depot.SetRelease(channel, release)
	}

	if err := validateVersionOrder(depot); err != nil {
		return Depot{}, err
	}
	return depot, nil
}

func (s *Scanner) scanRelease(depot string, channel Channel) (DepotRelease, error) {
	path := s.store.ManifestPath(depot, channel)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DepotRelease{}, ErrChannelNotFound
		}
		return DepotRelease{}, err
	}
	release, err := ParseManifest(data)
	if err != nil {
		return DepotRelease{}, fmt.Errorf("scan manifest %s/%s: %w", depot, channel, err)
	}
	if Channel(release.Channel) != channel {
		return DepotRelease{}, fmt.Errorf("scan manifest %s/%s: channel mismatch", depot, channel)
	}
	root := filepath.Dir(path)
	if err := ValidateReleaseAgainstFiles(root, release); err != nil {
		return DepotRelease{}, err
	}
	release.Files = sortReleaseFiles(release.Files)
	return release, nil
}

func validateVersionOrder(depot Depot) error {
	stable, stableOK := depot.Release(ChannelStable)
	beta, betaOK := depot.Release(ChannelBeta)
	testing, testingOK := depot.Release(ChannelTesting)

	if stableOK && betaOK && CompareSemVer(beta.FirmwareSemVer, stable.FirmwareSemVer) < 0 {
		return ErrVersionOrderViolation
	}
	if betaOK && testingOK && CompareSemVer(testing.FirmwareSemVer, beta.FirmwareSemVer) < 0 {
		return ErrVersionOrderViolation
	}
	return nil
}
