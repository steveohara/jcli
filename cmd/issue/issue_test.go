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
	cmd.ResetGlobalFlags()
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
	createFields = ""
	createProperties = ""
	createHistory = ""
	updateSummary = ""
	updateDescription = ""
	updatePriority = ""
	updateAssignee = ""
	updateDueDate = ""
	updateLabels = nil
	updateFields = ""
	updateProperties = ""
	updateHistory = ""
	commentBody = ""
	commentIDFlag = ""
	transitionID = ""
	transitionResolution = ""
	assignAccountID = ""
	worklogTimeSpent = ""
	worklogStarted = ""
	worklogComment = ""
	worklogIDFlag = ""
	watchAccountID = ""
	linkTypeName = ""
	linkInward = ""
	linkOutward = ""
	linkComment = ""
	linkIDFlag = ""
	attachFilePath = ""
	attachDeleteID = ""
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

// -----------------------------------------------------------------------
// issue create -- request body verification
// -----------------------------------------------------------------------

// captureCreateBody returns a test server that captures the decoded JSON body
// of the first POST it receives into *body, then responds with a created key.
func captureCreateBody(t *testing.T, body *map[string]interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var b map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
				t.Errorf("decode body: %v", err)
			}
			*body = b
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","key":"PROJ-99"}`))
	}))
}

// captureUpdateBody returns a test server that captures the decoded JSON body
// of the first PUT it receives into *body, then responds 204 No Content.
func captureUpdateBody(t *testing.T, body *map[string]interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			var b map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
				t.Errorf("decode body: %v", err)
			}
			*body = b
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}

// fieldStr is a helper that returns the nested string value at
// body["fields"][key], or "" if missing.
func fieldStr(body map[string]interface{}, key string) string {
	fields, ok := body["fields"].(map[string]interface{})
	if !ok {
		return ""
	}
	v, _ := fields[key].(string)
	return v
}

// fieldObj returns body["fields"][key] as a map, or nil.
func fieldObj(body map[string]interface{}, key string) map[string]interface{} {
	fields, ok := body["fields"].(map[string]interface{})
	if !ok {
		return nil
	}
	v, _ := fields[key].(map[string]interface{})
	return v
}

func TestIssueCreateCommand_ConvenienceFlags_SentInBody(t *testing.T) {
	var body map[string]interface{}
	ts := captureCreateBody(t, &body)
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "create",
		"--project", "PROJ",
		"--summary", "My summary",
		"--type", "Story",
		"--priority", "High",
		"--description", "A description",
		"--assignee", "user-abc",
		"--due-date", "2024-06-01",
		"--labels", "backend,api",
		"--components", "10001",
		"--fix-versions", "10010",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if body == nil {
		t.Fatal("server received no body")
	}
	fields, _ := body["fields"].(map[string]interface{})
	if fields == nil {
		t.Fatal("body missing 'fields'")
	}

	cases := []struct {
		field string
		check func() bool
		desc  string
	}{
		{"summary", func() bool { return fields["summary"] == "My summary" }, `summary = "My summary"`},
		{"description", func() bool { return fields["description"] == "A description" }, `description = "A description"`},
		{"duedate", func() bool { return fields["duedate"] == "2024-06-01" }, `duedate = "2024-06-01"`},
		{"issuetype", func() bool {
			obj, _ := fields["issuetype"].(map[string]interface{})
			return obj != nil && obj["name"] == "Story"
		}, `issuetype.name = "Story"`},
		{"priority", func() bool {
			obj, _ := fields["priority"].(map[string]interface{})
			return obj != nil && obj["name"] == "High"
		}, `priority.name = "High"`},
		{"assignee", func() bool {
			obj, _ := fields["assignee"].(map[string]interface{})
			return obj != nil && obj["id"] == "user-abc"
		}, `assignee.id = "user-abc"`},
		{"labels", func() bool {
			arr, _ := fields["labels"].([]interface{})
			return len(arr) == 2 && arr[0] == "backend" && arr[1] == "api"
		}, `labels = ["backend","api"]`},
		{"components", func() bool {
			arr, _ := fields["components"].([]interface{})
			if len(arr) != 1 {
				return false
			}
			obj, _ := arr[0].(map[string]interface{})
			return obj != nil && obj["id"] == "10001"
		}, `components[0].id = "10001"`},
		{"fixVersions", func() bool {
			arr, _ := fields["fixVersions"].([]interface{})
			if len(arr) != 1 {
				return false
			}
			obj, _ := arr[0].(map[string]interface{})
			return obj != nil && obj["id"] == "10010"
		}, `fixVersions[0].id = "10010"`},
	}

	for _, tc := range cases {
		if !tc.check() {
			t.Errorf("field %q: want %s; fields=%v", tc.field, tc.desc, fields)
		}
	}
}

func TestIssueCreateCommand_FieldsFlag_MergedIntoBody(t *testing.T) {
	var body map[string]interface{}
	ts := captureCreateBody(t, &body)
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "create",
		"--project", "PROJ",
		"--summary", "With fields flag",
		"--fields", `{"customfield_31004":{"id":"50628"},"customfield_23824":{"id":"36274"}}`,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fieldObj(body, "customfield_31004") == nil {
		t.Errorf("customfield_31004 missing from fields: %v", body["fields"])
	}
	if fieldObj(body, "customfield_23824") == nil {
		t.Errorf("customfield_23824 missing from fields: %v", body["fields"])
	}
	// Typed summary must still be present alongside extra fields.
	if fieldStr(body, "summary") != "With fields flag" {
		t.Errorf("summary missing or wrong after --fields merge: %v", body["fields"])
	}
}

func TestIssueCreateCommand_FieldsFlag_OverridesConvenienceFlag(t *testing.T) {
	var body map[string]interface{}
	ts := captureCreateBody(t, &body)
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "create",
		"--project", "PROJ",
		"--summary", "original",
		"--fields", `{"summary":"overridden"}`,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fieldStr(body, "summary") != "overridden" {
		t.Errorf("--fields should override --summary; got: %v", body["fields"])
	}
}

func TestIssueCreateCommand_PropertiesFlag(t *testing.T) {
	var body map[string]interface{}
	ts := captureCreateBody(t, &body)
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "create",
		"--project", "PROJ",
		"--summary", "With properties",
		"--properties", `[{"key":"pipeline.id","value":"build-42"}]`,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props, _ := body["properties"].([]interface{})
	if len(props) != 1 {
		t.Fatalf("expected 1 property, got %d: %v", len(props), body)
	}
	prop, _ := props[0].(map[string]interface{})
	if prop["key"] != "pipeline.id" {
		t.Errorf("property key = %v, want pipeline.id", prop["key"])
	}
}

func TestIssueCreateCommand_HistoryFlag(t *testing.T) {
	var body map[string]interface{}
	ts := captureCreateBody(t, &body)
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "create",
		"--project", "PROJ",
		"--summary", "With history",
		"--history", `{"activityDescription":"Created by CI","actor":{"id":"ci-bot","type":"automation"}}`,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hm, _ := body["historyMetadata"].(map[string]interface{})
	if hm == nil {
		t.Fatalf("historyMetadata missing from body: %v", body)
	}
	if hm["activityDescription"] != "Created by CI" {
		t.Errorf("activityDescription = %v, want 'Created by CI'", hm["activityDescription"])
	}
	actor, _ := hm["actor"].(map[string]interface{})
	if actor == nil || actor["id"] != "ci-bot" {
		t.Errorf("actor.id = %v, want ci-bot", hm["actor"])
	}
}

func TestIssueCreateCommand_InvalidFieldsJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"1","key":"PROJ-1"}`))
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "create",
		"--project", "PROJ",
		"--summary", "test",
		"--fields", `{not valid json`,
	)
	if err == nil {
		t.Fatal("expected error for invalid --fields JSON")
	}
	if !strings.Contains(err.Error(), "--fields") {
		t.Errorf("error should mention --fields: %v", err)
	}
}

func TestIssueCreateCommand_InvalidPropertiesJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"1","key":"PROJ-1"}`))
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "create",
		"--project", "PROJ",
		"--summary", "test",
		"--properties", `not-an-array`,
	)
	if err == nil {
		t.Fatal("expected error for invalid --properties JSON")
	}
	if !strings.Contains(err.Error(), "--properties") {
		t.Errorf("error should mention --properties: %v", err)
	}
}

func TestIssueCreateCommand_InvalidHistoryJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"1","key":"PROJ-1"}`))
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "create",
		"--project", "PROJ",
		"--summary", "test",
		"--history", `{bad`,
	)
	if err == nil {
		t.Fatal("expected error for invalid --history JSON")
	}
	if !strings.Contains(err.Error(), "--history") {
		t.Errorf("error should mention --history: %v", err)
	}
}

// -----------------------------------------------------------------------
// issue update -- request body verification
// -----------------------------------------------------------------------

func TestIssueUpdateCommand_ConvenienceFlags_SentInBody(t *testing.T) {
	var body map[string]interface{}
	ts := captureUpdateBody(t, &body)
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "update", "PROJ-1",
		"--summary", "Updated title",
		"--description", "New desc",
		"--priority", "Low",
		"--assignee", "user-xyz",
		"--due-date", "2024-09-01",
		"--labels", "frontend",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fields, _ := body["fields"].(map[string]interface{})
	if fields == nil {
		t.Fatal("body missing 'fields'")
	}

	cases := []struct {
		field string
		check func() bool
		desc  string
	}{
		{"summary", func() bool { return fields["summary"] == "Updated title" }, `summary = "Updated title"`},
		{"description", func() bool { return fields["description"] == "New desc" }, `description = "New desc"`},
		{"duedate", func() bool { return fields["duedate"] == "2024-09-01" }, `duedate = "2024-09-01"`},
		{"priority", func() bool {
			obj, _ := fields["priority"].(map[string]interface{})
			return obj != nil && obj["name"] == "Low"
		}, `priority.name = "Low"`},
		{"assignee", func() bool {
			obj, _ := fields["assignee"].(map[string]interface{})
			return obj != nil && obj["accountId"] == "user-xyz"
		}, `assignee.accountId = "user-xyz"`},
		{"labels", func() bool {
			arr, _ := fields["labels"].([]interface{})
			return len(arr) == 1 && arr[0] == "frontend"
		}, `labels = ["frontend"]`},
	}

	for _, tc := range cases {
		if !tc.check() {
			t.Errorf("field %q: want %s; fields=%v", tc.field, tc.desc, fields)
		}
	}
}

func TestIssueUpdateCommand_FieldsFlag_MergedIntoBody(t *testing.T) {
	var body map[string]interface{}
	ts := captureUpdateBody(t, &body)
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "update", "PROJ-1",
		"--fields", `{"customfield_31004":{"id":"50628"}}`,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fieldObj(body, "customfield_31004") == nil {
		t.Errorf("customfield_31004 missing from fields: %v", body["fields"])
	}
}

func TestIssueUpdateCommand_FieldsFlag_OverridesConvenienceFlag(t *testing.T) {
	var body map[string]interface{}
	ts := captureUpdateBody(t, &body)
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "update", "PROJ-1",
		"--summary", "original",
		"--fields", `{"summary":"overridden"}`,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fieldStr(body, "summary") != "overridden" {
		t.Errorf("--fields should override --summary; got: %v", body["fields"])
	}
}

func TestIssueUpdateCommand_PropertiesFlag(t *testing.T) {
	var body map[string]interface{}
	ts := captureUpdateBody(t, &body)
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "update", "PROJ-1",
		"--properties", `[{"key":"env","value":"staging"}]`,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props, _ := body["properties"].([]interface{})
	if len(props) != 1 {
		t.Fatalf("expected 1 property, got %d: %v", len(props), body)
	}
	prop, _ := props[0].(map[string]interface{})
	if prop["key"] != "env" {
		t.Errorf("property key = %v, want env", prop["key"])
	}
}

func TestIssueUpdateCommand_HistoryFlag(t *testing.T) {
	var body map[string]interface{}
	ts := captureUpdateBody(t, &body)
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "update", "PROJ-1",
		"--summary", "Auto-resolved",
		"--history", `{"activityDescription":"Resolved by CI","actor":{"id":"ci-bot","type":"automation"}}`,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hm, _ := body["historyMetadata"].(map[string]interface{})
	if hm == nil {
		t.Fatalf("historyMetadata missing from body: %v", body)
	}
	if hm["activityDescription"] != "Resolved by CI" {
		t.Errorf("activityDescription = %v, want 'Resolved by CI'", hm["activityDescription"])
	}
}

func TestIssueUpdateCommand_NoFlags_StillRequiresAtLeastOne(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "update", "PROJ-1")
	if err == nil {
		t.Fatal("expected error when no update flags are provided")
	}
}

func TestIssueUpdateCommand_OnlyPropertiesFlag_IsValid(t *testing.T) {
	var body map[string]interface{}
	ts := captureUpdateBody(t, &body)
	defer ts.Close()

	// --properties alone (no convenience flags, no --fields) should be accepted.
	_, err := runCmd(t, ts.URL, "issue", "update", "PROJ-1",
		"--properties", `[{"key":"k","value":"v"}]`,
	)
	if err != nil {
		t.Errorf("expected no error with only --properties; got: %v", err)
	}
}

func TestIssueUpdateCommand_OnlyHistoryFlag_IsValid(t *testing.T) {
	var body map[string]interface{}
	ts := captureUpdateBody(t, &body)
	defer ts.Close()

	// --history alone should also be accepted.
	_, err := runCmd(t, ts.URL, "issue", "update", "PROJ-1",
		"--history", `{"type":"madeAutomatically"}`,
	)
	if err != nil {
		t.Errorf("expected no error with only --history; got: %v", err)
	}
}

func TestIssueUpdateCommand_InvalidFieldsJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "update", "PROJ-1",
		"--fields", `{invalid`,
	)
	if err == nil {
		t.Fatal("expected error for invalid --fields JSON")
	}
	if !strings.Contains(err.Error(), "--fields") {
		t.Errorf("error should mention --fields: %v", err)
	}
}

func TestIssueUpdateCommand_InvalidPropertiesJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "update", "PROJ-1",
		"--properties", `notjson`,
	)
	if err == nil {
		t.Fatal("expected error for invalid --properties JSON")
	}
	if !strings.Contains(err.Error(), "--properties") {
		t.Errorf("error should mention --properties: %v", err)
	}
}

func TestIssueUpdateCommand_InvalidHistoryJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "update", "PROJ-1",
		"--history", `{bad`,
	)
	if err == nil {
		t.Fatal("expected error for invalid --history JSON")
	}
	if !strings.Contains(err.Error(), "--history") {
		t.Errorf("error should mention --history: %v", err)
	}
}

// -----------------------------------------------------------------------
// issue delete
// -----------------------------------------------------------------------

func TestIssueDelete_Success(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "delete", "PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE method, got: %s", gotMethod)
	}
	if !strings.Contains(gotPath, "PROJ-1") {
		t.Errorf("expected PROJ-1 in request path, got: %s", gotPath)
	}
}

func TestIssueDelete_WithSubtasks(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "delete", "PROJ-2", "--delete-subtasks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotURL, "deleteSubtasks=true") {
		t.Errorf("expected deleteSubtasks=true in URL, got: %s", gotURL)
	}
}

func TestIssueDelete_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["Issue Does Not Exist"],"errors":{}}`))
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "delete", "GONE-1")
	if err == nil {
		t.Error("expected error for 404 response")
	}
}

// -----------------------------------------------------------------------
// issue comment
// -----------------------------------------------------------------------

func TestCommentList_TableOutput(t *testing.T) {
	body := map[string]interface{}{
		"comments": []map[string]interface{}{
			{
				"id":      "10001",
				"created": "2024-01-01T10:00:00.000Z",
				"body":    "First comment",
				"author":  map[string]string{"displayName": "Alice"},
			},
		},
		"total": 1,
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		data, _ := json.Marshal(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "comment", "list", "PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "First comment") {
		t.Errorf("expected comment body in output, got: %s", out)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("expected author in output, got: %s", out)
	}
}

func TestCommentAdd_Success(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := map[string]interface{}{"id": "10002", "body": "Test comment"}
		data, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "comment", "add", "PROJ-1", "--body", "Test comment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["body"] != "Test comment" {
		t.Errorf("expected body in request, got: %v", gotBody["body"])
	}
}

func TestCommentUpdate_Success(t *testing.T) {
	var gotBody map[string]interface{}
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := map[string]interface{}{"id": "10001", "body": "Updated comment"}
		data, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "comment", "update", "PROJ-1",
		"--comment-id", "10001", "--body", "Updated comment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "10001") {
		t.Errorf("expected comment ID in path, got: %s", gotPath)
	}
}

func TestCommentDelete_Success(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "comment", "delete", "PROJ-1", "--comment-id", "10001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE method, got: %s", gotMethod)
	}
	if !strings.Contains(gotPath, "10001") {
		t.Errorf("expected comment ID in path, got: %s", gotPath)
	}
}

// -----------------------------------------------------------------------
// issue transition
// -----------------------------------------------------------------------

func TestTransitionList_TableOutput(t *testing.T) {
	body := map[string]interface{}{
		"transitions": []map[string]interface{}{
			{"id": "11", "name": "Start Progress", "to": map[string]string{"name": "In Progress"}},
			{"id": "31", "name": "Done", "to": map[string]string{"name": "Closed"}},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		data, _ := json.Marshal(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "transition", "list", "PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Start Progress") {
		t.Errorf("expected transition name in output, got: %s", out)
	}
	if !strings.Contains(out, "In Progress") {
		t.Errorf("expected target status in output, got: %s", out)
	}
}

func TestTransitionApply_Success(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "transition", "apply", "PROJ-1", "--id", "11")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tid, _ := gotBody["transition"].(map[string]interface{})
	if tid["id"] != "11" {
		t.Errorf("expected transition id=11 in body, got: %v", gotBody["transition"])
	}
}

func TestTransitionApply_WithResolution(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "transition", "apply", "PROJ-1",
		"--id", "31", "--resolution", "Fixed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fields, _ := gotBody["fields"].(map[string]interface{})
	res, _ := fields["resolution"].(map[string]interface{})
	if res["name"] != "Fixed" {
		t.Errorf("expected resolution=Fixed in body fields, got: %v", fields)
	}
}

// -----------------------------------------------------------------------
// issue assign
// -----------------------------------------------------------------------

func TestIssueAssign_Success(t *testing.T) {
	var gotBody map[string]interface{}
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "assign", "PROJ-1", "--account-id", "acc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "PROJ-1") {
		t.Errorf("expected issue key in path, got: %s", gotPath)
	}
	if gotBody["accountId"] != "acc123" {
		t.Errorf("expected accountId in body, got: %v", gotBody)
	}
}

func TestIssueAssign_Unassign(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "assign", "PROJ-1", "--account-id", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// empty account ID should send null
	if _, hasKey := gotBody["accountId"]; hasKey && gotBody["accountId"] != nil {
		t.Errorf("expected null accountId for unassign, got: %v", gotBody["accountId"])
	}
}

// -----------------------------------------------------------------------
// issue worklog
// -----------------------------------------------------------------------

func TestWorklogList_TableOutput(t *testing.T) {
	body := map[string]interface{}{
		"worklogs": []map[string]interface{}{
			{
				"id":        "10001",
				"started":   "2024-01-15T09:00:00.000Z",
				"timeSpent": "2h",
				"comment":   "Did some work",
				"author":    map[string]string{"displayName": "Alice"},
			},
		},
		"total": 1,
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		data, _ := json.Marshal(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "worklog", "list", "PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "2h") {
		t.Errorf("expected time spent in output, got: %s", out)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("expected author in output, got: %s", out)
	}
}

func TestWorklogAdd_Success(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := map[string]interface{}{"id": "10002", "timeSpent": "2h"}
		data, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "worklog", "add", "PROJ-1",
		"--time-spent", "2h",
		"--started", "2024-01-15T09:00:00.000+0000",
		"--comment", "Fixed the bug",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["timeSpent"] != "2h" {
		t.Errorf("expected timeSpent=2h in body, got: %v", gotBody["timeSpent"])
	}
}

func TestWorklogDelete_Success(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "worklog", "delete", "PROJ-1", "--worklog-id", "10001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE method, got: %s", gotMethod)
	}
	if !strings.Contains(gotPath, "10001") {
		t.Errorf("expected worklog ID in path, got: %s", gotPath)
	}
}

// -----------------------------------------------------------------------
// issue vote
// -----------------------------------------------------------------------

func TestVoteGet_TableOutput(t *testing.T) {
	body := map[string]interface{}{"votes": 3, "hasVoted": true}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		data, _ := json.Marshal(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "vote", "get", "PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "3") {
		t.Errorf("expected vote count in output, got: %s", out)
	}
}

func TestVoteAdd_Success(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "vote", "add", "PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST method, got: %s", gotMethod)
	}
	if !strings.Contains(gotPath, "votes") {
		t.Errorf("expected votes endpoint in path, got: %s", gotPath)
	}
}

func TestVoteRemove_Success(t *testing.T) {
	var gotMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "vote", "remove", "PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE method, got: %s", gotMethod)
	}
}

// -----------------------------------------------------------------------
// issue watch
// -----------------------------------------------------------------------

func TestWatchList_TableOutput(t *testing.T) {
	body := map[string]interface{}{
		"watchCount": 2,
		"watchers": []map[string]interface{}{
			{"accountId": "acc1", "displayName": "Alice", "emailAddress": "alice@example.com"},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		data, _ := json.Marshal(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "watch", "list", "PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("expected watcher name in output, got: %s", out)
	}
	if !strings.Contains(out, "Watch count: 2") {
		t.Errorf("expected watch count in output, got: %s", out)
	}
}

func TestWatchAdd_Success(t *testing.T) {
	var gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "watch", "add", "PROJ-1", "--account-id", "acc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "acc123") {
		t.Errorf("expected account ID in request body, got: %s", gotBody)
	}
}

func TestWatchRemove_Success(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "watch", "remove", "PROJ-1", "--account-id", "acc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotURL, "acc123") {
		t.Errorf("expected account ID in request URL, got: %s", gotURL)
	}
}

// -----------------------------------------------------------------------
// issue link
// -----------------------------------------------------------------------

func TestLinkTypes_TableOutput(t *testing.T) {
	body := map[string]interface{}{
		"issueLinkTypes": []map[string]interface{}{
			{"id": "10000", "name": "Blocks", "inward": "is blocked by", "outward": "blocks"},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		data, _ := json.Marshal(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "issue", "link", "types")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Blocks") {
		t.Errorf("expected link type in output, got: %s", out)
	}
}

func TestLinkCreate_Success(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "link", "create",
		"--type", "blocks",
		"--inward", "PROJ-42",
		"--outward", "PROJ-50",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lt, _ := gotBody["type"].(map[string]interface{})
	if lt["name"] != "blocks" {
		t.Errorf("expected link type name in body, got: %v", gotBody["type"])
	}
}

func TestLinkDelete_Success(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "link", "delete", "--link-id", "10000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE method, got: %s", gotMethod)
	}
	if !strings.Contains(gotPath, "10000") {
		t.Errorf("expected link ID in path, got: %s", gotPath)
	}
}

// -----------------------------------------------------------------------
// issue attach
// -----------------------------------------------------------------------

func TestAttachAdd_Success(t *testing.T) {
	var gotHeader string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Atlassian-Token")
		resp := []map[string]interface{}{{"id": "att1", "filename": "test.txt"}}
		data, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	// Create a temporary file to attach
	f, err := os.CreateTemp("", "attach_test_*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	_, _ = f.WriteString("test content")
	f.Close()

	_, err = runCmd(t, ts.URL, "issue", "attach", "add", "PROJ-1", "--file", f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHeader != "no-check" {
		t.Errorf("expected X-Atlassian-Token: no-check header, got: %s", gotHeader)
	}
}

func TestAttachDelete_Success(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "issue", "attach", "delete", "--attachment-id", "att1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE method, got: %s", gotMethod)
	}
	if !strings.Contains(gotPath, "att1") {
		t.Errorf("expected attachment ID in path, got: %s", gotPath)
	}
}
