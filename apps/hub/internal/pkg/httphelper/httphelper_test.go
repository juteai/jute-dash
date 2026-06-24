package httphelper

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireMethodWritesMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()

	if RequireMethod(rec, req, http.MethodGet) {
		t.Fatal("RequireMethod() = true, want false")
	}
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("unexpected response: status=%d allow=%q", rec.Code, rec.Header().Get("Allow"))
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "method not allowed" {
		t.Fatalf("error = %q", body["error"])
	}
}
