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
	// User-Agent is often required to avoid 403
	req.Header.Set("User-Agent", "Mozilla/5.0 (Compatible; auleOS/1.0)")

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

	// Regex scraping (brittle but standard lib only)
	// Structure typically: <a class="result__a" href="...">TITLE</a>
	// Snippet: <a class="result__snippet" ...>SNIPPET</a>
	
	// Let's look for result__body elements
	// This is very rough scraping.
	// Pattern for link and title: <a class="result__a" href="([^"]+)">([^<]+)</a>
	// Pattern for snippet: <a class="result__snippet" href="[^"]+">([^<]+)</a>
	
	// NOTE: Parsing HTML with regex is bad practice, but without x/net/html, it's our only zero-dependency option.
	// We will try to match "result__a" links.
	
	var results []SearchResult
	
	// Create regex for result items
	// We iterate over the "result" divs if possible, or just global match
	// Global match:
	reLink := regexp.MustCompile(`<a class="result__a" href="([^"]+)">([^<]+)</a>`)
	reSnippet := regexp.MustCompile(`<a class="result__snippet" href="[^"]+">([^<]+)</a>`)

	// Find all submatches
	linkMatches := reLink.FindAllStringSubmatch(html, 10) // Limit to 10
	snippetMatches := reSnippet.FindAllStringSubmatch(html, 10)

	for i, match := range linkMatches {
		if i >= 5 { break } // Top 5
		
		link := match[1]
		title := match[2]
		
		// Clean up DDG redirects in links (sometimes they are /l/?kh=...)
		// Actually html.duckduckgo.com usually returns direct links or u/ links.
		// Let's decode URL if needed.
		if strings.Contains(link, "duckduckgo.com/l/?") {
			// Extract real URL if possible, or leave it.
			// Ideally we use url.ParseQuery.
		}

		snippet := ""
		if i < len(snippetMatches) {
			snippet = snippetMatches[i][1]
		}

		// Simple HTML Entity decoding (basic)
		title = strings.ReplaceAll(title, "<b>", "")
		title = strings.ReplaceAll(title, "</b>", "")
		snippet = strings.ReplaceAll(snippet, "<b>", "")
		snippet = strings.ReplaceAll(snippet, "</b>", "")
		
		results = append(results, SearchResult{
			Title:   title,
			Link:    link,
			Snippet: snippet,
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results found on DuckDuckGo (layout likely changed)")
	}

	return results, nil
}
