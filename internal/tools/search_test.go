package tools

import "testing"

func TestParseDuckDuckGoResultsDecodesRedirectURLs(t *testing.T) {
	html := `
<div class="result results_links results_links_deep web-result ">
  <div class="links_main links_deep result__body">
    <h2 class="result__title">
      <a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgithub.com%2Fbrowser-use%2Fbrowser-use">browser-use</a>
    </h2>
    <a class="result__snippet">Web AI agent for browser tasks.</a>
  </div>
</div>`

	results := parseDuckDuckGoResults(html, "https://html.duckduckgo.com/html/?q=agent")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].URL != "https://github.com/browser-use/browser-use" {
		t.Fatalf("unexpected url: %q", results[0].URL)
	}
	if results[0].Title != "browser-use" {
		t.Fatalf("unexpected title: %q", results[0].Title)
	}
}

func TestNormalizeSearchURLResolvesRelativeLinks(t *testing.T) {
	got := normalizeSearchURL("/link?url=token", "https://www.sogou.com/web?query=agent")
	want := "https://www.sogou.com/link?url=token"
	if got != want {
		t.Fatalf("unexpected normalized url: got %q want %q", got, want)
	}
}
