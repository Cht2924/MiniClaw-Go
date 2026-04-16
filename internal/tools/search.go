package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"miniclaw-go/internal/core"
)

func RegisterSearchTool(reg *Registry) {
	reg.Register(core.ToolDescriptor{
		Name:        "search_web",
		Description: "Search the web for a query and return a short list of result titles, URLs, and snippets.",
		Source:      "native",
		InputSchema: schema(
			prop("query", "string", "Search query."),
			required("query"),
		),
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		var input struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(args, &input); err != nil {
			return "", err
		}
		query := strings.TrimSpace(input.Query)
		if query == "" {
			return "", fmt.Errorf("query is required")
		}

		results, err := searchWeb(ctx, query)
		if err != nil {
			return "", err
		}
		if len(results) == 0 {
			return "no search results found", nil
		}

		var b strings.Builder
		for i, item := range results {
			b.WriteString(fmt.Sprintf("%d. %s\nURL: %s\nSnippet: %s\n\n", i+1, item.Title, item.URL, item.Snippet))
		}
		return strings.TrimSpace(truncate(b.String(), 8000)), nil
	})
}

type searchResult struct {
	Title   string
	URL     string
	Snippet string
}

type searchProvider struct {
	Endpoint       string
	AcceptLanguage string
	Parse          func(htmlText string, baseURL string) []searchResult
}

func searchWeb(ctx context.Context, query string) ([]searchResult, error) {
	providers := []searchProvider{
		{
			Endpoint:       "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query),
			AcceptLanguage: "en-US,en;q=0.9",
			Parse:          parseDuckDuckGoResults,
		},
		{
			Endpoint:       "https://www.sogou.com/web?query=" + url.QueryEscape(query),
			AcceptLanguage: "zh-CN,zh;q=0.9",
			Parse:          parseSogouResults,
		},
	}

	client := &http.Client{Timeout: 15 * time.Second}
	for _, provider := range providers {
		results, err := runSearchProvider(ctx, client, provider)
		if err != nil {
			continue
		}
		if len(results) > 0 {
			return results, nil
		}
	}
	return nil, nil
}

func runSearchProvider(ctx context.Context, client *http.Client, provider searchProvider) ([]searchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.Endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", provider.AcceptLanguage)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("search provider returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, err
	}
	return provider.Parse(string(body), provider.Endpoint), nil
}

func parseSogouResults(htmlText string, baseURL string) []searchResult {
	// Sogou result: <div class="vrwrap" id="sogou_vr_...">...<h3 class="vr-title"><a href="URL">Title</a></h3>...<div class="space-txt">Snippet</div>
	resultRe := regexp.MustCompile(`(?is)<div[^>]*class="vrwrap[^"]*"[^>]*>.*?<h3[^>]*class="vr-title"[^>]*>.*?<a[^>]+href="([^"]+)"[^>]*>(.*?)</a>.*?<div[^>]*class="[^"]*space-txt[^"]*"[^>]*>(.*?)</div>`)
	tagRe := regexp.MustCompile(`(?is)<[^>]+>`)
	spaceRe := regexp.MustCompile(`\s+`)

	matches := resultRe.FindAllStringSubmatch(htmlText, 10)

	results := make([]searchResult, 0, len(matches))
	for _, match := range matches {
		url := normalizeSearchURL(html.UnescapeString(match[1]), baseURL)
		title := cleanHTMLText(match[2], tagRe, spaceRe)
		snippet := cleanHTMLText(match[3], tagRe, spaceRe)

		// Skip empty results
		if title == "" || url == "" {
			continue
		}

		results = append(results, searchResult{
			Title:   title,
			URL:     url,
			Snippet: snippet,
		})
	}
	return results
}

func parseDuckDuckGoResults(htmlText string, baseURL string) []searchResult {
	blockRe := regexp.MustCompile(`(?is)<div[^>]*class="result[^"]*"[^>]*>.*?</div>\s*</div>`)
	linkRe := regexp.MustCompile(`(?is)<a[^>]*class="result__a"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	snippetRe := regexp.MustCompile(`(?is)<a[^>]*class="result__snippet"[^>]*>(.*?)</a>|<div[^>]*class="result__snippet"[^>]*>(.*?)</div>`)
	tagRe := regexp.MustCompile(`(?is)<[^>]+>`)
	spaceRe := regexp.MustCompile(`\s+`)

	blocks := blockRe.FindAllString(htmlText, 10)
	if len(blocks) == 0 {
		// Fallback to parsing standalone result links if the outer container changes.
		blocks = regexp.MustCompile(`(?is)<a[^>]*class="result__a"[^>]*href="[^"]+"[^>]*>.*?</a>`).FindAllString(htmlText, 10)
	}

	results := make([]searchResult, 0, len(blocks))
	for _, block := range blocks {
		linkMatch := linkRe.FindStringSubmatch(block)
		if len(linkMatch) == 0 {
			continue
		}

		snippetMatch := snippetRe.FindStringSubmatch(block)
		snippet := ""
		if len(snippetMatch) > 0 {
			snippet = firstNonEmptySearch(snippetMatch[1], snippetMatch[2])
		}

		item := searchResult{
			Title:   cleanHTMLText(linkMatch[2], tagRe, spaceRe),
			URL:     normalizeSearchURL(html.UnescapeString(linkMatch[1]), baseURL),
			Snippet: cleanHTMLText(snippet, tagRe, spaceRe),
		}
		if item.Title == "" || item.URL == "" {
			continue
		}
		results = append(results, item)
	}
	return results
}

func cleanHTMLText(value string, tagRe, spaceRe *regexp.Regexp) string {
	value = html.UnescapeString(value)
	value = tagRe.ReplaceAllString(value, " ")
	value = strings.TrimSpace(spaceRe.ReplaceAllString(value, " "))
	return value
}

func normalizeSearchURL(raw string, baseURL string) string {
	raw = strings.TrimSpace(html.UnescapeString(raw))
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}

	if decoded := decodeSearchRedirectURL(raw); decoded != "" {
		return decoded
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	if baseURL == "" {
		return ""
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return base.ResolveReference(parsed).String()
}

func decodeSearchRedirectURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	query := parsed.Query()
	for _, key := range []string{"uddg", "url", "target"} {
		value := strings.TrimSpace(query.Get(key))
		if value == "" {
			continue
		}
		decoded, err := url.QueryUnescape(value)
		if err == nil {
			value = decoded
		}
		if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
			return value
		}
	}
	return ""
}

func firstNonEmptySearch(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
