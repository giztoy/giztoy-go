package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

// ConfigDir returns the giztoy configuration root directory.
//
// On Unix-like systems (Linux, macOS), it follows the XDG convention:
//
//	$XDG_CONFIG_HOME/giztoy  (if set)
//	~/.config/giztoy          (fallback)
//
// On Windows, it uses the standard app-data location:
//
//	%AppData%\giztoy          (via os.UserConfigDir)
func ConfigDir() (string, error) {
	if runtime.GOOS != "windows" {
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "giztoy"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "giztoy"), nil
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "giztoy"), nil
}
