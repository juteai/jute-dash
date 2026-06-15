package app

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

var errDisplayDisabled = errors.New("display serving is disabled")

type DisplayOptions struct {
	Headless bool
	Dir      string
	FS       fs.FS
}

func (o DisplayOptions) handler() (http.Handler, error) {
	if o.Headless {
		return nil, errDisplayDisabled
	}
	if strings.TrimSpace(o.Dir) != "" {
		return newDisplayHandler(os.DirFS(o.Dir)), nil
	}
	if o.FS == nil {
		return nil, errors.New("display assets are not configured")
	}
	sub, err := fs.Sub(o.FS, "dist")
	if err != nil {
		return nil, err
	}
	return newDisplayHandler(sub), nil
}

func newDisplayHandler(assets fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
			http.NotFound(w, r)
			return
		}

		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if cleanPath == "." || cleanPath == "" {
			cleanPath = "index.html"
		}
		if info, err := fs.Stat(assets, cleanPath); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		if err := serveDisplayFile(w, r, assets, "index.html"); err != nil {
			http.NotFound(w, r)
			return
		}
	})
}

func serveDisplayFile(w http.ResponseWriter, r *http.Request, assets fs.FS, name string) error {
	file, err := assets.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", name)
	}

	content, ok := file.(io.ReadSeeker)
	if !ok {
		return fmt.Errorf("%s is not seekable", name)
	}
	http.ServeContent(w, r, name, info.ModTime(), content)
	return nil
}
