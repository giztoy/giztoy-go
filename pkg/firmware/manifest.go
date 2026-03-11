package firmware

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func ParseManifest(data []byte) (DepotRelease, error) {
	var release DepotRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return DepotRelease{}, err
	}
	if err := ValidateRelease(release); err != nil {
		return DepotRelease{}, err
	}
	return release, nil
}

func ValidateRelease(release DepotRelease) error {
	if _, err := parseSemVer(release.FirmwareSemVer); err != nil {
		return fmt.Errorf("invalid firmware_semver %q", release.FirmwareSemVer)
	}
	if !IsValidChannel(Channel(release.Channel)) {
		return fmt.Errorf("invalid channel %q", release.Channel)
	}
	seen := map[string]struct{}{}
	for _, file := range release.Files {
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

func ValidateReleaseAgainstFiles(root string, release DepotRelease) error {
	if err := ValidateRelease(release); err != nil {
		return err
	}
	for _, file := range release.Files {
		fullPath := filepath.Join(root, file.Path)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", file.Path, err)
		}
		shaSum := sha256.Sum256(data)
		md5Sum := md5.Sum(data)
		if hex.EncodeToString(shaSum[:]) != file.SHA256 {
			return fmt.Errorf("sha256 mismatch for %s", file.Path)
		}
		if hex.EncodeToString(md5Sum[:]) != file.MD5 {
			return fmt.Errorf("md5 mismatch for %s", file.Path)
		}
	}
	return nil
}

func WriteManifest(path string, release DepotRelease) error {
	if err := ValidateRelease(release); err != nil {
		return err
	}
	data, err := json.MarshalIndent(release, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func releasePath(root, depot string, channel Channel) string {
	return filepath.Join(root, depot, string(channel))
}

func manifestPath(root, depot string, channel Channel) string {
	return filepath.Join(releasePath(root, depot, channel), "manifest.json")
}

func sortReleaseFiles(files []DepotFile) []DepotFile {
	out := make([]DepotFile, len(files))
	copy(out, files)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func CompareSemVer(a, b string) int {
	av, err := parseSemVer(a)
	if err != nil {
		return strings.Compare(a, b)
	}
	bv, err := parseSemVer(b)
	if err != nil {
		return strings.Compare(a, b)
	}
	switch {
	case av.major != bv.major:
		if av.major < bv.major {
			return -1
		}
		return 1
	case av.minor != bv.minor:
		if av.minor < bv.minor {
			return -1
		}
		return 1
	case av.patch != bv.patch:
		if av.patch < bv.patch {
			return -1
		}
		return 1
	}
	return comparePrerelease(av.prerelease, bv.prerelease)
}

type semVer struct {
	major      int
	minor      int
	patch      int
	prerelease []semVerIdentifier
}

type semVerIdentifier struct {
	raw      string
	numeric  bool
	intValue int
}

func parseSemVer(value string) (semVer, error) {
	core := value
	if idx := strings.IndexByte(value, '+'); idx >= 0 {
		if err := validateSemVerIdentifiers(value[idx+1:]); err != nil {
			return semVer{}, err
		}
		core = value[:idx]
	}

	corePart, prePart, hasPre := strings.Cut(core, "-")
	parts := strings.Split(corePart, ".")
	if len(parts) != 3 {
		return semVer{}, fmt.Errorf("invalid semver")
	}
	major, err := parseSemVerNumber(parts[0])
	if err != nil {
		return semVer{}, err
	}
	minor, err := parseSemVerNumber(parts[1])
	if err != nil {
		return semVer{}, err
	}
	patch, err := parseSemVerNumber(parts[2])
	if err != nil {
		return semVer{}, err
	}

	out := semVer{major: major, minor: minor, patch: patch}
	if hasPre {
		idents, err := parsePrerelease(prePart)
		if err != nil {
			return semVer{}, err
		}
		out.prerelease = idents
	}
	return out, nil
}

func parseSemVerNumber(value string) (int, error) {
	if value == "" {
		return 0, fmt.Errorf("invalid semver")
	}
	if len(value) > 1 && value[0] == '0' {
		return 0, fmt.Errorf("invalid semver")
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid semver")
	}
	return n, nil
}

func validateSemVerIdentifiers(value string) error {
	if value == "" {
		return fmt.Errorf("invalid semver")
	}
	for _, ident := range strings.Split(value, ".") {
		if !isValidSemVerIdentifier(ident) {
			return fmt.Errorf("invalid semver")
		}
	}
	return nil
}

func parsePrerelease(value string) ([]semVerIdentifier, error) {
	if err := validateSemVerIdentifiers(value); err != nil {
		return nil, err
	}
	parts := strings.Split(value, ".")
	out := make([]semVerIdentifier, 0, len(parts))
	for _, part := range parts {
		item := semVerIdentifier{raw: part}
		numeric := true
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				numeric = false
				break
			}
		}
		if numeric {
			if len(part) > 1 && part[0] == '0' {
				return nil, fmt.Errorf("invalid semver")
			}
			n, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid semver")
			}
			item.numeric = true
			item.intValue = n
		}
		out = append(out, item)
	}
	return out, nil
}

func isValidSemVerIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch >= 'a' && ch <= 'z' {
			continue
		}
		if ch >= 'A' && ch <= 'Z' {
			continue
		}
		if ch == '-' {
			continue
		}
		return false
	}
	return true
}

func comparePrerelease(a, b []semVerIdentifier) int {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 {
		return 1
	}
	if len(b) == 0 {
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		switch {
		case a[i].numeric && b[i].numeric:
			if a[i].intValue < b[i].intValue {
				return -1
			}
			if a[i].intValue > b[i].intValue {
				return 1
			}
		case a[i].numeric:
			return -1
		case b[i].numeric:
			return 1
		default:
			if a[i].raw < b[i].raw {
				return -1
			}
			if a[i].raw > b[i].raw {
				return 1
			}
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}
