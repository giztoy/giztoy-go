package firmware

import (
	"fmt"
	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"strconv"
	"strings"
)

func validateVersionOrder(depot apitypes.Depot) error {
	stable, stableOK := depotRelease(depot, Stable)
	beta, betaOK := depotRelease(depot, Beta)
	testing, testingOK := depotRelease(depot, Testing)
	if stableOK && betaOK && compareSemVer(beta.FirmwareSemver, stable.FirmwareSemver) < 0 {
		return errVersionOrderViolation
	}
	if betaOK && testingOK && compareSemVer(testing.FirmwareSemver, beta.FirmwareSemver) < 0 {
		return errVersionOrderViolation
	}
	return nil
}

func compareSemVer(a, b string) int {
	amaj, amin, apat, apre, err := parseSemVer(a)
	if err != nil {
		return strings.Compare(a, b)
	}
	bmaj, bmin, bpat, bpre, err := parseSemVer(b)
	if err != nil {
		return strings.Compare(a, b)
	}
	switch {
	case amaj != bmaj:
		if amaj < bmaj {
			return -1
		}
		return 1
	case amin != bmin:
		if amin < bmin {
			return -1
		}
		return 1
	case apat != bpat:
		if apat < bpat {
			return -1
		}
		return 1
	}
	return comparePrerelease(apre, bpre)
}

func parseSemVer(value string) (int, int, int, []string, error) {
	core := value
	if idx := strings.IndexByte(value, '+'); idx >= 0 {
		if err := validateSemVerIdentifiers(value[idx+1:]); err != nil {
			return 0, 0, 0, nil, err
		}
		core = value[:idx]
	}
	corePart, prePart, hasPre := strings.Cut(core, "-")
	parts := strings.Split(corePart, ".")
	if len(parts) != 3 {
		return 0, 0, 0, nil, fmt.Errorf("invalid semver")
	}
	major, err := parseSemVerNumber(parts[0])
	if err != nil {
		return 0, 0, 0, nil, err
	}
	minor, err := parseSemVerNumber(parts[1])
	if err != nil {
		return 0, 0, 0, nil, err
	}
	patch, err := parseSemVerNumber(parts[2])
	if err != nil {
		return 0, 0, 0, nil, err
	}
	var prerelease []string
	if hasPre {
		if err := validateSemVerIdentifiers(prePart); err != nil {
			return 0, 0, 0, nil, err
		}
		prerelease = strings.Split(prePart, ".")
	}
	return major, minor, patch, prerelease, nil
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

func comparePrerelease(a, b []string) int {
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
		aNum, aInt, aNumeric := parseNumericIdentifier(a[i])
		bNum, bInt, bNumeric := parseNumericIdentifier(b[i])
		switch {
		case aNumeric && bNumeric:
			if aInt < bInt {
				return -1
			}
			if aInt > bInt {
				return 1
			}
		case aNumeric:
			return -1
		case bNumeric:
			return 1
		default:
			if aNum < bNum {
				return -1
			}
			if aNum > bNum {
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

func parseNumericIdentifier(value string) (string, int, bool) {
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return value, 0, false
		}
	}
	if len(value) > 1 && value[0] == '0' {
		return value, 0, false
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return value, 0, false
	}
	return value, n, true
}
