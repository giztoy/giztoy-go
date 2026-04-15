package firmware

import (
	"encoding/json"
	"errors"
	"path"
	"strings"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
)

func TestManifestChannelHelpers(t *testing.T) {
	t.Parallel()

	channels := allChannels()
	if len(channels) != 4 {
		t.Fatalf("allChannels() len = %d", len(channels))
	}
	if !isValidChannel(Beta) || isValidChannel(Channel("dev")) {
		t.Fatal("isValidChannel() returned unexpected result")
	}

	depot := adminservice.Depot{}
	for _, channel := range []Channel{Rollback, Stable, Beta, Testing} {
		release := adminservice.DepotRelease{FirmwareSemver: "1.2.3"}
		setDepotRelease(&depot, channel, release)
		got, ok := depotRelease(depot, channel)
		if !ok || got.FirmwareSemver != "1.2.3" {
			t.Fatalf("depotRelease(%q) = (%+v, %v)", channel, got, ok)
		}
	}
	if _, ok := depotRelease(depot, Channel("dev")); ok {
		t.Fatal("depotRelease() should reject invalid channel")
	}

	if releaseChannel(adminservice.DepotRelease{}) != "" {
		t.Fatal("releaseChannel() expected empty")
	}
	if stringPtr("") != nil {
		t.Fatal("stringPtr(\"\") should be nil")
	}
	if got := stringPtr("x"); got == nil || *got != "x" {
		t.Fatalf("stringPtr(\"x\") = %#v", got)
	}
}

func TestNormalizeDepotHelpers(t *testing.T) {
	t.Parallel()

	info := adminservice.DepotInfo{
		Files: &[]adminservice.DepotInfoFile{{Path: "b.bin"}, {Path: "a.bin"}},
	}
	normalizedInfo := normalizeDepotInfo(info)
	if (*normalizedInfo.Files)[0].Path != "a.bin" {
		t.Fatalf("normalizeDepotInfo() did not sort: %#v", *normalizedInfo.Files)
	}
	infoFilesCopy := infoFiles(normalizedInfo)
	infoFilesCopy[0].Path = "mutated"
	if (*normalizedInfo.Files)[0].Path != "a.bin" {
		t.Fatal("infoFiles() should return copy")
	}

	release := adminservice.DepotRelease{
		FirmwareSemver: "1.0.0",
		Files: &[]adminservice.DepotFile{
			{Path: "b.bin"},
			{Path: "a.bin"},
		},
	}
	normalizedRelease := normalizeDepotRelease(release)
	if (*normalizedRelease.Files)[0].Path != "a.bin" {
		t.Fatalf("normalizeDepotRelease() did not sort: %#v", *normalizedRelease.Files)
	}
	releaseFilesCopy := releaseFiles(normalizedRelease)
	releaseFilesCopy[0].Path = "mutated"
	if (*normalizedRelease.Files)[0].Path != "a.bin" {
		t.Fatal("releaseFiles() should return copy")
	}
	if normalizeDepotRelease(adminservice.DepotRelease{}).Files != nil {
		t.Fatal("normalizeDepotRelease empty should omit files")
	}
	if releaseFiles(adminservice.DepotRelease{}) != nil {
		t.Fatal("releaseFiles empty should be nil")
	}
	if normalizeDepotInfo(adminservice.DepotInfo{}).Files != nil {
		t.Fatal("normalizeDepotInfo empty should omit files")
	}
}

func TestInfoAndManifestParsing(t *testing.T) {
	t.Parallel()

	info := depotInfo("firmware.bin")
	infoData, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal info: %v", err)
	}
	parsedInfo, err := parseInfo(infoData)
	if err != nil {
		t.Fatalf("parseInfo() unexpected error: %v", err)
	}
	if !sameInfoFiles(parsedInfo, depotReleaseForFiles(Beta, "1.0.0", map[string]string{"firmware.bin": "fw"})) {
		t.Fatal("sameInfoFiles() expected true")
	}
	if sameInfoFiles(parsedInfo, depotReleaseForFiles(Beta, "1.0.0", map[string]string{"other.bin": "fw"})) {
		t.Fatal("sameInfoFiles() expected false")
	}
	if sameInfoFiles(parsedInfo, depotReleaseForFiles(Beta, "1.0.0", map[string]string{"firmware.bin": "fw", "extra.bin": "x"})) {
		t.Fatal("sameInfoFiles() expected false for extra file")
	}
	if _, err := parseInfo([]byte("{")); err == nil {
		t.Fatal("parseInfo() expected JSON error")
	}
	infoBadData, err := json.Marshal(depotInfo("../bad"))
	if err != nil {
		t.Fatalf("marshal bad info: %v", err)
	}
	if _, err := parseInfo(infoBadData); err == nil {
		t.Fatal("parseInfo() expected validation error")
	}
	if err := validateDepotInfo(depotInfo("a.bin", "a.bin")); err == nil {
		t.Fatal("validateDepotInfo() expected duplicate path error")
	}
	if err := validateDepotInfo(depotInfo("../a.bin")); err == nil {
		t.Fatal("validateDepotInfo() expected invalid path error")
	}

	release := depotReleaseForFiles(Stable, "1.2.3", map[string]string{"firmware.bin": "fw"})
	releaseData, err := json.Marshal(release)
	if err != nil {
		t.Fatalf("marshal release: %v", err)
	}
	parsedRelease, err := parseManifest(releaseData)
	if err != nil {
		t.Fatalf("parseManifest() unexpected error: %v", err)
	}
	if releaseChannel(parsedRelease) != Stable {
		t.Fatalf("releaseChannel() = %q", releaseChannel(parsedRelease))
	}
	if _, err := parseManifest([]byte("{")); err == nil {
		t.Fatal("parseManifest() expected JSON error")
	}
	badManifestData, err := json.Marshal(func() adminservice.DepotRelease {
		bad := release
		bad.Channel = stringPtr("dev")
		return bad
	}())
	if err != nil {
		t.Fatalf("marshal bad manifest: %v", err)
	}
	if _, err := parseManifest(badManifestData); err == nil {
		t.Fatal("parseManifest() expected validation error")
	}

	badRelease := release
	badRelease.Channel = stringPtr("dev")
	if err := validateRelease(badRelease); err == nil {
		t.Fatal("validateRelease() expected invalid channel error")
	}
	badRelease = release
	badRelease.FirmwareSemver = "bad"
	if err := validateRelease(badRelease); err == nil {
		t.Fatal("validateRelease() expected invalid semver error")
	}
	badRelease = release
	badRelease.Files = &[]adminservice.DepotFile{{Path: "dup"}, {Path: "dup"}}
	if err := validateRelease(badRelease); err == nil {
		t.Fatal("validateRelease() expected duplicate path error")
	}
}

func TestManifestWriteAndValidation(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	info := depotInfo("fw.bin")
	if err := writeInfo(env.store, "depot/info.json", info); err != nil {
		t.Fatalf("writeInfo() unexpected error: %v", err)
	}
	if !strings.HasSuffix(string(env.readFile("depot/info.json")), "\n") {
		t.Fatal("writeInfo() should end with newline")
	}

	files := map[string]string{"fw.bin": "firmware"}
	release := depotReleaseForFiles(Stable, "1.2.3", files)
	for name, content := range files {
		env.writeFile(path.Join("depot", "stable", name), content)
	}
	if err := writeManifest(env.store, "depot/stable/manifest.json", release); err != nil {
		t.Fatalf("writeManifest() unexpected error: %v", err)
	}
	if !strings.HasSuffix(string(env.readFile("depot/stable/manifest.json")), "\n") {
		t.Fatal("writeManifest() should end with newline")
	}
	if err := validateReleaseAgainstFiles(env.store, "depot/stable", release); err != nil {
		t.Fatalf("validateReleaseAgainstFiles() unexpected error: %v", err)
	}

	env.writeFile("depot/stable/fw.bin", "modified")
	if err := validateReleaseAgainstFiles(env.store, "depot/stable", release); err == nil {
		t.Fatal("validateReleaseAgainstFiles() expected hash mismatch")
	}
	if err := validateReleaseAgainstFiles(env.store, "depot/missing", release); err == nil {
		t.Fatal("validateReleaseAgainstFiles() expected read error")
	}
	invalidRelease := release
	invalidRelease.Channel = stringPtr("dev")
	if err := validateReleaseAgainstFiles(env.store, "depot/stable", invalidRelease); err == nil {
		t.Fatal("validateReleaseAgainstFiles() expected validation error")
	}

	badStore := newMockStore(t)
	badStore.writeFile = func(name string, data []byte) error { return errors.New("boom") }
	if err := writeInfo(badStore, "depot/info.json", info); err == nil {
		t.Fatal("writeInfo() expected store error")
	}
	if err := writeManifest(badStore, "depot/stable/manifest.json", release); err == nil {
		t.Fatal("writeManifest() expected store error")
	}
	if err := writeInfo(env.store, "depot/info.json", depotInfo("../bad")); err == nil {
		t.Fatal("writeInfo() expected validation error")
	}
	badRelease := depotReleaseForFiles(Channel("dev"), "1.0.0", map[string]string{"fw.bin": "firmware"})
	if err := writeManifest(env.store, "depot/stable/manifest.json", badRelease); err == nil {
		t.Fatal("writeManifest() expected validation error")
	}
}

func TestValidateDepotAndRelativePath(t *testing.T) {
	t.Parallel()

	goodDepots := []string{"a", "a/b"}
	for _, depot := range goodDepots {
		if err := validateDepotName(depot); err != nil {
			t.Fatalf("validateDepotName(%q) unexpected error: %v", depot, err)
		}
	}

	badDepots := []string{"", "/abs", "../x", "a/../b", `a\b`}
	for _, depot := range badDepots {
		if err := validateDepotName(depot); err == nil {
			t.Fatalf("validateDepotName(%q) expected error", depot)
		}
	}

	goodPaths := []string{"a.bin", "dir/a.bin"}
	for _, p := range goodPaths {
		if err := validateRelativePath(p); err != nil {
			t.Fatalf("validateRelativePath(%q) unexpected error: %v", p, err)
		}
	}
	badPaths := []string{"", "/abs", "../a", "a/../../b", `a\b`}
	for _, p := range badPaths {
		if err := validateRelativePath(p); err == nil {
			t.Fatalf("validateRelativePath(%q) expected error", p)
		}
	}
}
