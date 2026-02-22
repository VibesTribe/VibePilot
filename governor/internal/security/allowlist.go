package security

import (
	"fmt"
	"net/url"
)

type HTTPAllowlist struct {
	allowedHosts map[string]bool
}

func NewHTTPAllowlist(hosts []string) *HTTPAllowlist {
	a := &HTTPAllowlist{
		allowedHosts: make(map[string]bool),
	}

	defaultHosts := []string{
		"api.supabase.co",
		"api.github.com",
		"github.com",
		"api.anthropic.com",
		"api.openai.com",
		"generativelanguage.googleapis.com",
		"api.deepseek.com",
	}

	for _, h := range defaultHosts {
		a.allowedHosts[h] = true
	}

	for _, h := range hosts {
		a.allowedHosts[h] = true
	}

	return a
}

func (a *HTTPAllowlist) ValidateURL(urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if !a.allowedHosts[u.Host] {
		return fmt.Errorf("host %s not in allowlist", u.Host)
	}

	return nil
}

func (a *HTTPAllowlist) IsAllowed(host string) bool {
	return a.allowedHosts[host]
}

func (a *HTTPAllowlist) AddHost(host string) {
	a.allowedHosts[host] = true
}

func (a *HTTPAllowlist) RemoveHost(host string) {
	delete(a.allowedHosts, host)
}
