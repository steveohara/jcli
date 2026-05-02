package issue

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/steveohara/jcli/cmd"
	"github.com/steveohara/jcli/internal/client"
)

// TestMain wires IssueCmd into the root command once for all tests in this
// package.  This mirrors what main() does in the binary.
func TestMain(m *testing.M) {
	cmd.RootCmd.AddCommand(IssueCmd)
	os.Exit(m.Run())
}

// resetCmdFlags resets all issue-command package-level flag variables to their
// zero / default values so that successive tests do not bleed into each other
// (pflag does not reset values between Execute() calls).
func resetCmdFlags() {
	getFields = nil
	getAllFields = false
	searchJQL = ""
	searchFields = nil
	searchStartAt = 0
	searchMaxResults = 50
	searchPage = 0
	searchAll = false
	searchAllFields = false
	deleteSubtasks = false
	createSummary = ""
	createDescription = ""
	createType = "Task"
	createPriority = ""
	createAssignee = ""
	createLabels = nil
	createComponents = nil
	createFixVersions = nil
	createDueDate = ""
	createParent = ""
	createProject = ""
	updateSummary = ""
	updateDescription = ""
	updatePriority = ""
	updateAssignee = ""
	updateDueDate = ""
	updateLabels = nil
}

// runCmd executes a jcli command against serverURL, captures stdout, and
// returns the captured text and any execution error.
func runCmd(t *testing.T, serverURL string, args ...string) (string, error) {
	t.Helper()
	resetCmdFlags()

	fullArgs := append([]string{"--server", serverURL, "--token", "test-token"}, args...)
	cmd.RootCmd.SetArgs(fullArgs)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	oldOut := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldOut })

	execErr := cmd.RootCmd.Execute()

	w.Close()
	os.Stdout = oldOut

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String(), execErr
}

// issuePayload returns a minimal Jira issue JSON object with sane defaults.
// The caller may override any field via the extra map.
func issuePayload(key string, extra map[string]interface{}) map[string]interface{} {
	fields := map[string]interface{}{
		"summary":     "Default summary",
		"issuetype":   map[string]string{"name": "Bug"},
		"status":      map[string]string{"name": "Open"},
		"priority":    map[string]string{"name": "Medium"},
		"assignee":    nil,
		"reporter":    nil,
		"project":     map[string]string{"key": "PROJ"},
		"created":     "2024-01-01T00:00:00.000Z",
		"updated":     "2024-01-02T00:00:00.000Z",
		"duedate":     "",
		"labels":      []string{},
		"description": "",
	}
	for k, v := range extra {
		fields[k] = v
	}
	return map[string]interface{}{"id": "10001", "key": key, "fields": fields}
}

// -----------------------------------------------------------------------
// Column / row metadata
// -----------------------------------------------------------------------

func TestDefaultSearchColumns_Definition(t *testing.T) {
	wantFields := []string{"issuetype", "priority", "status", "assignee", "summary"}
	if len(defaultSearchColumns) != len(wantFields) {
		t.Fatalf("expected %d search columns, got %d", len(wantFields), len(defaultSearchColumns))
	}
	for i, col := range defaultSearchColumns {
		if col.field != wantFields[i] {
			t.Errorf("col[%d].field = %q, want %q", i, col.field, wantFields[i])
		}
		if col.header == "" {
			t.Errorf("col[%d].header is empty", i)
		}
		if col.extract == nil {
			t.Errorf("col[%d].extract is nil", i)
		}
	}
}

func TestDefaultSearchColumns_Extractors(t *testing.T) {
	issue := client.Issue{
		Key: "PROJ-1",
		Fields: client.IssueFields{
			Summary:   "Test issue",
			IssueType: client.NamedObj{Name: "Bug"},
			Status:    client.NamedObj{Name: "Open"},
			Priority:  client.NamedObj{Name: "High"},
			Assignee:  &client.User{DisplayName: "Alice"},
		},
	}
	want := map[string]string{
		"issuetype": "Bug",
		"priority":  "High",
		"status":    "Open",
		"assignee":  "Alice",
		"summary":   "Test issue",
	}
	for _, col := range defaultSearchColumns {
		got := col.extract(issue)
		if exp, ok := want[col.field]; ok && got != exp {
			t.Errorf("extract(%q) = %q, want %q", col.field, got, exp)
		}
	}
}

func TestAllIssueKVRows_Definition(t *testing.T) {
	wantFields := []string{
		"summary", "issuetype", "status", "priority", "assignee",
		"reporter", "project", "created", "updated", "duedate", "labels", "description",
	}
	if len(allIssueKVRows) != len(wantFields) {
		t.Fatalf("expected %d KV rows, got %d", len(wantFields), len(allIssueKVRows))
	}
	for i, row := range allIssueKVRows {
		if row.field != wantFields[i] {
			t.Errorf("row[%d].field = %q, want %q", i, row.field, wantFields[i])
		}
		if row.label == "" {
			t.Errorf("row[%d].label is empty", i)
		}
		if row.extract == nil {
			t.Errorf("row[%d].extract is nil", i)
		}
	}
}

// -----------------------------------------------------------------------
// issue get
// -----------------------------------------------------------------------

func newIssueGetServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(issuePayload("PROJ-1", map[string]interface{}{
			"summary":   "Fix the login bug",
			"issuetype": map[string]string{"name": "Bug"},
			"status":    map[string]string{"name": "In Progress"},
			"priority":  map[string]string{"name": "High"},
			"assignee":  map[string]string{"displayName": "Alice"},
			"reporter":  map[string]string{"displayName": "Bob"},
			"project":   map[string]string{"key": "PROJ"},
		}))
	}))
}

func TestIssueGetCommand_AllFields(t *testing.T) {
	ts := newIssueGetServer(t)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "get", "PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"PROJ-1", "Fix the login bug", "Bug", "In Progress", "High", "Alice", "Bob", "PROJ"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestIssueGetCommand_FilteredFields(t *testing.T) {
	ts := newIssueGetServer(t)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "get", "PROJ-1", "--fields", "summary,status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Key / ID always shown.
	if !strings.Contains(out, "PROJ-1") {
		t.Errorf("output missing key PROJ-1:\n%s", out)
	}
	// Requested fields shown.
	if !strings.Contains(out, "Fix the login bug") {
		t.Errorf("output missing Summary value:\n%s", out)
	}
	if !strings.Contains(out, "In Progress") {
		t.Errorf("output missing Status value:\n%s", out)
	}
	// Non-requested fields not shown.
	if strings.Contains(out, "Alice") {
		t.Errorf("output should not contain Assignee value:\n%s", out)
	}
	if strings.Contains(out, "Bob") {
		t.Errorf("output should not contain Reporter value:\n%s", out)
	}
}

func TestIssueGetCommand_CustomField(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(issuePayload("PROJ-1", map[string]interface{}{
			"customfield_10001": map[string]string{"name": "Sprint 42"},
		}))
	}))
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "get", "PROJ-1", "--fields", "customfield_10001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Sprint 42") {
		t.Errorf("output missing custom field value 'Sprint 42':\n%s", out)
	}
	if !strings.Contains(out, "customfield_10001") {
		t.Errorf("output missing custom field label:\n%s", out)
	}
}

func TestIssueGetCommand_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errorMessages":["Issue Does Not Exist"],"errors":{}}`))
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "get", "PROJ-999")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should reference HTTP 404: %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "hint") {
		t.Errorf("error should contain 404 hint: %v", err)
	}
}

// -----------------------------------------------------------------------
// issue search
// -----------------------------------------------------------------------

func newIssueSearchServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total": 2, "startAt": 0, "maxResults": 50,
			"issues": []interface{}{
				issuePayload("PROJ-1", map[string]interface{}{
					"summary":   "First issue",
					"issuetype": map[string]string{"name": "Bug"},
					"status":    map[string]string{"name": "Open"},
					"priority":  map[string]string{"name": "High"},
					"assignee":  map[string]string{"displayName": "Alice"},
				}),
				issuePayload("PROJ-2", map[string]interface{}{
					"summary":   "Second issue",
					"issuetype": map[string]string{"name": "Story"},
					"status":    map[string]string{"name": "Closed"},
					"priority":  map[string]string{"name": "Low"},
					"assignee":  nil,
				}),
			},
		})
	}))
}

func TestIssueSearchCommand_AllColumns(t *testing.T) {
	ts := newIssueSearchServer(t)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "search", "--jql", "project = PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{
		"PROJ-1", "PROJ-2",
		"First issue", "Second issue",
		"Bug", "Story", "Open", "Closed", "High", "Low", "Alice",
		"KEY", "TYPE", "PRIORITY", "STATUS", "ASSIGNEE", "SUMMARY",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestIssueSearchCommand_FilteredColumns(t *testing.T) {
	ts := newIssueSearchServer(t)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "search",
		"--jql", "project = PROJ", "--fields", "summary,status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// KEY always present.
	if !strings.Contains(out, "PROJ-1") {
		t.Errorf("output missing PROJ-1:\n%s", out)
	}
	// Requested column headers present.
	for _, h := range []string{"KEY", "SUMMARY", "STATUS"} {
		if !strings.Contains(out, h) {
			t.Errorf("output missing header %q:\n%s", h, out)
		}
	}
	// Non-requested column headers absent.
	for _, h := range []string{"TYPE", "PRIORITY", "ASSIGNEE"} {
		if strings.Contains(out, h) {
			t.Errorf("output should not contain header %q:\n%s", h, out)
		}
	}
}

func TestIssueSearchCommand_CustomField(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total": 1, "startAt": 0, "maxResults": 50,
			"issues": []interface{}{
				issuePayload("PROJ-1", map[string]interface{}{
					"customfield_10001": "Sprint 42",
				}),
			},
		})
	}))
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "search",
		"--jql", "project = PROJ", "--fields", "customfield_10001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Sprint 42") {
		t.Errorf("output missing custom field value 'Sprint 42':\n%s", out)
	}
	if !strings.Contains(strings.ToUpper(out), "CUSTOMFIELD") {
		t.Errorf("output missing custom field column header:\n%s", out)
	}
}

// -----------------------------------------------------------------------
// issue search – pagination
// -----------------------------------------------------------------------

func TestIssueSearchCommand_Page(t *testing.T) {
	var gotStartAt string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotStartAt = r.URL.Query().Get("startAt")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total": 100, "startAt": 50, "maxResults": 50, "issues": []interface{}{},
		})
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "search",
		"--jql", "project = PROJ", "--page", "2", "--max-results", "50")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotStartAt != "50" {
		t.Errorf("startAt = %q, want %q (page 2 with max-results 50)", gotStartAt, "50")
	}
}

func TestIssueSearchCommand_All(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		startAt := r.URL.Query().Get("startAt")
		var issues []interface{}
		switch startAt {
		case "", "0":
			issues = []interface{}{
				issuePayload("PROJ-1", nil),
				issuePayload("PROJ-2", nil),
			}
		case "2":
			issues = []interface{}{
				issuePayload("PROJ-3", nil),
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total": 3, "startAt": 0, "maxResults": 2, "issues": issues,
		})
	}))
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "search",
		"--jql", "project = PROJ", "--all", "--max-results", "2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 2 {
		t.Errorf("expected ≥2 API calls for --all, got %d", callCount)
	}
	for _, key := range []string{"PROJ-1", "PROJ-2", "PROJ-3"} {
		if !strings.Contains(out, key) {
			t.Errorf("output missing %q:\n%s", key, out)
		}
	}
}

// -----------------------------------------------------------------------
// issue create
// -----------------------------------------------------------------------

func TestIssueCreateCommand(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "10001", "key": "PROJ-42",
		})
	}))
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "create",
		"--project", "PROJ",
		"--summary", "New test issue",
		"--type", "Bug",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "PROJ-42") {
		t.Errorf("output missing created issue key PROJ-42:\n%s", out)
	}
}

// -----------------------------------------------------------------------
// issue update
// -----------------------------------------------------------------------

func TestIssueUpdateCommand(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "PROJ-1") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "update", "PROJ-1", "--summary", "Updated title")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIssueUpdateCommand_NoFlags(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "update", "PROJ-1")
	if err == nil {
		t.Fatal("expected error when no update flags are provided")
	}
}

// -----------------------------------------------------------------------
// JSON output – custom fields present
// -----------------------------------------------------------------------

func TestIssueGetCommand_JSONIncludesCustomFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(issuePayload("PROJ-1", map[string]interface{}{
			"summary":           "Bug report",
			"customfield_10001": map[string]string{"name": "Sprint 7"},
			"customfield_10002": "some-label",
		}))
	}))
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "get", "PROJ-1", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"customfield_10001", "Sprint 7", "customfield_10002", "some-label"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q:\n%s", want, out)
		}
	}
}

func TestIssueSearchCommand_JSONIncludesCustomFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total": 1, "startAt": 0, "maxResults": 50,
			"issues": []interface{}{
				issuePayload("PROJ-1", map[string]interface{}{
					"customfield_10001": "Sprint 9",
				}),
			},
		})
	}))
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "search", "--jql", "project = PROJ", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"customfield_10001", "Sprint 9"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q:\n%s", want, out)
		}
	}
}

// -----------------------------------------------------------------------
// --all-fields: empty / null fields present in JSON output
// -----------------------------------------------------------------------

// allFieldsPayload returns a Jira issue whose fields include null / empty
// values that omitempty would normally suppress.
func allFieldsPayload(key string) map[string]interface{} {
	return map[string]interface{}{
		"id":  "10001",
		"key": key,
		"fields": map[string]interface{}{
			"summary":     "Test issue",
			"description": "",
			"issuetype":   map[string]string{"name": "Bug"},
			"status":      map[string]string{"name": "Open"},
			"priority":    nil, // null – no priority set
			"assignee":    nil, // null – unassigned
			"reporter":    nil,
			"project":     map[string]string{"key": "PROJ"},
			"duedate":     "", // empty string
			"votes":       nil,
			"watches":     nil,
			"parent":      nil,
		},
	}
}

func TestIssueGetCommand_AllFieldsFlag_JSONIncludesNulls(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(allFieldsPayload("PROJ-1"))
	}))
	defer ts.Close()

	// Without --all-fields: null/empty fields absent.
	outCompact, err := runCmd(t, ts.URL, "issue", "get", "PROJ-1", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(outCompact, `"priority"`) {
		t.Errorf("compact JSON should not contain null priority:\n%s", outCompact)
	}
	if strings.Contains(outCompact, `"duedate"`) {
		t.Errorf("compact JSON should not contain empty duedate:\n%s", outCompact)
	}

	// With --all-fields: null/empty fields must appear.
	outFull, err := runCmd(t, ts.URL, "issue", "get", "PROJ-1", "--output", "json", "--all-fields")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{`"priority"`, `"assignee"`, `"duedate"`, `"votes"`, `"parent"`} {
		if !strings.Contains(outFull, want) {
			t.Errorf("--all-fields JSON missing %q:\n%s", want, outFull)
		}
	}
}

func TestIssueSearchCommand_AllFieldsFlag_JSONIncludesNulls(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total": 1, "startAt": 0, "maxResults": 50,
			"issues": []interface{}{allFieldsPayload("PROJ-1")},
		})
	}))
	defer ts.Close()

	// Without --all-fields: null priority absent.
	outCompact, err := runCmd(t, ts.URL, "issue", "search",
		"--jql", "project = PROJ", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(outCompact, `"priority":null`) {
		t.Errorf("compact JSON should not contain null priority:\n%s", outCompact)
	}

	// With --all-fields: null/empty fields must appear.
	outFull, err := runCmd(t, ts.URL, "issue", "search",
		"--jql", "project = PROJ", "--output", "json", "--all-fields")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{`"priority"`, `"assignee"`, `"duedate"`, `"votes"`, `"parent"`} {
		if !strings.Contains(outFull, want) {
			t.Errorf("--all-fields JSON missing %q:\n%s", want, outFull)
		}
	}
}

func TestIssueGetCommand_AllFieldsFlag_NoEffectOnTableOutput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(allFieldsPayload("PROJ-1"))
	}))
	defer ts.Close()

	// --all-fields must not break table output.
	out, err := runCmd(t, ts.URL, "issue", "get", "PROJ-1", "--all-fields")
	if err != nil {
		t.Fatalf("--all-fields with table output: %v", err)
	}
	if !strings.Contains(out, "PROJ-1") {
		t.Errorf("table output missing issue key:\n%s", out)
	}
}
