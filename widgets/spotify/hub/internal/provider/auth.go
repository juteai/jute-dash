package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (c Client) refreshToken(ctx context.Context) (string, error) {
	if c.settings.RefreshToken == "" || c.settings.ClientID == "" {
		return "", errors.New("missing credentials for refresh")
	}

	tokenURL := "https://accounts.spotify.com/api/token" //nolint:gosec // URL is not a secret credential
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", c.settings.RefreshToken)
	data.Set("client_id", c.settings.ClientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	if c.settings.ClientSecret != "" {
		authHeader := base64.StdEncoding.EncodeToString(
			[]byte(fmt.Sprintf("%s:%s", c.settings.ClientID, c.settings.ClientSecret)),
		)
		req.Header.Set("Authorization", fmt.Sprintf("Basic %s", authHeader))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("refresh failed with status %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	return tokenResp.AccessToken, nil
}

func (c Client) doRequest(
	ctx context.Context,
	method, urlStr string,
	body []byte,
) (*http.Response, error) {
	token := c.settings.AccessToken

	if token == "" || (c.settings.ExpiresAt > 0 && time.Now().Unix() >= c.settings.ExpiresAt-60) {
		newToken, err := c.refreshToken(ctx)
		if err == nil {
			token = newToken
		}
	}

	makeReq := func(tok string) (*http.Request, error) {
		var rBody io.Reader
		if body != nil {
			rBody = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, urlStr, rBody)
		if err != nil {
			return nil, err
		}
		if tok != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tok))
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return req, nil
	}

	req, err := makeReq(token)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		newToken, err := c.refreshToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("unauthorized and refresh failed: %w", err)
		}
		req2, err := makeReq(newToken)
		if err != nil {
			return nil, err
		}
		return http.DefaultClient.Do(req2)
	}

	return resp, nil
}
