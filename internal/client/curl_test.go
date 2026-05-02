package client

import (
	"strings"
	"testing"
)

func TestShellQuote_NoSpecialChars(t *testing.T) {
	got := shellQuote("hello world")
	want := "'hello world'"
	if got != want {
		t.Errorf("shellQuote = %q, want %q", got, want)
	}
}

func TestShellQuote_ContainsSingleQuote(t *testing.T) {
	// e.g. a JQL value like: project = IM and 'Cause Team' is not EMPTY
	got := shellQuote("'Cause Team'")
	// The single quotes inside must be escaped as '\''
	if !strings.Contains(got, `'\''`) {
		t.Errorf("single quotes not escaped correctly: %s", got)
	}
	if strings.Count(got, "'") < 4 {
		t.Errorf("expected escaped single quotes in: %s", got)
	}
}

func TestFormatCurl_GET(t *testing.T) {
	headers := []header{
		{"Authorization", "Bearer mytoken"},
		{"Accept", "application/json"},
	}
	got := formatCurl("GET", "https://jira.example.com/rest/api/2/issue/PROJ-1", headers, nil, false)

	if !strings.Contains(got, "curl -s -X GET") {
		t.Errorf("missing method: %s", got)
	}
	if !strings.Contains(got, "-H 'Authorization: Bearer mytoken'") {
		t.Errorf("missing auth header: %s", got)
	}
	if !strings.Contains(got, "-H 'Accept: application/json'") {
		t.Errorf("missing accept header: %s", got)
	}
	if !strings.Contains(got, "'https://jira.example.com/rest/api/2/issue/PROJ-1'") {
		t.Errorf("missing URL: %s", got)
	}
	if strings.Contains(got, "--data-raw") {
		t.Errorf("GET should not have --data-raw: %s", got)
	}
}

func TestFormatCurl_POST_WithBody(t *testing.T) {
	headers := []header{
		{"Authorization", "Bearer mytoken"},
		{"Accept", "application/json"},
		{"Content-Type", "application/json"},
	}
	body := []byte(`{"jql":"project = PROJ","maxResults":50}`)
	got := formatCurl("POST", "https://jira.example.com/rest/api/2/search", headers, body, false)

	if !strings.Contains(got, "-X POST") {
		t.Errorf("missing POST method: %s", got)
	}
	if !strings.Contains(got, "--data-raw") {
		t.Errorf("missing --data-raw: %s", got)
	}
	if !strings.Contains(got, "project = PROJ") {
		t.Errorf("body content missing from output: %s", got)
	}
	if !strings.Contains(got, "-H 'Content-Type: application/json'") {
		t.Errorf("missing content-type header: %s", got)
	}
}

func TestFormatCurl_Insecure(t *testing.T) {
	headers := []header{{"Authorization", "Bearer tok"}}
	got := formatCurl("GET", "https://jira.internal/rest/api/2/myself", headers, nil, true)

	if !strings.Contains(got, "-k") {
		t.Errorf("expected -k flag for insecure: %s", got)
	}
}

func TestFormatCurl_MultiLine(t *testing.T) {
	headers := []header{
		{"Authorization", "Bearer tok"},
		{"Accept", "application/json"},
	}
	got := formatCurl("GET", "https://example.com/api", headers, nil, false)

	lines := strings.Split(got, "\n")
	if len(lines) < 3 {
		t.Errorf("expected multi-line output, got:\n%s", got)
	}
	// All lines except the last must end with backslash continuation.
	for i, line := range lines[:len(lines)-1] {
		if !strings.HasSuffix(line, `\`) {
			t.Errorf("line %d missing backslash continuation: %q", i, line)
		}
	}
}

func TestFormatCurl_SingleQuoteInJQL(t *testing.T) {
	// Reproduces: --jql "project = IM and 'Cause Team' is not EMPTY"
	headers := []header{
		{"Authorization", "Bearer tok"},
		{"Accept", "application/json"},
		{"Content-Type", "application/json"},
	}
	body := []byte(`{"jql":"project = IM and 'Cause Team' is not EMPTY","maxResults":100}`)
	got := formatCurl("POST", "https://jira.example.com/rest/api/2/search", headers, body, false)

	// The single quotes in the JQL body must be escaped so the shell command is valid.
	if strings.Contains(got, `"'Cause Team'"`) {
		t.Errorf("unescaped single quotes inside shell-quoted string: %s", got)
	}
}
