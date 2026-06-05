package dashboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BackgroundsRoutePrefix is the API base for local background image assets.
const BackgroundsRoutePrefix = "/api/v1/backgrounds"

// backgroundsFilesPrefix serves the raw image binaries.
const backgroundsFilesPrefix = BackgroundsRoutePrefix + "/files/"

// maxBackgroundUploadBytes caps a single uploaded background image.
const maxBackgroundUploadBytes = 20 << 20 // 20 MiB

func isAllowedBackgroundExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return true
	default:
		return false
	}
}

// BackgroundsController manages the local, hub-owned background image library.
// Images are stored as local media assets under a hub-managed directory and are
// referenced from display settings by file name only.
type BackgroundsController struct {
	dir string
}

func NewBackgroundsController(dir string) *BackgroundsController {
	return &BackgroundsController{dir: strings.TrimSpace(dir)}
}

func (c *BackgroundsController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(BackgroundsRoutePrefix, c.handleCollection)
	mux.Handle(backgroundsFilesPrefix, http.StripPrefix(backgroundsFilesPrefix, c.fileServer()))
}

type backgroundImage struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func (c *BackgroundsController) handleCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.handleList(w, r)
	case http.MethodPost:
		c.handleUpload(w, r)
	case http.MethodDelete:
		c.handleDelete(w, r)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		writeBackgroundError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (c *BackgroundsController) handleList(w http.ResponseWriter, _ *http.Request) {
	images, err := c.list()
	if err != nil {
		writeBackgroundError(w, http.StatusInternalServerError, "background library is unavailable")
		return
	}
	writeBackgroundJSON(w, http.StatusOK, map[string]any{"images": images})
}

func (c *BackgroundsController) handleUpload(w http.ResponseWriter, r *http.Request) {
	if c.dir == "" {
		writeBackgroundError(w, http.StatusServiceUnavailable, "background storage is not configured")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBackgroundUploadBytes+1024)

	if err := r.ParseMultipartForm(maxBackgroundUploadBytes + 1024); err != nil {
		writeBackgroundError(w, http.StatusRequestEntityTooLarge, "uploaded image is too large or malformed")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeBackgroundError(w, http.StatusBadRequest, "a 'file' image part is required")
		return
	}
	defer func() { _ = file.Close() }()

	name, err := safeBackgroundName(header.Filename)
	if err != nil {
		writeBackgroundError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := os.MkdirAll(c.dir, 0o750); err != nil {
		writeBackgroundError(w, http.StatusInternalServerError, "could not prepare background storage")
		return
	}
	name = c.uniqueName(name)
	//nolint:gosec // name is validated and flattened by safeBackgroundName.
	dest, err := os.Create(filepath.Join(c.dir, name))
	if err != nil {
		writeBackgroundError(w, http.StatusInternalServerError, "could not save background image")
		return
	}
	defer func() { _ = dest.Close() }()
	if _, err := io.Copy(dest, io.LimitReader(file, maxBackgroundUploadBytes)); err != nil {
		writeBackgroundError(w, http.StatusInternalServerError, "could not write background image")
		return
	}
	writeBackgroundJSON(w, http.StatusCreated, backgroundImage{Name: name, URL: backgroundsFilesPrefix + name})
}

func (c *BackgroundsController) handleDelete(w http.ResponseWriter, r *http.Request) {
	name, err := safeBackgroundName(r.URL.Query().Get("name"))
	if err != nil {
		writeBackgroundError(w, http.StatusBadRequest, err.Error())
		return
	}
	if c.dir == "" {
		writeBackgroundError(w, http.StatusServiceUnavailable, "background storage is not configured")
		return
	}
	//nolint:gosec // name is validated and flattened by safeBackgroundName.
	if err := os.Remove(filepath.Join(c.dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
		writeBackgroundError(w, http.StatusInternalServerError, "could not delete background image")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *BackgroundsController) list() ([]backgroundImage, error) {
	images := []backgroundImage{}
	if c.dir == "" {
		return images, nil
	}
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return images, nil
		}
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isAllowedBackgroundExt(name) {
			continue
		}
		images = append(images, backgroundImage{Name: name, URL: backgroundsFilesPrefix + name})
	}
	sort.Slice(images, func(i, j int) bool { return images[i].Name < images[j].Name })
	return images, nil
}

func (c *BackgroundsController) fileServer() http.Handler {
	dir := c.dir
	if dir == "" {
		dir = os.TempDir()
	}
	return http.FileServer(http.Dir(dir))
}

// uniqueName avoids clobbering an existing file by appending a numeric suffix.
func (c *BackgroundsController) uniqueName(name string) string {
	//nolint:gosec // name is validated and flattened by safeBackgroundName.
	if _, err := os.Stat(filepath.Join(c.dir, name)); errors.Is(err, os.ErrNotExist) {
		return name
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		//nolint:gosec // candidate is derived from a validated, flattened name.
		if _, err := os.Stat(filepath.Join(c.dir, candidate)); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return name
}

// safeBackgroundName validates and normalizes an uploaded file name to a safe,
// flat, allowed-extension reference (no paths, no traversal).
func safeBackgroundName(raw string) (string, error) {
	name := filepath.Base(strings.TrimSpace(raw))
	if name == "" || name == "." || name == ".." {
		return "", errors.New("invalid image file name")
	}
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return "", errors.New("invalid image file name")
	}
	if !isAllowedBackgroundExt(name) {
		return "", errors.New("unsupported image type; use jpg, png, webp, or gif")
	}
	return name, nil
}

func writeBackgroundJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeBackgroundError(w http.ResponseWriter, status int, message string) {
	writeBackgroundJSON(w, status, map[string]string{"error": message})
}
