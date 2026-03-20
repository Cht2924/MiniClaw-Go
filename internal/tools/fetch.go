package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"miniclaw-go/internal/core"
)

func RegisterFetchTool(reg *Registry) {
	reg.Register(core.ToolDescriptor{
		Name:        "fetch_url",
		Description: "Fetch a URL and return cleaned text content.",
		Source:      "native",
		InputSchema: schema(
			prop("url", "string", "URL to fetch."),
			required("url"),
		),
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		var input struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(args, &input); err != nil {
			return "", err
		}
		if !strings.HasPrefix(input.URL, "http://") && !strings.HasPrefix(input.URL, "https://") {
			return "", fmt.Errorf("url must start with http:// or https://")
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, input.URL, nil)
		if err != nil {
			return "", err
		}
		client := &http.Client{Timeout: 12 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
		if err != nil {
			return "", err
		}
		text := cleanHTML(string(data))
		return truncate(text, 8000), nil
	})
}

func cleanHTML(s string) string {
	reScript := regexp.MustCompile(`(?is)<script.*?>.*?</script>`)
	reStyle := regexp.MustCompile(`(?is)<style.*?>.*?</style>`)
	reTag := regexp.MustCompile(`(?is)<[^>]+>`)
	reSpace := regexp.MustCompile(`\s+`)

	s = reScript.ReplaceAllString(s, " ")
	s = reStyle.ReplaceAllString(s, " ")
	s = reTag.ReplaceAllString(s, " ")
	s = strings.TrimSpace(reSpace.ReplaceAllString(s, " "))
	return s
}
