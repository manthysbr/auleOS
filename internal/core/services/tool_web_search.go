package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

type WebSearchTool struct{}

func NewWebSearchTool() *domain.Tool {
	return &domain.Tool{
		Name:        "web_search",
		Description: "Searches the web for information using Brave Search (if configured) or DuckDuckGo. Returns top results with titles, snippets, and URLs.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query (e.g., 'latest golang release notes').",
				},
			},
			Required: []string{"query"},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			query, ok := params["query"].(string)
			if !ok || query == "" {
				return nil, fmt.Errorf("query is required")
			}

			// 1. Try Brave Search (API Key required)
			if apiKey := os.Getenv("BRAVE_SEARCH_API_KEY"); apiKey != "" {
				results, err := searchBrave(ctx, query, apiKey)
				if err == nil {
					return results, nil
				}
				// Log error and fall back?
				// For now, just fall back silently or maybe return the error?
				// Let's fall back to DDG if Brave fails.
			}

			// 2. Fallback to DuckDuckGo (HTML Scraping)
			return searchDuckDuckGo(ctx, query)
		},
	}
}

type SearchResult struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
}

func searchBrave(ctx context.Context, query string, apiKey string) ([]SearchResult, error) {
	reqURL := "https://api.search.brave.com/res/v1/web/search?q=" + url.QueryEscape(query) + "&count=5"
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Subscription-Token", apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("brave api error: %d", resp.StatusCode)
	}

	var braveResp struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				Url         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&braveResp); err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, r := range braveResp.Web.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			Link:    r.Url,
			Snippet: r.Description,
		})
	}
	return results, nil
}

func searchDuckDuckGo(ctx context.Context, query string) ([]SearchResult, error) {
	// Use html.duckduckgo.com for lighter non-JS version
	reqURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	// Use a modern User-Agent to avoid being blocked or served mobile version
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ddg error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	html := string(body)

	// Updated Regex for current DDG HTML layout (2024/2025)
	// The results are typically in <div class="result ...">
	// Title link: <a class="result__a" href="...">
	// Snippet: <a class="result__snippet" ...>
	
	// Let's broaden the regex slightly to be more robust
	// Find result blocks first? No, regex on HTML is hard.
	// We will look for the specific class signatures which seem stable on the HTML version.

	// Pattern for result title link: <a class="result__a" href="(url)">(title)</a>
	// We use `(?s)` to allow dot to match newlines if needed, though usually on one line.
	reLink := regexp.MustCompile(`<a[^>]+class="[^"]*result__a[^"]*"[^>]+href="([^"]+)"[^>]*>([^<]+)</a>`)
	
	// Pattern for snippet: <a class="result__snippet" ...>(text)</a>
	reSnippet := regexp.MustCompile(`<a[^>]+class="[^"]*result__snippet[^"]*"[^>]*>([^<]+)</a>`)

	linkMatches := reLink.FindAllStringSubmatch(html, 10)
	snippetMatches := reSnippet.FindAllStringSubmatch(html, 10)

	var results []SearchResult
	for i, match := range linkMatches {
		if i >= 5 { break }
		
		rawLink := match[1]
		title := match[2]
		
		// Decode URL if it is a DDG redirect (//duckduckgo.com/l/?kh=-1&uddg=...)
		decodedLink := rawLink
		if strings.Contains(rawLink, "uddg=") {
			if u, err := url.Parse(rawLink); err == nil {
				if val := u.Query().Get("uddg"); val != "" {
					decodedLink = val
				}
			}
		}

		snippet := ""
		if i < len(snippetMatches) {
			snippet = snippetMatches[i][1]
		}

		// Simple HTML decoding
		title = strings.TrimSpace(title)
		snippet = strings.TrimSpace(snippet)
		
		// Remove bold tags
		title = strings.ReplaceAll(title, "<b>", "")
		title = strings.ReplaceAll(title, "</b>", "")
		snippet = strings.ReplaceAll(snippet, "<b>", "")
		snippet = strings.ReplaceAll(snippet, "</b>", "")

		if title != "" && decodedLink != "" {
			results = append(results, SearchResult{
				Title:   title,
				Link:    decodedLink,
				Snippet: snippet,
			})
		}
	}

	if len(results) == 0 {
		// Log the HTML snippet for debugging if we fail (would go to logger ideally)
		// For now, return error with hint
		return nil, fmt.Errorf("no results found on DuckDuckGo (layout likely changed or blocked)")
	}

	return results, nil
}
