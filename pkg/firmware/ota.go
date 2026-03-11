package firmware

import (
	"fmt"
	"os"
	"path/filepath"
)

type OTAService struct {
	scanner *Scanner
	store   *Store
}

func NewOTAService(store *Store, scanner *Scanner) *OTAService {
	return &OTAService{store: store, scanner: scanner}
}

func (s *OTAService) Resolve(depotName string, channel Channel) (OTASummary, error) {
	depot, err := s.scanner.ScanDepot(depotName)
	if err != nil {
		return OTASummary{}, err
	}
	release, ok := depot.Release(channel)
	if !ok {
		return OTASummary{}, ErrFirmwareNotFound
	}
	return OTASummary{
		Depot:          depot.Name,
		Channel:        string(channel),
		FirmwareSemVer: release.FirmwareSemVer,
		Files:          release.Files,
	}, nil
}

func (s *OTAService) ResolveFile(depotName string, channel Channel, relativePath string) (string, DepotFile, error) {
	if err := validateRelativePath(relativePath); err != nil {
		return "", DepotFile{}, err
	}
	depot, err := s.scanner.ScanDepot(depotName)
	if err != nil {
		return "", DepotFile{}, err
	}
	release, ok := depot.Release(channel)
	if !ok {
		return "", DepotFile{}, ErrFirmwareNotFound
	}
	for _, file := range release.Files {
		if file.Path == relativePath {
			fullPath := filepath.Join(s.store.ChannelPath(depotName, channel), relativePath)
			if _, err := os.Stat(fullPath); err != nil {
				if os.IsNotExist(err) {
					return "", DepotFile{}, ErrFirmwareNotFound
				}
				return "", DepotFile{}, err
			}
			return fullPath, file, nil
		}
	}
	return "", DepotFile{}, fmt.Errorf("%w: %s", ErrFirmwareNotFound, relativePath)
}
