package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisplayAssetsServeRootAndClientRoutes(t *testing.T) {
	dir := writeDisplayAssets(t)
	handler := NewServer(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		nil,
		nil,
		nil,
		"",
		nil,
		DisplayAssets{Directory: dir},
	)

	for _, target := range []string{"/", "/settings/voice"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d: %s", target, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "Jute display shell") {
			t.Fatalf("%s did not serve display index: %s", target, rec.Body.String())
		}
	}
}

func TestDisplayAssetsKeepAPIPrecedence(t *testing.T) {
	dir := writeDisplayAssets(t)
	handler := NewServer(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		nil,
		nil,
		nil,
		"",
		nil,
		DisplayAssets{Directory: dir},
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Jute display shell") {
		t.Fatalf("API route was handled by display fallback: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"eventStream"`) {
		t.Fatalf("API route did not return status JSON: %s", rec.Body.String())
	}
}

func TestDisplayAssetsReturnMissingAssetAsNotFound(t *testing.T) {
	dir := writeDisplayAssets(t)
	handler := NewServer(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		nil,
		nil,
		nil,
		"",
		nil,
		DisplayAssets{Directory: dir},
	)
	req := httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDisplayAssetsCanBeDisabledForHeadlessMode(t *testing.T) {
	dir := writeDisplayAssets(t)
	handler := NewServer(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		nil,
		nil,
		nil,
		"",
		nil,
		DisplayAssets{Headless: true, Directory: dir},
	)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
}

func writeDisplayAssets(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "index.html"),
		[]byte("<!doctype html><title>Jute display shell</title>"),
		0o600,
	); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o700); err != nil {
		t.Fatalf("make assets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log('jute')"), 0o600); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	return dir
}
