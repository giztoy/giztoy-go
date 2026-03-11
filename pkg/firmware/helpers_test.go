package firmware

import (
	"archive/tar"
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestValidationHelpers(t *testing.T) {
	if _, err := ParseInfo([]byte("{")); err == nil {
		t.Fatal("ParseInfo should reject invalid json")
	}
	if err := ValidateInfo(DepotInfo{Files: []DepotInfoFile{{Path: "a.bin"}, {Path: "a.bin"}}}); err == nil {
		t.Fatal("ValidateInfo should reject duplicate paths")
	}
	for _, path := range []string{"", "/abs", "../bad", `a\\b`} {
		if err := validateRelativePath(path); err == nil {
			t.Fatalf("validateRelativePath(%q) should fail", path)
		}
	}
	items := []string{"c", "a", "b"}
	sortStrings(items)
	if items[0] != "a" || items[2] != "c" {
		t.Fatalf("sortStrings = %v", items)
	}

	if CompareSemVer("1.0.0", "1.0.0") != 0 {
		t.Fatal("equal semver should compare to zero")
	}
	if CompareSemVer("1.0.0", "1.2.0") >= 0 {
		t.Fatal("lower semver should compare negative")
	}
	if CompareSemVer("1.0.0-rc.1", "1.0.0") >= 0 {
		t.Fatal("prerelease should compare lower than stable")
	}
	if CompareSemVer("1.0.0+build.2", "1.0.0+build.1") != 0 {
		t.Fatal("build metadata should not affect precedence")
	}
	if CompareSemVer("1.0.0-rc.2", "1.0.0-rc.10") >= 0 {
		t.Fatal("numeric prerelease identifiers should compare numerically")
	}

	if _, err := ParseManifest([]byte(`{"firmware_semver":"bad","channel":"stable","files":[]}`)); err == nil {
		t.Fatal("ParseManifest should reject invalid semver")
	}
	if _, err := ParseManifest([]byte(`{"firmware_semver":"1.0.0-rc.1+build.5","channel":"stable","files":[]}`)); err != nil {
		t.Fatalf("ParseManifest should accept prerelease/build semver: %v", err)
	}
	if err := ValidateRelease(DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        "invalid",
	}); err == nil {
		t.Fatal("ValidateRelease should reject invalid channel")
	}
	if err := ValidateRelease(DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(ChannelStable),
		Files:          []DepotFile{{Path: "a.bin"}, {Path: "a.bin"}},
	}); err == nil {
		t.Fatal("ValidateRelease should reject duplicate paths")
	}

	dir := t.TempDir()
	data := []byte("firmware")
	if err := os.WriteFile(filepath.Join(dir, "a.bin"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	sum256 := sha256.Sum256(data)
	sumMD5 := md5.Sum(data)
	release := DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(ChannelStable),
		Files: []DepotFile{{
			Path:   "a.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}
	if err := ValidateReleaseAgainstFiles(dir, release); err != nil {
		t.Fatalf("ValidateReleaseAgainstFiles error: %v", err)
	}
	if err := WriteManifest(filepath.Join(dir, "manifest.json"), release); err != nil {
		t.Fatalf("WriteManifest error: %v", err)
	}
	if err := WriteManifest(filepath.Join(dir, "bad.json"), DepotRelease{FirmwareSemVer: "bad", Channel: string(ChannelStable)}); err == nil {
		t.Fatal("WriteManifest should reject invalid release")
	}
	release.Files[0].SHA256 = "bad"
	if err := ValidateReleaseAgainstFiles(dir, release); err == nil {
		t.Fatal("ValidateReleaseAgainstFiles should fail on checksum mismatch")
	}
}

func TestStoreValidateDepotAndScannerEdges(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	scanner := NewScanner(store)

	for _, depot := range []string{"", "../escape", "demo/../escape", "/abs", "demo//nested", `demo\windows`} {
		if err := store.EnsureDepot(depot); err == nil {
			t.Fatalf("EnsureDepot(%q) should fail", depot)
		}
		if err := store.ValidateDepot(depot); err == nil {
			t.Fatalf("ValidateDepot(%q) should fail", depot)
		}
	}

	if err := store.ValidateDepot("missing"); !errors.Is(err, ErrDepotNotFound) {
		t.Fatalf("ValidateDepot missing err = %v", err)
	}

	fileDepot := filepath.Join(root, "file-depot")
	if err := os.WriteFile(fileDepot, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.ValidateDepot("file-depot"); err == nil {
		t.Fatalf("ValidateDepot file err = %v", err)
	}

	if err := os.MkdirAll(filepath.Join(root, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "visible"), 0o755); err != nil {
		t.Fatal(err)
	}
	items, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("Scan items = %+v", items)
	}

	nestedStore := NewStore(t.TempDir())
	nestedScanner := NewScanner(nestedStore)
	nestedUploader := NewUploader(nestedStore, nestedScanner)
	payload := []byte("nested-fw")
	sum256 := sha256.Sum256(payload)
	sumMD5 := md5.Sum(payload)
	if err := nestedUploader.PutInfo("demo/main", DepotInfo{Files: []DepotInfoFile{{Path: "firmware.bin"}}}); err != nil {
		t.Fatalf("nested PutInfo error: %v", err)
	}
	if _, err := nestedUploader.UploadTar("demo/main", ChannelStable, bytes.NewReader(buildReleaseTar(t, DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(ChannelStable),
		Files: []DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload}))); err != nil {
		t.Fatalf("nested UploadTar error: %v", err)
	}
	items, err = nestedScanner.Scan()
	if err != nil {
		t.Fatalf("nested Scan error: %v", err)
	}
	if len(items) != 1 || items[0].Name != "demo/main" {
		t.Fatalf("nested Scan items = %+v", items)
	}
}

func TestExtractTarAndSwitcherErrors(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	scanner := NewScanner(store)
	uploader := NewUploader(store, scanner)
	switcher := NewSwitcher(store, scanner)

	if _, err := extractTar(t.TempDir(), ChannelStable, bytes.NewReader(nil)); err == nil {
		t.Fatal("extractTar should require manifest")
	}

	var dirTar bytes.Buffer
	tw := tar.NewWriter(&dirTar)
	if err := tw.WriteHeader(&tar.Header{Name: "dir", Typeflag: tar.TypeDir}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := extractTar(t.TempDir(), ChannelStable, bytes.NewReader(dirTar.Bytes())); err == nil {
		t.Fatal("extractTar should reject non-regular entries")
	}

	if err := uploader.PutInfo("demo", DepotInfo{Files: []DepotInfoFile{{Path: "firmware.bin"}}}); err != nil {
		t.Fatalf("PutInfo error: %v", err)
	}
	if _, err := uploader.UploadTar("demo", Channel("invalid"), bytes.NewReader(nil)); err == nil {
		t.Fatal("UploadTar should reject invalid channel")
	}
	if _, err := uploader.UploadTar("demo", ChannelStable, bytes.NewReader(buildReleaseTar(t, DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(ChannelBeta),
		Files:          []DepotFile{{Path: "firmware.bin", SHA256: "bad", MD5: "bad"}},
	}, map[string][]byte{"firmware.bin": []byte("x")}))); err == nil {
		t.Fatal("UploadTar should reject channel mismatch")
	}

	if _, err := switcher.Rollback("demo"); err == nil {
		t.Fatal("Rollback should fail when rollback channel missing")
	}
	if _, err := switcher.Release("demo"); err == nil {
		t.Fatal("Release should fail when beta/testing missing")
	}
}

func TestResolveAndVersionOrderErrors(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	scanner := NewScanner(store)
	uploader := NewUploader(store, scanner)
	ota := NewOTAService(store, scanner)

	if _, err := os.Stat(root); err != nil {
		t.Fatal(err)
	}
	if _, err := ota.Resolve("missing", ChannelStable); err == nil {
		t.Fatal("Resolve should fail for missing depot")
	}
	if _, _, err := ota.ResolveFile("missing", ChannelStable, "firmware.bin"); err == nil {
		t.Fatal("ResolveFile should fail for missing depot")
	}
	if err := uploader.PutInfo("missing-file", DepotInfo{Files: []DepotInfoFile{{Path: "firmware.bin"}}}); err != nil {
		t.Fatal(err)
	}
	payloadMissing := []byte("fw")
	sum256Missing := sha256.Sum256(payloadMissing)
	sumMD5Missing := md5.Sum(payloadMissing)
	if _, err := uploader.UploadTar("missing-file", ChannelStable, bytes.NewReader(buildReleaseTar(t, DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(ChannelStable),
		Files: []DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256Missing[:]),
			MD5:    hex.EncodeToString(sumMD5Missing[:]),
		}},
	}, map[string][]byte{"firmware.bin": payloadMissing}))); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ota.ResolveFile("missing-file", ChannelStable, "other.bin"); err == nil {
		t.Fatal("ResolveFile should fail for file absent from manifest")
	}

	if err := store.EnsureDepot("demo"); err != nil {
		t.Fatal(err)
	}
	payload := []byte("fw")
	shaSum := sha256.Sum256(payload)
	md5Sum := md5.Sum(payload)
	stablePath := store.ChannelPath("demo", ChannelStable)
	betaPath := store.ChannelPath("demo", ChannelBeta)
	if err := os.MkdirAll(stablePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(betaPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(store.ManifestPath("demo", ChannelStable), DepotRelease{
		FirmwareSemVer: "2.0.0",
		Channel:        string(ChannelStable),
		Files:          []DepotFile{{Path: "firmware.bin", SHA256: hex.EncodeToString(shaSum[:]), MD5: hex.EncodeToString(md5Sum[:])}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stablePath, "firmware.bin"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(store.ManifestPath("demo", ChannelBeta), DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(ChannelBeta),
		Files:          []DepotFile{{Path: "firmware.bin", SHA256: hex.EncodeToString(shaSum[:]), MD5: hex.EncodeToString(md5Sum[:])}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(betaPath, "firmware.bin"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scanner.ScanDepot("demo"); err == nil {
		t.Fatal("ScanDepot should reject invalid version order")
	}

	prereleaseDepot := "prerelease-order"
	if err := store.EnsureDepot(prereleaseDepot); err != nil {
		t.Fatal(err)
	}
	stablePath = store.ChannelPath(prereleaseDepot, ChannelStable)
	betaPath = store.ChannelPath(prereleaseDepot, ChannelBeta)
	if err := os.MkdirAll(stablePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(betaPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(store.ManifestPath(prereleaseDepot, ChannelStable), DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(ChannelStable),
		Files:          []DepotFile{{Path: "firmware.bin", SHA256: hex.EncodeToString(shaSum[:]), MD5: hex.EncodeToString(md5Sum[:])}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stablePath, "firmware.bin"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(store.ManifestPath(prereleaseDepot, ChannelBeta), DepotRelease{
		FirmwareSemVer: "1.0.0-rc.1",
		Channel:        string(ChannelBeta),
		Files:          []DepotFile{{Path: "firmware.bin", SHA256: hex.EncodeToString(shaSum[:]), MD5: hex.EncodeToString(md5Sum[:])}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(betaPath, "firmware.bin"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scanner.ScanDepot(prereleaseDepot); err == nil {
		t.Fatal("ScanDepot should reject prerelease beta older than stable")
	}
}

func TestRewriteManifestChannel(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	scanner := NewScanner(store)
	switcher := NewSwitcher(store, scanner)
	payload := []byte("fw")
	sum256 := sha256.Sum256(payload)
	sumMD5 := md5.Sum(payload)

	if err := store.EnsureDepot("demo"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store.ChannelPath("demo", ChannelStable), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.ChannelPath("demo", ChannelStable), "firmware.bin"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(store.ManifestPath("demo", ChannelStable), DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(ChannelRollback),
		Files: []DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := switcher.rewriteManifestChannel("demo", ChannelStable); err != nil {
		t.Fatalf("rewriteManifestChannel error: %v", err)
	}
	data, err := os.ReadFile(store.ManifestPath("demo", ChannelStable))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := ParseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Channel != string(ChannelStable) {
		t.Fatalf("manifest channel = %q", manifest.Channel)
	}
}

func TestReleaseAndRollbackWithoutExistingStable(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	scanner := NewScanner(store)
	uploader := NewUploader(store, scanner)
	switcher := NewSwitcher(store, scanner)

	if err := uploader.PutInfo("demo", DepotInfo{Files: []DepotInfoFile{{Path: "firmware.bin"}}}); err != nil {
		t.Fatal(err)
	}

	makeTar := func(version string, channel Channel) []byte {
		payload := []byte(version)
		sum256 := sha256.Sum256(payload)
		sumMD5 := md5.Sum(payload)
		return buildReleaseTar(t, DepotRelease{
			FirmwareSemVer: version,
			Channel:        string(channel),
			Files: []DepotFile{{
				Path:   "firmware.bin",
				SHA256: hex.EncodeToString(sum256[:]),
				MD5:    hex.EncodeToString(sumMD5[:]),
			}},
		}, map[string][]byte{"firmware.bin": payload})
	}

	if _, err := uploader.UploadTar("demo", ChannelBeta, bytes.NewReader(makeTar("1.1.0", ChannelBeta))); err != nil {
		t.Fatal(err)
	}
	if _, err := uploader.UploadTar("demo", ChannelTesting, bytes.NewReader(makeTar("1.2.0", ChannelTesting))); err != nil {
		t.Fatal(err)
	}
	depot, err := switcher.Release("demo")
	if err != nil {
		t.Fatalf("Release error: %v", err)
	}
	if depot.Stable.FirmwareSemVer != "1.1.0" || depot.Beta.FirmwareSemVer != "1.2.0" {
		t.Fatalf("unexpected depot after release: %+v", depot)
	}

	root2 := t.TempDir()
	store2 := NewStore(root2)
	scanner2 := NewScanner(store2)
	uploader2 := NewUploader(store2, scanner2)
	switcher2 := NewSwitcher(store2, scanner2)
	if err := uploader2.PutInfo("demo", DepotInfo{Files: []DepotInfoFile{{Path: "firmware.bin"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := uploader2.UploadTar("demo", ChannelRollback, bytes.NewReader(makeTar("0.9.0", ChannelRollback))); err != nil {
		t.Fatal(err)
	}
	depot, err = switcher2.Rollback("demo")
	if err != nil {
		t.Fatalf("Rollback error: %v", err)
	}
	if depot.Stable.FirmwareSemVer != "0.9.0" {
		t.Fatalf("unexpected depot after rollback: %+v", depot)
	}
}

func TestReleaseAndRollbackRestoreLayoutOnFailure(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	scanner := NewScanner(store)
	uploader := NewUploader(store, scanner)
	switcher := NewSwitcher(store, scanner)

	if err := uploader.PutInfo("demo", DepotInfo{Files: []DepotInfoFile{{Path: "firmware.bin"}}}); err != nil {
		t.Fatal(err)
	}
	makeTar := func(version string, channel Channel) []byte {
		payload := []byte(version)
		sum256 := sha256.Sum256(payload)
		sumMD5 := md5.Sum(payload)
		return buildReleaseTar(t, DepotRelease{
			FirmwareSemVer: version,
			Channel:        string(channel),
			Files: []DepotFile{{
				Path:   "firmware.bin",
				SHA256: hex.EncodeToString(sum256[:]),
				MD5:    hex.EncodeToString(sumMD5[:]),
			}},
		}, map[string][]byte{"firmware.bin": payload})
	}
	if _, err := uploader.UploadTar("demo", ChannelStable, bytes.NewReader(makeTar("1.0.0", ChannelStable))); err != nil {
		t.Fatal(err)
	}
	if _, err := uploader.UploadTar("demo", ChannelBeta, bytes.NewReader(makeTar("1.1.0", ChannelBeta))); err != nil {
		t.Fatal(err)
	}
	if _, err := uploader.UploadTar("demo", ChannelTesting, bytes.NewReader(makeTar("1.2.0", ChannelTesting))); err != nil {
		t.Fatal(err)
	}
	if _, err := uploader.UploadTar("demo", ChannelRollback, bytes.NewReader(makeTar("0.9.0", ChannelRollback))); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(store.ManifestPath("demo", ChannelBeta), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := switcher.Release("demo"); err == nil {
		t.Fatal("Release should fail when promoted manifest is invalid")
	}
	if _, err := os.Stat(store.ChannelPath("demo", ChannelStable)); err != nil {
		t.Fatalf("stable path missing after failed release: %v", err)
	}
	if _, err := os.Stat(store.ChannelPath("demo", ChannelBeta)); err != nil {
		t.Fatalf("beta path missing after failed release: %v", err)
	}
	if _, err := os.Stat(store.ChannelPath("demo", ChannelTesting)); err != nil {
		t.Fatalf("testing path missing after failed release: %v", err)
	}
	if _, err := os.Stat(store.ChannelPath("demo", ChannelRollback)); err != nil {
		t.Fatalf("rollback path missing after failed release: %v", err)
	}

	root2 := t.TempDir()
	store2 := NewStore(root2)
	scanner2 := NewScanner(store2)
	uploader2 := NewUploader(store2, scanner2)
	switcher2 := NewSwitcher(store2, scanner2)
	if err := uploader2.PutInfo("demo", DepotInfo{Files: []DepotInfoFile{{Path: "firmware.bin"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := uploader2.UploadTar("demo", ChannelStable, bytes.NewReader(makeTar("1.0.0", ChannelStable))); err != nil {
		t.Fatal(err)
	}
	if _, err := uploader2.UploadTar("demo", ChannelRollback, bytes.NewReader(makeTar("0.9.0", ChannelRollback))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store2.ManifestPath("demo", ChannelRollback), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := switcher2.Rollback("demo"); err == nil {
		t.Fatal("Rollback should fail when rollback manifest is invalid")
	}
	if _, err := os.Stat(store2.ChannelPath("demo", ChannelStable)); err != nil {
		t.Fatalf("stable path missing after failed rollback: %v", err)
	}
	if _, err := os.Stat(store2.ChannelPath("demo", ChannelRollback)); err != nil {
		t.Fatalf("rollback path missing after failed rollback: %v", err)
	}
}

func TestPutInfoMismatchAndScanReleaseChannelMismatch(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	scanner := NewScanner(store)
	uploader := NewUploader(store, scanner)
	payload := []byte("fw")
	sum256 := sha256.Sum256(payload)
	sumMD5 := md5.Sum(payload)

	if err := uploader.PutInfo("demo", DepotInfo{Files: []DepotInfoFile{{Path: "firmware.bin"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := uploader.UploadTar("demo", ChannelStable, bytes.NewReader(buildReleaseTar(t, DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(ChannelStable),
		Files: []DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload}))); err != nil {
		t.Fatal(err)
	}
	if err := uploader.PutInfo("demo", DepotInfo{Files: []DepotInfoFile{{Path: "other.bin"}}}); err == nil {
		t.Fatal("PutInfo should reject info/release mismatch")
	}

	depot := "mismatch"
	if err := store.EnsureDepot(depot); err != nil {
		t.Fatal(err)
	}
	channelPath := store.ChannelPath(depot, ChannelBeta)
	if err := os.MkdirAll(channelPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(channelPath, "firmware.bin"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(store.ManifestPath(depot, ChannelBeta), DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(ChannelStable),
		Files: []DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := scanner.scanRelease(depot, ChannelBeta); err == nil {
		t.Fatal("scanRelease should reject channel mismatch")
	}
}

func TestJSONManifestEscapesFileNames(t *testing.T) {
	release := DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(ChannelStable),
		Files: []DepotFile{{
			Path:   `fw"prod.bin`,
			SHA256: "sha",
			MD5:    "md5",
		}},
	}
	data, err := jsonManifest(release)
	if err != nil {
		t.Fatalf("jsonManifest error: %v", err)
	}
	var decoded DepotRelease
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("jsonManifest should produce valid JSON: %v", err)
	}
	if decoded.Files[0].Path != `fw"prod.bin` {
		t.Fatalf("decoded path = %q", decoded.Files[0].Path)
	}
}

func TestResolveFileMissingOnDisk(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	scanner := NewScanner(store)
	ota := NewOTAService(store, scanner)
	payload := []byte("fw")
	sum256 := sha256.Sum256(payload)
	sumMD5 := md5.Sum(payload)

	if err := store.EnsureDepot("demo"); err != nil {
		t.Fatal(err)
	}
	channelPath := store.ChannelPath("demo", ChannelStable)
	if err := os.MkdirAll(channelPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(store.ManifestPath("demo", ChannelStable), DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(ChannelStable),
		Files: []DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ota.ResolveFile("demo", ChannelStable, "firmware.bin"); err == nil {
		t.Fatalf("ResolveFile err = %v", err)
	}
}
