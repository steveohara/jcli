package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
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
