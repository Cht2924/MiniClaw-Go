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

		endpoint := "https://www.sogou.com/web?query=" + url.QueryEscape(query)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		if err != nil {
			return "", err
		}

		results := parseBingResults(string(body))
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

func parseBingResults(htmlText string) []searchResult {
	// Sogou result: <div class="vrwrap" id="sogou_vr_...">...<h3 class="vr-title"><a href="URL">Title</a></h3>...<div class="space-txt">Snippet</div>
	resultRe := regexp.MustCompile(`(?is)<div[^>]*class="vrwrap[^"]*"[^>]*>.*?<h3[^>]*class="vr-title"[^>]*>.*?<a[^>]+href="([^"]+)"[^>]*>(.*?)</a>.*?<div[^>]*class="[^"]*space-txt[^"]*"[^>]*>(.*?)</div>`)
	tagRe := regexp.MustCompile(`(?is)<[^>]+>`)
	spaceRe := regexp.MustCompile(`\s+`)

	matches := resultRe.FindAllStringSubmatch(htmlText, 10)

	results := make([]searchResult, 0, len(matches))
	for _, match := range matches {
		url := html.UnescapeString(match[1])
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

func cleanHTMLText(value string, tagRe, spaceRe *regexp.Regexp) string {
	value = html.UnescapeString(value)
	value = tagRe.ReplaceAllString(value, " ")
	value = strings.TrimSpace(spaceRe.ReplaceAllString(value, " "))
	return value
}

func normalizeSearchURL(raw string) string {
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	return raw
}

func firstNonEmptySearch(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
