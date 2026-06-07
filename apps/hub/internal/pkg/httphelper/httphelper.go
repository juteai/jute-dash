package httphelper

import (
	"encoding/json"
	"net/http"
)

// WriteJSON writes a JSON payload with a given HTTP status code.
func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// WriteError writes an error message as JSON.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}

// RequireMethod checks if the request method matches the expected method, writing Method Not Allowed if not.
func RequireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	return true
}

// WriteMethodNotAllowed writes an Allow header and a Method Not Allowed error.
func WriteMethodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
}
