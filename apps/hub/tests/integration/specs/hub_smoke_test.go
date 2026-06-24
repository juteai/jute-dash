package specs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"jute-dash/apps/hub/tests/integration/bootstrap"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Hub API", Label("SMOKE"), func() {
	var client *http.Client

	BeforeEach(func() {
		client = bootstrap.Client()
	})

	It("serves health, status, and config", func(ctx SpecContext) {
		expectJSON(ctx, client, http.MethodGet, "/healthz", nil, http.StatusOK)
		expectJSON(ctx, client, http.MethodGet, "/api/v1/status", nil, http.StatusOK)
		expectJSON(ctx, client, http.MethodGet, "/api/v1/config", nil, http.StatusOK)
	})

	It("serves settings, layout, and voice provider APIs", func(ctx SpecContext) {
		expectJSON(ctx, client, http.MethodGet, "/api/v1/settings/household", nil, http.StatusOK)
		expectJSON(ctx, client, http.MethodGet, "/api/v1/widgets/layout", nil, http.StatusOK)
		expectJSON(ctx, client, http.MethodGet, "/api/v1/voice/providers", nil, http.StatusOK)
	})

	It("exposes an SSE stream", func(ctx SpecContext) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, bootstrap.BaseURL()+"/api/v1/events", nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err := client.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(resp.Header.Get("Content-Type")).To(ContainSubstring("text/event-stream"))

		reader := bufio.NewReader(resp.Body)
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(strings.TrimSpace(line)).NotTo(BeEmpty())
	})

	It("does not serve an embedded display app", func(ctx SpecContext) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, bootstrap.BaseURL()+"/", nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err := client.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		Expect(string(body)).NotTo(ContainSubstring("Jute Dash"))
	})

	It("accepts final voice transcript requests as text when voice is enabled", func(ctx SpecContext) {
		enableVoice(ctx, client)
		body := expectJSON(
			ctx,
			client,
			http.MethodPost,
			"/api/v1/voice/transcripts/final",
			map[string]any{"text": "hello", "agentId": "mock-agent"},
			http.StatusOK,
			http.StatusBadGateway,
		)
		Expect(body).NotTo(BeNil())
	})
})

func enableVoice(ctx SpecContext, client *http.Client) {
	expectJSON(ctx, client, http.MethodPatch, "/api/v1/voice/settings", map[string]any{
		"enabled":    true,
		"ttsEnabled": true,
	}, http.StatusOK)
	expectJSON(ctx, client, http.MethodPost, "/api/v1/voice/unmute", nil, http.StatusOK)
}

func expectJSON(
	ctx SpecContext,
	client *http.Client,
	method string,
	path string,
	body any,
	statuses ...int,
) map[string]any {
	var reader io.Reader
	if body != nil {
		if raw, ok := body.(string); ok {
			reader = strings.NewReader(raw)
		} else {
			raw, err := json.Marshal(body)
			Expect(err).NotTo(HaveOccurred())
			reader = bytes.NewReader(raw)
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, bootstrap.BaseURL()+path, reader)
	Expect(err).NotTo(HaveOccurred())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	Expect(statuses).To(ContainElement(resp.StatusCode))
	if resp.Body == nil {
		return nil
	}
	raw, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())
	if len(raw) == 0 {
		return nil
	}
	var decoded map[string]any
	Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
	return decoded
}

var _ = BeforeSuite(func() {
	SetDefaultEventuallyTimeout(5 * time.Second)
})
