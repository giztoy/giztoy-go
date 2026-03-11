package firmware

import (
	"archive/tar"
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

func TestUploaderAndOTAFlow(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	scanner := NewScanner(store)
	uploader := NewUploader(store, scanner)
	switcher := NewSwitcher(store, scanner)
	ota := NewOTAService(store, scanner)

	info := DepotInfo{Files: []DepotInfoFile{{Path: "firmware.bin"}}}
	if err := uploader.PutInfo("demo", info); err != nil {
		t.Fatalf("PutInfo error: %v", err)
	}

	payload := []byte("firmware-v1")
	sum256 := sha256.Sum256(payload)
	sumMD5 := md5.Sum(payload)
	releaseTar := buildReleaseTar(t, DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(ChannelStable),
		Files: []DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload})

	if _, err := uploader.UploadTar("demo", ChannelStable, bytes.NewReader(releaseTar)); err != nil {
		t.Fatalf("UploadTar stable error: %v", err)
	}

	otaSummary, err := ota.Resolve("demo", ChannelStable)
	if err != nil {
		t.Fatalf("Resolve OTA error: %v", err)
	}
	if otaSummary.FirmwareSemVer != "1.0.0" {
		t.Fatalf("OTA firmware semver = %q", otaSummary.FirmwareSemVer)
	}

	fullPath, file, err := ota.ResolveFile("demo", ChannelStable, "firmware.bin")
	if err != nil {
		t.Fatalf("ResolveFile error: %v", err)
	}
	if fullPath == "" || file.Path != "firmware.bin" {
		t.Fatalf("ResolveFile returned invalid data: %q %+v", fullPath, file)
	}

	betaPayload := []byte("firmware-v2")
	sum256 = sha256.Sum256(betaPayload)
	sumMD5 = md5.Sum(betaPayload)
	betaTar := buildReleaseTar(t, DepotRelease{
		FirmwareSemVer: "1.1.0",
		Channel:        string(ChannelBeta),
		Files: []DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": betaPayload})
	if _, err := uploader.UploadTar("demo", ChannelBeta, bytes.NewReader(betaTar)); err != nil {
		t.Fatalf("UploadTar beta error: %v", err)
	}

	testingPayload := []byte("firmware-v3")
	sum256 = sha256.Sum256(testingPayload)
	sumMD5 = md5.Sum(testingPayload)
	testingTar := buildReleaseTar(t, DepotRelease{
		FirmwareSemVer: "1.2.0",
		Channel:        string(ChannelTesting),
		Files: []DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": testingPayload})
	if _, err := uploader.UploadTar("demo", ChannelTesting, bytes.NewReader(testingTar)); err != nil {
		t.Fatalf("UploadTar testing error: %v", err)
	}

	if _, err := switcher.Release("demo"); err != nil {
		t.Fatalf("Release error: %v", err)
	}
	depot, err := scanner.ScanDepot("demo")
	if err != nil {
		t.Fatalf("ScanDepot after release error: %v", err)
	}
	if depot.Stable.FirmwareSemVer != "1.1.0" || depot.Beta.FirmwareSemVer != "1.2.0" {
		t.Fatalf("unexpected promoted versions: stable=%s beta=%s", depot.Stable.FirmwareSemVer, depot.Beta.FirmwareSemVer)
	}

	if _, err := switcher.Rollback("demo"); err != nil {
		t.Fatalf("Rollback error: %v", err)
	}
	depot, err = scanner.ScanDepot("demo")
	if err != nil {
		t.Fatalf("ScanDepot after rollback error: %v", err)
	}
	if depot.Stable.FirmwareSemVer != "1.0.0" {
		t.Fatalf("stable after rollback = %q", depot.Stable.FirmwareSemVer)
	}
}

func TestScannerAndValidationHelpers(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	scanner := NewScanner(store)
	uploader := NewUploader(store, scanner)

	if err := uploader.PutInfo("demo", DepotInfo{Files: []DepotInfoFile{{Path: "../bad"}}}); err == nil {
		t.Fatal("PutInfo with invalid path should fail")
	}

	if _, err := scanner.Scan(); err != nil {
		t.Fatalf("Scan empty root error: %v", err)
	}

	if CompareSemVer("1.2.0", "1.1.9") <= 0 {
		t.Fatal("CompareSemVer expected positive result")
	}
	if _, _, err := NewOTAService(store, scanner).ResolveFile("demo", ChannelStable, "../bad"); err == nil {
		t.Fatal("ResolveFile with invalid path should fail")
	}
}

func TestUploadTarRollsBackOnFinalScanFailure(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	scanner := NewScanner(store)
	uploader := NewUploader(store, scanner)

	if err := uploader.PutInfo("demo", DepotInfo{Files: []DepotInfoFile{{Path: "firmware.bin"}}}); err != nil {
		t.Fatalf("PutInfo error: %v", err)
	}

	stableTar := buildReleaseTar(t, mustRelease(t, "2.0.0", ChannelStable, []byte("stable")), map[string][]byte{"firmware.bin": []byte("stable")})
	if _, err := uploader.UploadTar("demo", ChannelStable, bytes.NewReader(stableTar)); err != nil {
		t.Fatalf("UploadTar stable error: %v", err)
	}

	badBetaTar := buildReleaseTar(t, mustRelease(t, "1.0.0", ChannelBeta, []byte("beta-old")), map[string][]byte{"firmware.bin": []byte("beta-old")})
	if _, err := uploader.UploadTar("demo", ChannelBeta, bytes.NewReader(badBetaTar)); err == nil {
		t.Fatal("UploadTar beta should fail when version order is invalid")
	}

	depot, err := scanner.ScanDepot("demo")
	if err != nil {
		t.Fatalf("ScanDepot after rollback error: %v", err)
	}
	if depot.Stable.FirmwareSemVer != "2.0.0" {
		t.Fatalf("stable after failed upload = %q", depot.Stable.FirmwareSemVer)
	}
	if _, ok := depot.Release(ChannelBeta); ok {
		t.Fatal("beta release should not remain after failed upload")
	}
	if _, err := os.Stat(store.ChannelPath("demo", ChannelBeta)); !os.IsNotExist(err) {
		t.Fatalf("beta path should be removed, stat err = %v", err)
	}
}

func mustRelease(t *testing.T, version string, channel Channel, payload []byte) DepotRelease {
	t.Helper()
	sum256 := sha256.Sum256(payload)
	sumMD5 := md5.Sum(payload)
	return DepotRelease{
		FirmwareSemVer: version,
		Channel:        string(channel),
		Files: []DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}
}

func buildReleaseTar(t *testing.T, release DepotRelease, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	manifest, err := jsonManifest(release)
	if err != nil {
		t.Fatalf("jsonManifest error: %v", err)
	}
	writeTarFile(t, tw, "manifest.json", manifest)
	for name, data := range files {
		writeTarFile(t, tw, name, data)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close error: %v", err)
	}
	return buf.Bytes()
}

func writeTarFile(t *testing.T, tw *tar.Writer, name string, data []byte) {
	t.Helper()
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
		t.Fatalf("tar header error: %v", err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("tar write error: %v", err)
	}
}
