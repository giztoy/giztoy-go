package firmware

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"

	"github.com/GizClaw/gizclaw-go/pkg/store/depotstore"
)

var (
	errDepotNotFound         = errors.New("firmware: depot not found")
	errChannelNotFound       = errors.New("firmware: channel not found")
	errFirmwareNotFound      = errors.New("firmware: firmware not found")
	errInvalidPath           = errors.New("firmware: invalid path")
	errVersionOrderViolation = errors.New("firmware: version order violation")
)

func isValidChannel(channel Channel) bool {
	return channel.Valid()
}

func allChannels() []Channel {
	return []Channel{Rollback, Stable, Beta, Testing}
}

func depotRelease(depot apitypes.Depot, channel Channel) (apitypes.DepotRelease, bool) {
	switch channel {
	case Rollback:
		return depot.Rollback, depot.Rollback.FirmwareSemver != ""
	case Stable:
		return depot.Stable, depot.Stable.FirmwareSemver != ""
	case Beta:
		return depot.Beta, depot.Beta.FirmwareSemver != ""
	case Testing:
		return depot.Testing, depot.Testing.FirmwareSemver != ""
	default:
		return apitypes.DepotRelease{}, false
	}
}

func setDepotRelease(depot *apitypes.Depot, channel Channel, release apitypes.DepotRelease) {
	switch channel {
	case Rollback:
		depot.Rollback = release
	case Stable:
		depot.Stable = release
	case Beta:
		depot.Beta = release
	case Testing:
		depot.Testing = release
	}
}

func normalizeDepotInfo(info apitypes.DepotInfo) apitypes.DepotInfo {
	files := infoFiles(info)
	if len(files) == 0 {
		return apitypes.DepotInfo{}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return apitypes.DepotInfo{Files: &files}
}

func normalizeDepotRelease(release apitypes.DepotRelease) apitypes.DepotRelease {
	files := releaseFiles(release)
	if len(files) > 0 {
		sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
		release.Files = &files
	} else {
		release.Files = nil
	}
	return release
}

func infoFiles(info apitypes.DepotInfo) []apitypes.DepotInfoFile {
	if info.Files == nil {
		return nil
	}
	out := append([]apitypes.DepotInfoFile(nil), (*info.Files)...)
	return out
}

func releaseFiles(release apitypes.DepotRelease) []apitypes.DepotFile {
	if release.Files == nil {
		return nil
	}
	out := append([]apitypes.DepotFile(nil), (*release.Files)...)
	return out
}

func releaseChannel(release apitypes.DepotRelease) Channel {
	if release.Channel == nil {
		return ""
	}
	return Channel(*release.Channel)
}

func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	out := v
	return &out
}

func sameInfoFiles(info apitypes.DepotInfo, release apitypes.DepotRelease) bool {
	infoList := infoFiles(normalizeDepotInfo(info))
	releaseList := releaseFiles(normalizeDepotRelease(release))
	if len(infoList) != len(releaseList) {
		return false
	}
	for i := range infoList {
		if infoList[i].Path != releaseList[i].Path {
			return false
		}
	}
	return true
}

func parseInfo(data []byte) (apitypes.DepotInfo, error) {
	var info apitypes.DepotInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return apitypes.DepotInfo{}, err
	}
	if err := validateDepotInfo(info); err != nil {
		return apitypes.DepotInfo{}, err
	}
	return normalizeDepotInfo(info), nil
}

func validateDepotInfo(info apitypes.DepotInfo) error {
	seen := map[string]struct{}{}
	for _, file := range infoFiles(info) {
		if err := validateRelativePath(file.Path); err != nil {
			return fmt.Errorf("info.json path %q: %w", file.Path, err)
		}
		if _, ok := seen[file.Path]; ok {
			return fmt.Errorf("info.json duplicate path %q", file.Path)
		}
		seen[file.Path] = struct{}{}
	}
	return nil
}

func writeInfo(store depotstore.Store, path string, info apitypes.DepotInfo) error {
	info = normalizeDepotInfo(info)
	if err := validateDepotInfo(info); err != nil {
		return err
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return store.WriteFile(path, data)
}

func parseManifest(data []byte) (apitypes.DepotRelease, error) {
	var release apitypes.DepotRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return apitypes.DepotRelease{}, err
	}
	if err := validateRelease(release); err != nil {
		return apitypes.DepotRelease{}, err
	}
	return normalizeDepotRelease(release), nil
}

func validateRelease(release apitypes.DepotRelease) error {
	if _, _, _, _, err := parseSemVer(release.FirmwareSemver); err != nil {
		return fmt.Errorf("invalid firmware_semver %q", release.FirmwareSemver)
	}
	channel := releaseChannel(release)
	if !isValidChannel(channel) {
		return fmt.Errorf("invalid channel %q", channel)
	}
	seen := map[string]struct{}{}
	for _, file := range releaseFiles(release) {
		if err := validateRelativePath(file.Path); err != nil {
			return fmt.Errorf("manifest path %q: %w", file.Path, err)
		}
		if _, ok := seen[file.Path]; ok {
			return fmt.Errorf("manifest duplicate path %q", file.Path)
		}
		seen[file.Path] = struct{}{}
	}
	return nil
}

func validateReleaseAgainstFiles(store depotstore.Store, root string, release apitypes.DepotRelease) error {
	if err := validateRelease(release); err != nil {
		return err
	}
	for _, file := range releaseFiles(release) {
		fullPath := path.Join(root, file.Path)
		data, err := store.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", file.Path, err)
		}
		shaSum := sha256.Sum256(data)
		md5Sum := md5.Sum(data)
		if hex.EncodeToString(shaSum[:]) != file.Sha256 {
			return fmt.Errorf("sha256 mismatch for %s", file.Path)
		}
		if hex.EncodeToString(md5Sum[:]) != file.Md5 {
			return fmt.Errorf("md5 mismatch for %s", file.Path)
		}
	}
	return nil
}

func writeManifest(store depotstore.Store, path string, release apitypes.DepotRelease) error {
	release = normalizeDepotRelease(release)
	if err := validateRelease(release); err != nil {
		return err
	}
	data, err := json.MarshalIndent(release, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return store.WriteFile(path, data)
}

func validateDepotName(depot string) error {
	if depot == "" {
		return fmt.Errorf("firmware: empty depot name")
	}
	if strings.Contains(depot, `\`) {
		return fmt.Errorf("firmware: invalid depot name %q", depot)
	}
	if path.IsAbs(depot) || strings.HasPrefix(depot, "/") {
		return fmt.Errorf("firmware: invalid depot name %q", depot)
	}
	if cleaned := path.Clean(depot); cleaned != depot {
		return fmt.Errorf("firmware: invalid depot name %q", depot)
	}
	for _, part := range strings.Split(depot, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("firmware: invalid depot name %q", depot)
		}
	}
	return nil
}

func validateRelativePath(p string) error {
	if p == "" {
		return errInvalidPath
	}
	if strings.HasPrefix(p, "/") {
		return errInvalidPath
	}
	clean := path.Clean(p)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, `\`) {
		return errInvalidPath
	}
	if strings.Contains(clean, "/../") {
		return errInvalidPath
	}
	return nil
}
