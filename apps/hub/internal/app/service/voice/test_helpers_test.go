package voice

import (
	"encoding/json"
	"strings"
	"testing"
)

func assertJSONOmits(t *testing.T, value any, forbidden ...string) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal value: %v", err)
	}
	payload := string(data)
	for _, needle := range forbidden {
		if strings.Contains(strings.ToLower(payload), strings.ToLower(needle)) {
			t.Fatalf("projection leaked %q in JSON %s", needle, payload)
		}
	}
}
