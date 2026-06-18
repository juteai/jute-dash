package app

import (
	"bytes"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type DisplayAssets struct {
	Headless  bool
	Directory string
	FS        fs.FS
}

func selectedDisplayAssets(options []DisplayAssets) DisplayAssets {
	if len(options) == 0 {
		return DisplayAssets{}
	}
	return options[0]
}

func displayAssetHandler(assets DisplayAssets) http.Handler {
	if assets.Headless {
		return nil
	}

	var displayFS fs.FS
	if dir := strings.TrimSpace(assets.Directory); dir != "" {
		displayFS = os.DirFS(dir)
	} else {
		displayFS = assets.FS
	}
	if displayFS == nil || !displayAssetExists(displayFS, "index.html") {
		return nil
	}

	fileServer := http.FileServer(http.FS(displayFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := cleanDisplayAssetPath(r.URL.Path)
		if name == "" || displayAssetExists(displayFS, name) {
			fileServer.ServeHTTP(w, r)
			return
		}
		if filepath.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}

		serveDisplayIndex(w, r, displayFS)
	})
}

func serveDisplayIndex(w http.ResponseWriter, r *http.Request, displayFS fs.FS) {
	content, err := fs.ReadFile(displayFS, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	info, err := fs.Stat(displayFS, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, "index.html", info.ModTime(), bytes.NewReader(content))
}

func cleanDisplayAssetPath(requestPath string) string {
	cleaned := path.Clean("/" + requestPath)
	if cleaned == "/" {
		return ""
	}
	return strings.TrimPrefix(cleaned, "/")
}

func displayAssetExists(displayFS fs.FS, name string) bool {
	info, err := fs.Stat(displayFS, name)
	return err == nil && !info.IsDir()
}
