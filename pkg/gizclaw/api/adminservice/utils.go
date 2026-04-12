package adminservice

import (
	"encoding/json"
	"github.com/giztoy/giztoy-go/pkg/firmware"
	"time"
)

func reencode[T any](v any) (T, error) {
	var out T
	data, err := json.Marshal(v)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	out := v
	return &out
}

func boolPtr(v bool) *bool {
	if !v {
		return nil
	}
	out := v
	return &out
}

func millisPtr(ms int64) *time.Time {
	if ms == 0 {
		return nil
	}
	t := time.UnixMilli(ms).UTC()
	return &t
}

func millisTime(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

func toAdminDepotFile(in firmware.DepotFile) DepotFile {
	return DepotFile{
		Md5:    in.MD5,
		Path:   in.Path,
		Sha256: in.SHA256,
	}
}

func toAdminDepotInfo(in firmware.DepotInfo) DepotInfo {
	files := make([]DepotInfoFile, 0, len(in.Files))
	for _, file := range in.Files {
		files = append(files, DepotInfoFile{Path: file.Path})
	}
	if len(files) == 0 {
		return DepotInfo{}
	}
	return DepotInfo{Files: &files}
}

func toAdminDepotRelease(in firmware.DepotRelease) DepotRelease {
	out := DepotRelease{
		Channel:        stringPtr(in.Channel),
		FirmwareSemver: in.FirmwareSemVer,
	}
	if len(in.Files) > 0 {
		files := make([]DepotFile, 0, len(in.Files))
		for _, file := range in.Files {
			files = append(files, toAdminDepotFile(file))
		}
		out.Files = &files
	}
	return out
}

func toAdminDepot(in firmware.Depot) Depot {
	return Depot{
		Beta:     toAdminDepotRelease(in.Beta),
		Info:     toAdminDepotInfo(in.Info),
		Name:     in.Name,
		Rollback: toAdminDepotRelease(in.Rollback),
		Stable:   toAdminDepotRelease(in.Stable),
		Testing:  toAdminDepotRelease(in.Testing),
	}
}
