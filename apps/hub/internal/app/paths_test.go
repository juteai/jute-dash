package app

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveDataDirUsesExplicitPath(t *testing.T) {
	explicit := filepath.Join(t.TempDir(), "custom-data")

	got, err := ResolveDataDir(explicit)
	if err != nil {
		t.Fatalf("ResolveDataDir() error = %v", err)
	}
	want, err := filepath.Abs(explicit)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	if got != want {
		t.Fatalf("ResolveDataDir() = %q, want %q", got, want)
	}
}

func TestResolveDataDirUsesJuteHome(t *testing.T) {
	juteHome := filepath.Join(t.TempDir(), "jute-home")
	t.Setenv("JUTE_HOME", juteHome)

	got, err := ResolveDataDir("")
	if err != nil {
		t.Fatalf("ResolveDataDir() error = %v", err)
	}
	want, err := filepath.Abs(juteHome)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	if got != want {
		t.Fatalf("ResolveDataDir() = %q, want %q", got, want)
	}
}

func TestResolveDataDirUsesPlatformDefaultWithoutJuteHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("JUTE_HOME", "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg-data"))

	got, err := ResolveDataDir("")
	if err != nil {
		t.Fatalf("ResolveDataDir() error = %v", err)
	}
	if got == "" {
		t.Fatal("ResolveDataDir() returned empty path")
	}

	switch runtime.GOOS {
	case "darwin":
		if !strings.HasSuffix(got, filepath.Join("Library", "Application Support", "Jute Dash")) {
			t.Fatalf("unexpected macOS default path: %q", got)
		}
	case "windows":
		if !strings.HasSuffix(got, "Jute Dash") {
			t.Fatalf("unexpected Windows default path: %q", got)
		}
	default:
		want := filepath.Join(home, "xdg-data", "jute-dash")
		if got != want {
			t.Fatalf("ResolveDataDir() = %q, want %q", got, want)
		}
	}
}

func TestDatabasePath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "jute")
	if got := DatabasePath(dir); got != filepath.Join(dir, "jute.db") {
		t.Fatalf("DatabasePath() = %q", got)
	}
}
