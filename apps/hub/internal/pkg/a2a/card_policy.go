package a2a

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var ErrAgentCardURLNotAllowed = errors.New("agent card url is not allowed")

type AgentCardURLPolicy struct {
	URLs     []string `json:"allowedAgentCardURLs" yaml:"allowed-agent-card-urls"`
	Loopback *bool    `json:"allowLoopback" yaml:"allow-loopback"` //nolint:golines // Config keys stay explicit for YAML/JSON users.
}

func DefaultAgentCardURLPolicy() AgentCardURLPolicy {
	allowLoopback := true
	return AgentCardURLPolicy{Loopback: &allowLoopback}
}

type AuthorizedAgentCardURL struct {
	raw string
}

func (u AuthorizedAgentCardURL) String() string {
	return u.raw
}

func (p AgentCardURLPolicy) Authorize(raw string) (AuthorizedAgentCardURL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return AuthorizedAgentCardURL{}, errors.New("agent card url is required")
	}
	parsed, err := parseAgentCardURL(trimmed)
	if err != nil {
		return AuthorizedAgentCardURL{}, err
	}
	if p.loopbackAllowed() && isLoopbackHost(parsed.Hostname()) {
		return AuthorizedAgentCardURL{raw: parsed.String()}, nil
	}
	for _, allowed := range p.URLs {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if matchesAllowedAgentCardURL(parsed, allowed) {
			return AuthorizedAgentCardURL{raw: parsed.String()}, nil
		}
	}
	return AuthorizedAgentCardURL{}, ErrAgentCardURLNotAllowed
}

func (p AgentCardURLPolicy) loopbackAllowed() bool {
	return p.Loopback == nil || *p.Loopback
}

func ValidateAgentCardURLPolicy(p AgentCardURLPolicy) []string {
	var problems []string
	for i, allowed := range p.URLs {
		location := fmt.Sprintf("a2a.allowedAgentCardURLs[%d]", i)
		if strings.TrimSpace(allowed) == "" {
			problems = append(problems, location+" is required")
			continue
		}
		if _, err := parseAllowedAgentCardURL(allowed); err != nil {
			problems = append(problems, location+" "+err.Error())
		}
	}
	return problems
}

func parseAgentCardURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("agent card url is invalid: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("agent card url must use http or https")
	}
	if parsed.User != nil {
		return nil, errors.New("agent card url must not include user info")
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return nil, errors.New("agent card url must include a host")
	}
	if strings.TrimSpace(parsed.RawQuery) != "" || strings.TrimSpace(parsed.Fragment) != "" {
		return nil, errors.New("agent card url must not include a query or fragment")
	}
	return parsed, nil
}

func parseAllowedAgentCardURL(raw string) (*url.URL, error) {
	parsed, err := parseAgentCardURL(raw)
	if err != nil {
		return nil, errors.New(strings.NewReplacer("agent card url ", "").Replace(err.Error()))
	}
	host := parsed.Hostname()
	if strings.Contains(host, "*") && !strings.HasPrefix(host, "*.") {
		return nil, errors.New("wildcards must be a leading host label such as *.example.com")
	}
	return parsed, nil
}

func matchesAllowedAgentCardURL(candidate *url.URL, allowedRaw string) bool {
	allowed, err := parseAllowedAgentCardURL(allowedRaw)
	if err != nil {
		return false
	}
	if candidate.Scheme != allowed.Scheme || candidate.Port() != allowed.Port() {
		return false
	}
	if !matchesAllowedHost(candidate.Hostname(), allowed.Hostname()) {
		return false
	}
	return cleanURLPath(candidate.EscapedPath()) == cleanURLPath(allowed.EscapedPath())
}

func matchesAllowedHost(candidate, allowed string) bool {
	candidate = strings.ToLower(strings.TrimSuffix(candidate, "."))
	allowed = strings.ToLower(strings.TrimSuffix(allowed, "."))
	if !strings.HasPrefix(allowed, "*.") {
		return candidate == allowed
	}
	suffix := strings.TrimPrefix(allowed, "*")
	return strings.HasSuffix(candidate, suffix) && candidate != strings.TrimPrefix(suffix, ".")
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func cleanURLPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}
