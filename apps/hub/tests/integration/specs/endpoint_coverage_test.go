package specs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"jute-dash/apps/hub/tests/integration/bootstrap"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Hub endpoint coverage", Label("SMOKE"), func() {
	var client *http.Client

	BeforeEach(func() {
		client = bootstrap.Client()
	})

	It("covers setup, home, and settings endpoints", func(ctx SpecContext) {
		expectJSON(ctx, client, http.MethodGet, "/api/v1/setup/status", nil, http.StatusOK)
		household := expectJSON(ctx, client, http.MethodGet, "/api/v1/settings/household", nil, http.StatusOK)
		expectJSON(ctx, client, http.MethodPatch, "/api/v1/settings/household", household, http.StatusOK)

		rooms := expectJSON(ctx, client, http.MethodGet, "/api/v1/settings/rooms", nil, http.StatusOK)
		tiles := expectJSON(ctx, client, http.MethodGet, "/api/v1/settings/tiles", nil, http.StatusOK)
		defer expectJSON(ctx, client, http.MethodPut, "/api/v1/settings/rooms", rooms, http.StatusOK)
		defer expectJSON(ctx, client, http.MethodPut, "/api/v1/settings/tiles", tiles, http.StatusOK)

		expectJSON(ctx, client, http.MethodPut, "/api/v1/settings/rooms", map[string]any{
			"rooms": []map[string]any{{
				"id": "integration-room", "name": "Integration Room",
			}},
		}, http.StatusOK)
		expectJSON(ctx, client, http.MethodPut, "/api/v1/settings/tiles", map[string]any{
			"tiles": []map[string]any{{
				"id": "integration-tile", "kind": "status", "label": "Integration", "value": "OK",
			}},
		}, http.StatusOK)
		expectJSON(ctx, client, http.MethodGet, "/api/v1/settings/connection-kinds", nil, http.StatusOK)
		expectJSON(ctx, client, http.MethodGet, "/api/v1/settings/connections", nil, http.StatusOK)
		expectJSON(ctx, client, http.MethodPost, "/api/v1/settings/connections", "{", http.StatusBadRequest)
		expectJSON(ctx, client, http.MethodGet, "/api/v1/home", nil, http.StatusOK)
	})

	It("covers widget, background, and agent endpoints", func(ctx SpecContext) {
		expectJSON(ctx, client, http.MethodGet, "/api/v1/widgets/catalog", nil, http.StatusOK)
		layout := expectJSON(ctx, client, http.MethodGet, "/api/v1/widgets/layout", nil, http.StatusOK)
		expectJSON(ctx, client, http.MethodPut, "/api/v1/widgets/layout", layout, http.StatusOK)
		expectJSON(ctx, client, http.MethodPatch, "/api/v1/widgets/layout/active-screen", map[string]any{
			"screenId": stringField(layout, "activeScreenId", "home"),
		}, http.StatusOK)
		expectJSON(
			ctx,
			client,
			http.MethodPost,
			"/api/v1/widgets/layout/reset?profileId=missing-integration-profile",
			nil,
			http.StatusBadRequest,
		)

		expectJSON(ctx, client, http.MethodGet, "/api/v1/backgrounds", nil, http.StatusOK)
		uploaded := expectMultipartBackground(ctx, client)
		if uploaded != "" {
			expectNoBody(ctx, client, http.MethodDelete, "/api/v1/backgrounds?name="+uploaded, http.StatusNoContent)
		}
		expectJSON(ctx, client, http.MethodDelete, "/api/v1/backgrounds", nil, http.StatusBadRequest)
		expectNoBody(ctx, client, http.MethodGet, "/api/v1/backgrounds/files/missing.png", http.StatusNotFound)

		expectJSON(ctx, client, http.MethodGet, "/api/v1/agents", nil, http.StatusOK)
		expectJSON(ctx, client, http.MethodPost, "/api/v1/agents", "{", http.StatusBadRequest)
		expectJSON(ctx, client, http.MethodPatch, "/api/v1/agents/missing", map[string]any{
			"enabled": false,
		}, http.StatusNotFound)
		expectJSON(ctx, client, http.MethodDelete, "/api/v1/agents/missing", nil, http.StatusNotFound)
		expectJSON(
			ctx,
			client,
			http.MethodPost,
			"/api/v1/agents/missing/refresh-card",
			nil,
			http.StatusNotFound,
		)
		expectJSON(ctx, client, http.MethodGet, "/api/v1/proxy/agents/missing", nil, http.StatusNotFound)
	})

	It("covers voice and TTS endpoints", func(ctx SpecContext) {
		streamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, bootstrap.BaseURL()+"/api/v1/events", nil)
		Expect(err).NotTo(HaveOccurred())
		streamResp, err := client.Do(streamReq)
		Expect(err).NotTo(HaveOccurred())
		defer streamResp.Body.Close()
		stream := bufio.NewReader(streamResp.Body)

		status := expectJSON(ctx, client, http.MethodGet, "/api/v1/voice/status", nil, http.StatusOK)
		expectJSON(ctx, client, http.MethodPatch, "/api/v1/voice/settings", map[string]any{
			"enabled":               boolField(status, "enabled"),
			"ttsEnabled":            boolField(status, "ttsEnabled"),
			"wakeSensitivity":       numberField(status, "wakeSensitivity", 0.5),
			"followupWindowSeconds": numberField(status, "followupWindowSeconds", 8),
		}, http.StatusOK)
		expectJSON(ctx, client, http.MethodPost, "/api/v1/voice/mute", nil, http.StatusOK)
		expectSSEPayload(ctx, stream, "voice.state_changed", true)
		expectJSON(ctx, client, http.MethodPost, "/api/v1/voice/unmute", nil, http.StatusOK)
		expectSSEPayload(ctx, stream, "voice.state_changed", false)
		expectJSON(ctx, client, http.MethodPost, "/api/v1/voice/cancel", nil, http.StatusOK)
		expectJSON(ctx, client, http.MethodGet, "/api/v1/voice/providers", nil, http.StatusOK)
		expectJSON(ctx, client, http.MethodPost, "/api/v1/voice/transcripts/final", map[string]any{
			"text": "",
		}, http.StatusBadRequest)
		expectJSON(ctx, client, http.MethodGet, "/api/v1/tts/voices", nil, http.StatusOK)
		expectJSON(ctx, client, http.MethodPost, "/api/v1/tts/speak", map[string]any{
			"text": "integration test",
		}, http.StatusOK)
		expectJSON(ctx, client, http.MethodPost, "/api/v1/tts/stop", map[string]any{}, http.StatusOK)
		if boolField(status, "muted") {
			expectJSON(ctx, client, http.MethodPost, "/api/v1/voice/mute", nil, http.StatusOK)
		} else {
			expectJSON(ctx, client, http.MethodPost, "/api/v1/voice/unmute", nil, http.StatusOK)
		}
	})
})

func expectMultipartBackground(ctx SpecContext, client *http.Client) string {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "integration-background.png")
	Expect(err).NotTo(HaveOccurred())
	_, err = part.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	Expect(err).NotTo(HaveOccurred())
	Expect(writer.Close()).To(Succeed())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, bootstrap.BaseURL()+"/api/v1/backgrounds", &body)
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := client.Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	Expect([]int{http.StatusCreated, http.StatusServiceUnavailable}).To(ContainElement(resp.StatusCode))
	raw, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())
	if resp.StatusCode != http.StatusCreated {
		return ""
	}
	var decoded map[string]any
	Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
	return stringField(decoded, "name", "")
}

func expectNoBody(ctx SpecContext, client *http.Client, method, path string, status int) {
	req, err := http.NewRequestWithContext(ctx, method, bootstrap.BaseURL()+path, nil)
	Expect(err).NotTo(HaveOccurred())
	resp, err := client.Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(status))
}

func expectSSEPayload(ctx SpecContext, reader *bufio.Reader, eventName string, muted bool) {
	var sawEvent bool
	for {
		select {
		case <-ctx.Done():
			Fail("timed out waiting for " + eventName)
		default:
		}
		line, err := reader.ReadString('\n')
		Expect(err).NotTo(HaveOccurred())
		line = strings.TrimSpace(line)
		if line == "event: "+eventName {
			sawEvent = true
			continue
		}
		if !sawEvent || !strings.HasPrefix(line, "data: ") {
			continue
		}
		var envelope struct {
			Payload struct {
				Muted bool `json:"muted"`
			} `json:"payload"`
		}
		Expect(json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &envelope)).To(Succeed())
		if envelope.Payload.Muted == muted {
			return
		}
		sawEvent = false
	}
}

func stringField(values map[string]any, key, fallback string) string {
	if value, ok := values[key].(string); ok && value != "" {
		return value
	}
	return fallback
}

func boolField(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}

func numberField(values map[string]any, key string, fallback float64) float64 {
	if value, ok := values[key].(float64); ok {
		return value
	}
	return fallback
}
