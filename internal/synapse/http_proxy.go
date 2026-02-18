package synapse

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// HTTPProxy provides SSRF-safe HTTP fetching for Wasm plugins.
// Each plugin has an explicit domain allowlist; everything else is denied.
type HTTPProxy struct {
	logger *slog.Logger
	client *http.Client

	mu          sync.RWMutex
	permissions map[string][]string // plugin → allowed hostnames
}

// NewHTTPProxy creates a new HTTP proxy with SSRF protections.
func NewHTTPProxy(logger *slog.Logger) *HTTPProxy {
	return &HTTPProxy{
		logger: logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		permissions: make(map[string][]string),
	}
}

// SetPluginPermissions sets the allowed hostnames for a plugin.
func (p *HTTPProxy) SetPluginPermissions(plugin string, hosts []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.permissions[plugin] = hosts
}

// Fetch performs an HTTP request on behalf of a plugin.
// Returns body bytes, status code, and error.
func (p *HTTPProxy) Fetch(ctx context.Context, plugin, method, rawURL, body string) ([]byte, int, error) {
	// 1. Parse the URL
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid URL: %w", err)
	}

	host := parsed.Hostname()

	// 2. SSRF deny list — always block internal addresses
	if isInternalHost(host) {
		return nil, 0, fmt.Errorf("host %q denied: internal address", host)
	}

	// 3. Check plugin permissions
	p.mu.RLock()
	allowed, hasPerms := p.permissions[plugin]
	p.mu.RUnlock()

	if !hasPerms {
		return nil, 0, fmt.Errorf("no HTTP permissions configured for plugin %q", plugin)
	}

	if !hostInAllowlist(host, allowed) {
		return nil, 0, fmt.Errorf("host %q not in plugin allowlist for %q", host, plugin)
	}

	// 4. Build and execute the request
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Limit to 1MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return data, resp.StatusCode, nil
}

// isInternalHost returns true for localhost, private IPs, metadata endpoints.
func isInternalHost(host string) bool {
	lower := strings.ToLower(host)

	// Explicit names
	if lower == "localhost" || lower == "metadata.google.internal" {
		return true
	}

	// Parse as IP
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

// hostInAllowlist checks whether the host is in the allowed list.
func hostInAllowlist(host string, allowed []string) bool {
	for _, h := range allowed {
		if strings.EqualFold(host, h) {
			return true
		}
	}
	return false
}

// ExtractHost extracts the hostname from a URL string.
// Returns empty string if the URL is invalid.
func ExtractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" {
		return ""
	}
	return u.Hostname()
}
