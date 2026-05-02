package client

import (
	"strings"
	"testing"
)

func TestIsHTML_ContentType(t *testing.T) {
	cases := []struct {
		ct   string
		want bool
	}{
		{"text/html; charset=utf-8", true},
		{"TEXT/HTML", true},
		{"application/json", false},
		{"", false},
	}
	for _, tc := range cases {
		got := isHTML(tc.ct, []byte("<p>hi</p>"))
		if got != tc.want {
			t.Errorf("isHTML(%q, ...) = %v, want %v", tc.ct, got, tc.want)
		}
	}
}

func TestIsHTML_Sniff(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{"<!DOCTYPE html><html>", true},
		{"<!doctype html><html>", true},
		{"<html><head></head>", true},
		{`{"errorMessages":["bad"]}`, false},
		{"plain text response", false},
	}
	for _, tc := range cases {
		got := isHTML("", []byte(tc.body))
		if got != tc.want {
			t.Errorf("isHTML(%q) = %v, want %v", tc.body[:min(len(tc.body), 30)], got, tc.want)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestHTMLToText_BasicPage(t *testing.T) {
	html := []byte(`<!DOCTYPE html>
<html>
<head><title>503 Service Unavailable</title><style>body{color:red}</style></head>
<body>
  <h1>Service Unavailable</h1>
  <p>The server is temporarily unable to service your request. Please try again later.</p>
  <script>alert('hi')</script>
</body>
</html>`)

	got := htmlToText(html, 10)

	if !strings.Contains(got, "503 Service Unavailable") {
		t.Errorf("expected title in output, got:\n%s", got)
	}
	if !strings.Contains(got, "Service Unavailable") {
		t.Errorf("expected h1 in output, got:\n%s", got)
	}
	if !strings.Contains(got, "temporarily unable") {
		t.Errorf("expected paragraph text in output, got:\n%s", got)
	}
	if strings.Contains(got, "<") || strings.Contains(got, ">") {
		t.Errorf("output still contains HTML tags:\n%s", got)
	}
	if strings.Contains(got, "alert") {
		t.Errorf("script content leaked into output:\n%s", got)
	}
}

func TestHTMLToText_401Page(t *testing.T) {
	html := []byte(`<html>
<head><title>Unauthorized</title></head>
<body><h1>401 Unauthorized</h1><p>You must authenticate to access this resource.</p></body>
</html>`)

	got := htmlToText(html, 10)

	if !strings.Contains(got, "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in output, got:\n%s", got)
	}
	if strings.Contains(got, "<") {
		t.Errorf("output contains raw HTML:\n%s", got)
	}
}

func TestHTMLToText_EntityDecoding(t *testing.T) {
	html := []byte(`<html><body><p>AT&amp;T &lt;rocks&gt; &#39;yeah&#39;</p></body></html>`)
	got := htmlToText(html, 10)

	if !strings.Contains(got, "AT&T") {
		t.Errorf("expected decoded &amp; in output, got: %s", got)
	}
	if !strings.Contains(got, "<rocks>") {
		t.Errorf("expected decoded &lt;&gt; in output, got: %s", got)
	}
	if strings.Contains(got, "&amp;") || strings.Contains(got, "&lt;") {
		t.Errorf("entities not decoded in output: %s", got)
	}
}

func TestHTMLToText_MaxLines(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("<html><body>")
	for i := 0; i < 20; i++ {
		sb.WriteString("<p>Line of content here</p>")
	}
	sb.WriteString("</body></html>")

	got := htmlToText([]byte(sb.String()), 3)
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) > 3 {
		t.Errorf("expected at most 3 lines, got %d:\n%s", len(lines), got)
	}
}

func TestHTMLToText_AnchorTextStripped(t *testing.T) {
	html := []byte(`<html><body>
<p>Authentication failed. Please contact your administrator for more details.</p>
<a href="/">Go to Jira home</a>
</body></html>`)

	got := htmlToText(html, 10)

	if strings.Contains(got, "Go to Jira home") {
		t.Errorf("anchor/navigation text should be stripped, got:\n%s", got)
	}
	if !strings.Contains(got, "Authentication failed") {
		t.Errorf("expected paragraph text in output, got:\n%s", got)
	}
}

func TestHTMLToText_OrphanedLabelStripped(t *testing.T) {
	html := []byte(`<html><body>
<div class="warning"><span>Warning:</span>
<p>Encountered a 403 Forbidden error.</p>
</div>
</body></html>`)

	got := htmlToText(html, 10)

	// "Warning:" on its own line should be dropped
	for _, line := range strings.Split(got, "\n") {
		if strings.TrimSpace(line) == "Warning:" {
			t.Errorf("orphaned label 'Warning:' should be filtered out, got:\n%s", got)
		}
	}
	if !strings.Contains(got, "403 Forbidden") {
		t.Errorf("expected error text in output, got:\n%s", got)
	}
}

func TestHTMLToText_FullJira403Page(t *testing.T) {
	// Approximates the real Jira 403 error page structure.
	page := []byte(`<!DOCTYPE html>
<html>
<head><title>Forbidden (403)</title><style>body{font-family:sans-serif}</style></head>
<body>
  <div class="error-page">
    <h1>Encountered a "403 - Forbidden" error while loading this page.</h1>
    <div class="warning"><span>Warning:</span>
      <p>Authentication failed. Please contact your administrator for more details.</p>
    </div>
    <p><a href="/">Go to Jira home</a></p>
  </div>
  <script>trackError(403)</script>
</body>
</html>`)

	got := htmlToText(page, 10)

	if strings.Contains(got, "<") || strings.Contains(got, ">") {
		t.Errorf("output contains HTML tags:\n%s", got)
	}
	if strings.Contains(got, "Go to Jira home") {
		t.Errorf("navigation link text should be stripped:\n%s", got)
	}
	if strings.Contains(got, "Warning:") && !strings.Contains(got, "Warning: ") {
		t.Errorf("orphaned 'Warning:' label should be stripped:\n%s", got)
	}
	if !strings.Contains(got, "Forbidden") {
		t.Errorf("expected 'Forbidden' in output:\n%s", got)
	}
	if !strings.Contains(got, "Authentication failed") {
		t.Errorf("expected 'Authentication failed' in output:\n%s", got)
	}
}

func TestHTMLToText_EmptyBody(t *testing.T) {
	got := htmlToText([]byte(`<html><body></body></html>`), 10)
	if got != "" {
		t.Errorf("expected empty output for empty body, got: %q", got)
	}
}
