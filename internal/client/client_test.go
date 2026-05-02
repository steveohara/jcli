package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/steveohara/jcli/internal/client"
	"github.com/steveohara/jcli/internal/config"
)

// newTestClient creates a Client that points to the provided test server.
func newTestClient(server string) *client.Client {
	return client.New(&config.Config{
		Server: server,
		Token:  "test-token",
	})
}

func TestGetIssue_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/issue/PROJ-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":  "10001",
			"key": "PROJ-1",
			"fields": map[string]interface{}{
				"summary": "Test issue",
				"status":  map[string]string{"name": "Open"},
			},
		})
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	issue, err := c.GetIssue(context.Background(), "PROJ-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Key != "PROJ-1" {
		t.Errorf("key = %q, want %q", issue.Key, "PROJ-1")
	}
	if issue.Fields.Summary != "Test issue" {
		t.Errorf("summary = %q, want %q", issue.Fields.Summary, "Test issue")
	}
}

func TestGetIssue_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["Issue does not exist"]}`))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.GetIssue(context.Background(), "FAKE-999", nil)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestSearchIssues(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total":      2,
			"startAt":    0,
			"maxResults": 50,
			"issues": []map[string]interface{}{
				{"id": "1", "key": "PROJ-1", "fields": map[string]interface{}{"summary": "First"}},
				{"id": "2", "key": "PROJ-2", "fields": map[string]interface{}{"summary": "Second"}},
			},
		})
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	result, err := c.SearchIssues(context.Background(), client.SearchOptions{JQL: "project = PROJ"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	if len(result.Issues) != 2 {
		t.Errorf("issues len = %d, want 2", len(result.Issues))
	}
}

func TestAddComment(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "100",
			"body": "Hello world",
		})
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	comment, err := c.AddComment(context.Background(), "PROJ-1", "Hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if comment.ID != "100" {
		t.Errorf("comment id = %q, want %q", comment.ID, "100")
	}
}

func TestGetProjects(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "10000", "key": "PROJ", "name": "My Project"},
		})
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	projects, err := c.GetProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 1 || projects[0].Key != "PROJ" {
		t.Errorf("unexpected projects: %+v", projects)
	}
}

func TestGetBoards(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"maxResults": 50,
			"startAt":    0,
			"total":      1,
			"isLast":     true,
			"values": []map[string]interface{}{
				{"id": 1, "name": "Sprint Board", "type": "scrum"},
			},
		})
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	result, err := c.GetBoards(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Values) != 1 {
		t.Errorf("board count = %d, want 1", len(result.Values))
	}
}

func TestGetIssue_HTMLErrorResponse(t *testing.T) {
	htmlBody := `<!DOCTYPE html>
<html>
<head><title>503 Service Unavailable</title></head>
<body>
  <h1>Service Unavailable</h1>
  <p>The server is temporarily unable to service your request.</p>
</body>
</html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(htmlBody))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.GetIssue(context.Background(), "PROJ-1", nil)
	if err == nil {
		t.Fatal("expected error for 503 HTML response")
	}
	msg := err.Error()
	if strings.Contains(msg, "<html") || strings.Contains(msg, "<body") || strings.Contains(msg, "<p>") {
		t.Errorf("error message contains raw HTML tags: %s", msg)
	}
	if !strings.Contains(msg, "503") {
		t.Errorf("error message missing status code: %s", msg)
	}
	if !strings.Contains(msg, "Service Unavailable") {
		t.Errorf("error message missing page content: %s", msg)
	}
}

func TestCreateIssue(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "10001",
			"key":  "PROJ-1",
			"self": "https://jira.example.com/rest/api/2/issue/10001",
		})
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	req := &client.CreateIssueRequest{
		Fields: client.CreateIssueFields{
			Project:   client.IDObj{Key: "PROJ"},
			Summary:   "New issue",
			IssueType: client.NamedObj{Name: "Bug"},
		},
	}
	resp, err := c.CreateIssue(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Key != "PROJ-1" {
		t.Errorf("key = %q, want %q", resp.Key, "PROJ-1")
	}
}

// TestIssueFields_CustomFieldRoundTrip verifies that custom fields captured in
// IssueFields.Extra during UnmarshalJSON are re-emitted by MarshalJSON so that
// --output json includes them.
func TestIssueFields_CustomFieldRoundTrip(t *testing.T) {
	raw := `{
		"summary": "Test",
		"status": {"name": "Open"},
		"customfield_10001": {"name": "Sprint 5"},
		"customfield_10002": "some string value"
	}`

	var fields client.IssueFields
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}

	if fields.Summary != "Test" {
		t.Errorf("Summary = %q, want %q", fields.Summary, "Test")
	}
	if len(fields.Extra) != 2 {
		t.Errorf("Extra len = %d, want 2; Extra = %v", len(fields.Extra), fields.Extra)
	}

	// Re-marshal and verify custom fields are present in the output.
	out, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	outStr := string(out)
	if !strings.Contains(outStr, "customfield_10001") {
		t.Errorf("marshaled JSON missing customfield_10001: %s", outStr)
	}
	if !strings.Contains(outStr, "Sprint 5") {
		t.Errorf("marshaled JSON missing Sprint 5 value: %s", outStr)
	}
	if !strings.Contains(outStr, "customfield_10002") {
		t.Errorf("marshaled JSON missing customfield_10002: %s", outStr)
	}
	if !strings.Contains(outStr, "some string value") {
		t.Errorf("marshaled JSON missing string value: %s", outStr)
	}
	// Known fields must also be present.
	if !strings.Contains(outStr, `"summary"`) {
		t.Errorf("marshaled JSON missing summary field: %s", outStr)
	}
}
