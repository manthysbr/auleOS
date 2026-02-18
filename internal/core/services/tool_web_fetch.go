package services

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// isSSRFTarget checks if a URL targets internal/metadata endpoints.
func isSSRFTarget(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return true // block unparseable URLs
	}

	host := parsed.Hostname()

	// Block common SSRF targets
	blocked := []string{
		"localhost",
		"127.0.0.1",
		"0.0.0.0",
		"[::1]",
		"::1",
		"169.254.169.254", // AWS metadata
		"metadata.google.internal",
		"metadata.google",
	}
	for _, b := range blocked {
		if strings.EqualFold(host, b) {
			return true
		}
	}

	// Block private IP ranges
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return true
		}
	}

	// Block non-HTTP schemes
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return true
	}

	return false
}

// NewWebFetchTool creates a tool that fetches content from a URL.
func NewWebFetchTool() *domain.Tool {
	return &domain.Tool{
		Name:        "web_fetch",
		Description: "Fetches the content of a web page URL. Returns the text content (HTML stripped of scripts/styles). Use after web_search to read a page's full content. Max 1MB response.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "The URL to fetch (e.g., 'https://example.com/article').",
				},
			},
			Required: []string{"url"},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			rawURL, ok := params["url"].(string)
			if !ok || rawURL == "" {
				return nil, fmt.Errorf("url is required")
			}

			// Ensure scheme
			if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
				rawURL = "https://" + rawURL
			}

			// SSRF protection
			if isSSRFTarget(rawURL) {
				return nil, fmt.Errorf("URL denied: cannot fetch internal/private addresses")
			}

			// Create request with timeout
			fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(fetchCtx, "GET", rawURL, nil)
			if err != nil {
				return nil, fmt.Errorf("invalid URL: %w", err)
			}
			req.Header.Set("User-Agent", "auleOS/1.0 (Agent Web Fetch)")
			req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain,*/*")

			client := &http.Client{
				Timeout: 30 * time.Second,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					// Check each redirect target for SSRF
					if isSSRFTarget(req.URL.String()) {
						return fmt.Errorf("redirect to internal address denied")
					}
					if len(via) >= 5 {
						return fmt.Errorf("too many redirects")
					}
					return nil
				},
			}

			resp, err := client.Do(req)
			if err != nil {
				return nil, fmt.Errorf("fetch failed: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 {
				return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			}

			// Read up to 1MB
			const maxSize = 1024 * 1024
			limited := io.LimitReader(resp.Body, maxSize+1)
			body, err := io.ReadAll(limited)
			if err != nil {
				return nil, fmt.Errorf("failed to read response: %w", err)
			}
			if len(body) > maxSize {
				body = body[:maxSize]
			}

			content := string(body)

			// Basic HTML to text extraction (strip script/style tags and HTML tags)
			contentType := resp.Header.Get("Content-Type")
			if strings.Contains(contentType, "text/html") || strings.Contains(content, "<html") {
				content = extractTextFromHTML(content)
			}

			// Truncate for token budget
			if len(content) > 32000 {
				content = content[:32000] + "\n\n... (content truncated at 32KB)"
			}

			if strings.TrimSpace(content) == "" {
				return "(page returned empty content)", nil
			}

			return fmt.Sprintf("URL: %s\nStatus: %d\n\n%s", rawURL, resp.StatusCode, content), nil
		},
	}
}

// extractTextFromHTML does a basic HTML-to-text conversion.
// Strips script, style, and HTML tags. Not a full parser, but sufficient for agent consumption.
func extractTextFromHTML(html string) string {
	// Remove script and style blocks
	result := html
	for _, tag := range []string{"script", "style", "noscript", "nav", "footer", "header"} {
		for {
			openTag := strings.Index(strings.ToLower(result), "<"+tag)
			if openTag == -1 {
				break
			}
			closeTag := strings.Index(strings.ToLower(result[openTag:]), "</"+tag+">")
			if closeTag == -1 {
				// Remove from open tag to end
				result = result[:openTag]
				break
			}
			endIdx := openTag + closeTag + len("</"+tag+">")
			result = result[:openTag] + result[endIdx:]
		}
	}

	// Strip all remaining HTML tags
	var text strings.Builder
	inTag := false
	for _, ch := range result {
		if ch == '<' {
			inTag = true
			continue
		}
		if ch == '>' {
			inTag = false
			text.WriteRune(' ')
			continue
		}
		if !inTag {
			text.WriteRune(ch)
		}
	}

	// Collapse whitespace
	lines := strings.Split(text.String(), "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.Join(strings.Fields(line), " ")
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}

	return strings.Join(cleaned, "\n")
}
