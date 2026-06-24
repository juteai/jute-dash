package paths

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	envJuteHome = "JUTE_HOME"
	dbFileName  = "jute.db"
)

func ResolveDataDir(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return absoluteClean(explicit)
	}
	if home := strings.TrimSpace(os.Getenv(envJuteHome)); home != "" {
		return absoluteClean(home)
	}
	return defaultDataDir()
}

func DatabasePath(dataDir string) string {
	return filepath.Join(dataDir, dbFileName)
}

// BackgroundsDir returns the hub-managed directory for local background images.
func BackgroundsDir(dataDir string) string {
	return filepath.Join(dataDir, "backgrounds")
}

// backgroundsDir is the resolved local background image directory, set once at
// startup from the resolved data directory. Empty in tests / library use.
//
//nolint:gochecknoglobals // Startup-set runtime path; mirrors data-dir resolution.
var backgroundsDir string

// SetBackgroundsDir records the hub-managed background image directory. It is
// called once during startup before the HTTP handler is constructed.
func SetBackgroundsDir(dir string) {
	backgroundsDir = strings.TrimSpace(dir)
}

func BackgroundsDirPath() string {
	return backgroundsDir
}

func defaultDataDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(home) == "" {
			return "", errors.New("home directory is empty")
		}
		return filepath.Join(home, "Library", "Application Support", "Jute Dash"), nil
	case "windows":
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(configDir) == "" {
			return "", errors.New("user config directory is empty")
		}
		return filepath.Join(configDir, "Jute Dash"), nil
	default:
		if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
			return absoluteClean(filepath.Join(xdg, "jute-dash"))
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(home) == "" {
			return "", errors.New("home directory is empty")
		}
		return filepath.Join(home, ".local", "share", "jute-dash"), nil
	}
}

func absoluteClean(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}
