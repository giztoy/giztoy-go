package firmware

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ParseInfo(data []byte) (DepotInfo, error) {
	var info DepotInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return DepotInfo{}, err
	}
	if err := ValidateInfo(info); err != nil {
		return DepotInfo{}, err
	}
	return info, nil
}

func ValidateInfo(info DepotInfo) error {
	seen := map[string]struct{}{}
	for _, file := range info.Files {
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

func WriteInfo(path string, info DepotInfo) error {
	if err := ValidateInfo(info); err != nil {
		return err
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func normalizeInfoPaths(files []DepotInfoFile) []DepotInfoFile {
	out := make([]DepotInfoFile, len(files))
	copy(out, files)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func infoPath(root, depot string) string {
	return filepath.Join(root, depot, "info.json")
}

func validateRelativePath(path string) error {
	if path == "" {
		return ErrInvalidPath
	}
	if strings.HasPrefix(path, "/") {
		return ErrInvalidPath
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, `\`) {
		return ErrInvalidPath
	}
	if strings.Contains(clean, "/../") {
		return ErrInvalidPath
	}
	return nil
}
