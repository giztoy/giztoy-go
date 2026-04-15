package firmware

import (
	"errors"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
)

func TestValidateVersionOrder(t *testing.T) {
	t.Parallel()

	depot := adminservice.Depot{
		Stable:  adminservice.DepotRelease{FirmwareSemver: "1.0.0"},
		Beta:    adminservice.DepotRelease{FirmwareSemver: "1.1.0"},
		Testing: adminservice.DepotRelease{FirmwareSemver: "1.2.0"},
	}
	if err := validateVersionOrder(depot); err != nil {
		t.Fatalf("validateVersionOrder() unexpected error: %v", err)
	}

	depot.Beta.FirmwareSemver = "0.9.0"
	if err := validateVersionOrder(depot); !errors.Is(err, errVersionOrderViolation) {
		t.Fatalf("validateVersionOrder() error = %v, want %v", err, errVersionOrderViolation)
	}

	depot.Beta.FirmwareSemver = "1.1.0"
	depot.Testing.FirmwareSemver = "1.0.9"
	if err := validateVersionOrder(depot); !errors.Is(err, errVersionOrderViolation) {
		t.Fatalf("validateVersionOrder() error = %v, want %v", err, errVersionOrderViolation)
	}
}

func TestCompareSemVer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "major", a: "2.0.0", b: "1.9.9", want: 1},
		{name: "minor", a: "1.2.0", b: "1.10.0", want: -1},
		{name: "patch", a: "1.2.3", b: "1.2.2", want: 1},
		{name: "prerelease", a: "1.0.0-alpha", b: "1.0.0", want: -1},
		{name: "build metadata ignored", a: "1.0.0+abc", b: "1.0.0+xyz", want: 0},
		{name: "fallback invalid", a: "bad", b: "also-bad", want: 1},
		{name: "equal", a: "1.2.3", b: "1.2.3", want: 0},
		{name: "invalid rhs fallback", a: "1.2.3", b: "bad", want: -1},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := compareSemVer(tc.a, tc.b); got != tc.want {
				t.Fatalf("compareSemVer(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestParseSemVer(t *testing.T) {
	t.Parallel()

	major, minor, patch, prerelease, err := parseSemVer("1.2.3-alpha.1+build")
	if err != nil {
		t.Fatalf("parseSemVer() unexpected error: %v", err)
	}
	if major != 1 || minor != 2 || patch != 3 {
		t.Fatalf("parseSemVer() numbers = %d.%d.%d", major, minor, patch)
	}
	if len(prerelease) != 2 || prerelease[0] != "alpha" || prerelease[1] != "1" {
		t.Fatalf("parseSemVer() prerelease = %#v", prerelease)
	}

	badValues := []string{"1.2", "01.2.3", "1.2.-1", "1.2.3-"}
	for _, value := range badValues {
		if _, _, _, _, err := parseSemVer(value); err == nil {
			t.Fatalf("parseSemVer(%q) expected error", value)
		}
	}
}

func TestSemVerHelpers(t *testing.T) {
	t.Parallel()

	if _, err := parseSemVerNumber("01"); err == nil {
		t.Fatal("parseSemVerNumber should reject leading zero")
	}
	if _, err := parseSemVerNumber("3"); err != nil {
		t.Fatalf("parseSemVerNumber unexpected error: %v", err)
	}
	if _, err := parseSemVerNumber("999999999999999999999999"); err == nil {
		t.Fatal("parseSemVerNumber should reject overflow")
	}
	if _, err := parseSemVerNumber("x"); err == nil {
		t.Fatal("parseSemVerNumber should reject non-numeric")
	}

	if err := validateSemVerIdentifiers("alpha.1"); err != nil {
		t.Fatalf("validateSemVerIdentifiers unexpected error: %v", err)
	}
	if err := validateSemVerIdentifiers(""); err == nil {
		t.Fatal("validateSemVerIdentifiers expected error")
	}
	if err := validateSemVerIdentifiers("alpha.!"); err == nil {
		t.Fatal("validateSemVerIdentifiers should reject invalid identifier")
	}

	if !isValidSemVerIdentifier("alpha-1") {
		t.Fatal("isValidSemVerIdentifier expected true")
	}
	if isValidSemVerIdentifier("") {
		t.Fatal("isValidSemVerIdentifier empty should be false")
	}
	if isValidSemVerIdentifier("alpha!") {
		t.Fatal("isValidSemVerIdentifier expected false")
	}

	if got := comparePrerelease([]string{"1"}, []string{"alpha"}); got != -1 {
		t.Fatalf("comparePrerelease numeric vs alpha = %d, want -1", got)
	}
	if got := comparePrerelease([]string{"alpha"}, []string{"1"}); got != 1 {
		t.Fatalf("comparePrerelease alpha vs numeric = %d, want 1", got)
	}
	if got := comparePrerelease(nil, nil); got != 0 {
		t.Fatalf("comparePrerelease empty vs empty = %d, want 0", got)
	}
	if got := comparePrerelease(nil, []string{"alpha"}); got != 1 {
		t.Fatalf("comparePrerelease release vs prerelease = %d, want 1", got)
	}
	if got := comparePrerelease([]string{"alpha"}, nil); got != -1 {
		t.Fatalf("comparePrerelease prerelease vs release = %d, want -1", got)
	}
	if got := comparePrerelease([]string{"alpha", "1"}, []string{"alpha", "2"}); got != -1 {
		t.Fatalf("comparePrerelease alpha.1 vs alpha.2 = %d, want -1", got)
	}
	if got := comparePrerelease([]string{"alpha", "2"}, []string{"alpha", "1"}); got != 1 {
		t.Fatalf("comparePrerelease alpha.2 vs alpha.1 = %d, want 1", got)
	}
	if got := comparePrerelease([]string{"alpha"}, []string{"beta"}); got != -1 {
		t.Fatalf("comparePrerelease alpha vs beta = %d, want -1", got)
	}
	if got := comparePrerelease([]string{"alpha"}, []string{"alpha"}); got != 0 {
		t.Fatalf("comparePrerelease equal = %d, want 0", got)
	}
	if got := comparePrerelease([]string{"alpha"}, []string{"alpha", "1"}); got != -1 {
		t.Fatalf("comparePrerelease shorter = %d, want -1", got)
	}
	if got := comparePrerelease([]string{"alpha", "1"}, []string{"alpha"}); got != 1 {
		t.Fatalf("comparePrerelease longer = %d, want 1", got)
	}

	if value, n, ok := parseNumericIdentifier("42"); !ok || value != "42" || n != 42 {
		t.Fatalf("parseNumericIdentifier numeric = (%q, %d, %v)", value, n, ok)
	}
	if _, _, ok := parseNumericIdentifier("01"); ok {
		t.Fatal("parseNumericIdentifier should reject leading zero numeric")
	}
	if _, _, ok := parseNumericIdentifier("alpha"); ok {
		t.Fatal("parseNumericIdentifier alpha should be non-numeric")
	}
	if _, _, ok := parseNumericIdentifier("999999999999999999999999"); ok {
		t.Fatal("parseNumericIdentifier overflow should be non-numeric")
	}
}
