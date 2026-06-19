package dashboard

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func newBackgroundsServer(t *testing.T) *http.ServeMux {
	t.Helper()
	dir := t.TempDir()
	mux := http.NewServeMux()
	NewBackgroundsController(dir).RegisterRoutes(mux)
	return mux
}

func uploadBackground(t *testing.T, mux *http.ServeMux, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write part: %v", err)
	}
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, BackgroundsRoutePrefix, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestBackgroundUploadListDelete(t *testing.T) {
	mux := newBackgroundsServer(t)

	rec := uploadBackground(t, mux, "Kitchen Wall.png", []byte("fake-png-bytes"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var uploaded backgroundImage
	if err := json.Unmarshal(rec.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decode upload: %v", err)
	}
	if uploaded.Name == "" || uploaded.URL == "" {
		t.Fatalf("upload returned empty image: %+v", uploaded)
	}

	listReq := httptest.NewRequest(http.MethodGet, BackgroundsRoutePrefix, nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", listRec.Code)
	}
	var listed struct {
		Images []backgroundImage `json:"images"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed.Images) != 1 || listed.Images[0].Name != uploaded.Name {
		t.Fatalf("unexpected library contents: %+v", listed.Images)
	}

	delReq := httptest.NewRequest(
		http.MethodDelete,
		BackgroundsRoutePrefix+"?name="+url.QueryEscape(uploaded.Name),
		nil,
	)
	delRec := httptest.NewRecorder()
	mux.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", delRec.Code)
	}
}

func TestBackgroundUploadRejectsUnsupportedType(t *testing.T) {
	mux := newBackgroundsServer(t)
	rec := uploadBackground(t, mux, "evil.svg", []byte("<svg/>"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for unsupported type", rec.Code)
	}
}

func TestSafeBackgroundNameRejectsTraversal(t *testing.T) {
	// Genuinely invalid names are rejected.
	for _, name := range []string{"..", "", ".", "noext", "evil.svg"} {
		if _, err := safeBackgroundName(name); err == nil {
			t.Fatalf("expected error for unsafe name %q", name)
		}
	}
	// Path components are stripped to a safe flat basename.
	for _, tc := range []struct{ in, want string }{
		{"../escape.png", "escape.png"},
		{"a/b/c.png", "c.png"},
	} {
		got, err := safeBackgroundName(tc.in)
		if err != nil {
			t.Fatalf("safeBackgroundName(%q) error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("safeBackgroundName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeLayoutDefaultsModeAndEnforces12Col(t *testing.T) {
	catalog := widgetCatalogByKind()
	layout := WidgetLayout{
		ProfileID: DefaultLayoutProfileID,
		Widgets: []WidgetInstance{
			{ID: "w1", Kind: "date-time", Title: "Clock", X: 0, Y: 0, W: 6, H: 1, MinW: 3, MinH: 1, Size: "wide"},
		},
	}
	normalized, err := NormalizeWidgetLayout(layout, catalog)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized.Widgets[0].Mode != WidgetModeUI {
		t.Fatalf("expected default mode ui, got %q", normalized.Widgets[0].Mode)
	}

	headless := WidgetLayout{
		ProfileID: DefaultLayoutProfileID,
		Widgets: []WidgetInstance{
			{
				ID:    "w1",
				Kind:  "weather",
				Title: "W",
				X:     0,
				Y:     0,
				W:     6,
				H:     1,
				MinW:  3,
				MinH:  1,
				Size:  "wide",
				Mode:  WidgetModeHeadless,
			},
		},
	}
	if _, err := NormalizeWidgetLayout(headless, catalog); err != nil {
		t.Fatalf("headless mode should be valid: %v", err)
	}

	tooWide := WidgetLayout{
		ProfileID: DefaultLayoutProfileID,
		Widgets: []WidgetInstance{
			{ID: "w1", Kind: "date-time", Title: "Clock", X: 8, Y: 0, W: 6, H: 1, MinW: 3, MinH: 1, Size: "wide"},
		},
	}
	if _, err := NormalizeWidgetLayout(tooWide, catalog); err == nil {
		t.Fatalf("expected column-bounds error for x+w > %d", BaseColumns)
	}
}
