package firmware

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Uploader struct {
	store   *Store
	scanner *Scanner
}

func NewUploader(store *Store, scanner *Scanner) *Uploader {
	return &Uploader{store: store, scanner: scanner}
}

func (u *Uploader) PutInfo(depot string, info DepotInfo) error {
	unlock := u.store.LockDepot(depot)
	defer unlock()

	if err := u.store.EnsureDepot(depot); err != nil {
		return err
	}
	if current, err := u.scanner.ScanDepot(depot); err == nil {
		want := normalizeInfoPaths(info.Files)
		for _, channel := range []Channel{ChannelRollback, ChannelStable, ChannelBeta, ChannelTesting} {
			release, ok := current.Release(channel)
			if !ok {
				continue
			}
			if !sameInfoFiles(want, release.Files) {
				return fmt.Errorf("firmware: info files mismatch with %s", channel)
			}
		}
	}
	return WriteInfo(u.store.InfoPath(depot), info)
}

func (u *Uploader) UploadTar(depot string, channel Channel, r io.Reader) (DepotRelease, error) {
	if !IsValidChannel(channel) {
		return DepotRelease{}, fmt.Errorf("firmware: invalid channel %q", channel)
	}
	unlock := u.store.LockDepot(depot)
	defer unlock()

	if err := u.store.EnsureDepot(depot); err != nil {
		return DepotRelease{}, err
	}

	tmpPath := u.store.TempPath(depot, string(channel))
	_ = os.RemoveAll(tmpPath)
	if err := os.MkdirAll(tmpPath, 0o755); err != nil {
		return DepotRelease{}, err
	}
	defer os.RemoveAll(tmpPath)

	release, err := extractTar(tmpPath, channel, r)
	if err != nil {
		return DepotRelease{}, err
	}

	if info, err := os.ReadFile(u.store.InfoPath(depot)); err == nil {
		depotInfo, err := ParseInfo(info)
		if err != nil {
			return DepotRelease{}, err
		}
		if !sameInfoFiles(normalizeInfoPaths(depotInfo.Files), release.Files) {
			return DepotRelease{}, fmt.Errorf("firmware: info files mismatch")
		}
	} else if !os.IsNotExist(err) {
		return DepotRelease{}, err
	}

	targetPath := u.store.ChannelPath(depot, channel)
	swapPath := targetPath + ".old"
	_ = os.RemoveAll(swapPath)
	hadPrevious := false
	if _, err := os.Stat(targetPath); err == nil {
		hadPrevious = true
		if err := os.Rename(targetPath, swapPath); err != nil {
			return DepotRelease{}, err
		}
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		if hadPrevious {
			_ = os.Rename(swapPath, targetPath)
		}
		return DepotRelease{}, err
	}

	depotSnapshot, err := u.scanner.ScanDepot(depot)
	if err != nil {
		_ = os.RemoveAll(targetPath)
		if hadPrevious {
			_ = os.Rename(swapPath, targetPath)
		}
		return DepotRelease{}, err
	}
	uploaded, ok := depotSnapshot.Release(channel)
	if !ok {
		_ = os.RemoveAll(targetPath)
		if hadPrevious {
			_ = os.Rename(swapPath, targetPath)
		}
		return DepotRelease{}, ErrChannelNotFound
	}
	_ = os.RemoveAll(swapPath)
	return uploaded, nil
}

func extractTar(dst string, wantChannel Channel, r io.Reader) (DepotRelease, error) {
	tr := tar.NewReader(r)
	seen := make(map[string]struct{})
	var manifest DepotRelease
	var manifestLoaded bool
	files := make(map[string][]byte)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return DepotRelease{}, err
		}
		if hdr.Typeflag != tar.TypeReg {
			return DepotRelease{}, fmt.Errorf("firmware: illegal tar entry %s", hdr.Name)
		}
		if err := validateRelativePath(hdr.Name); err != nil {
			return DepotRelease{}, err
		}
		if _, ok := seen[hdr.Name]; ok {
			return DepotRelease{}, fmt.Errorf("firmware: duplicate tar entry %s", hdr.Name)
		}
		seen[hdr.Name] = struct{}{}

		data, err := io.ReadAll(tr)
		if err != nil {
			return DepotRelease{}, err
		}
		if hdr.Name == "manifest.json" {
			manifest, err = ParseManifest(data)
			if err != nil {
				return DepotRelease{}, err
			}
			if Channel(manifest.Channel) != wantChannel {
				return DepotRelease{}, fmt.Errorf("firmware: manifest channel mismatch")
			}
			manifestLoaded = true
			continue
		}
		files[hdr.Name] = data
	}

	if !manifestLoaded {
		return DepotRelease{}, fmt.Errorf("firmware: manifest missing")
	}

	for _, file := range manifest.Files {
		data, ok := files[file.Path]
		if !ok {
			return DepotRelease{}, fmt.Errorf("firmware: missing manifest file %s", file.Path)
		}
		target := filepath.Join(dst, file.Path)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return DepotRelease{}, err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return DepotRelease{}, err
		}
		delete(files, file.Path)
	}
	if len(files) > 0 {
		return DepotRelease{}, fmt.Errorf("firmware: tar files mismatch")
	}

	manifestBytes, _ := jsonManifest(manifest)
	if err := os.WriteFile(filepath.Join(dst, "manifest.json"), manifestBytes, 0o644); err != nil {
		return DepotRelease{}, err
	}
	if err := ValidateReleaseAgainstFiles(dst, manifest); err != nil {
		return DepotRelease{}, err
	}
	return manifest, nil
}

func sameInfoFiles(info []DepotInfoFile, files []DepotFile) bool {
	if len(info) != len(files) {
		return false
	}
	wants := make([]string, 0, len(info))
	gots := make([]string, 0, len(files))
	for _, file := range info {
		wants = append(wants, file.Path)
	}
	for _, file := range files {
		gots = append(gots, file.Path)
	}
	sortStrings(wants)
	sortStrings(gots)
	for i := range wants {
		if wants[i] != gots[i] {
			return false
		}
	}
	return true
}

func jsonManifest(release DepotRelease) ([]byte, error) {
	return json.Marshal(release)
}

func sortStrings(items []string) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j] < items[i] {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}
