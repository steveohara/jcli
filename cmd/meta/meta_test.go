// Package meta contains tests for all meta sub-commands.
//
// Each test spins up an httptest.Server that returns a minimal JSON fixture,
// calls runCmd with the appropriate arguments, and verifies the output or error.
// Global flags are reset before every test to prevent bleed between runs.
package meta

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
)

// TestMain wires MetaCmd into the root command once for all tests in this
// package, mirroring what main() does in the binary.
func TestMain(m *testing.M) {
	cmd.RootCmd.AddCommand(MetaCmd)
	os.Exit(m.Run())
}

// resetCmdFlags resets global flags in the cmd package so that a JSON test
// does not affect the next table test. Meta commands use only inline flag
// reads rather than package-level variables, so only the global reset is needed.
func resetCmdFlags() {
	cmd.ResetGlobalFlags()
}

// runCmd executes a jcli command against serverURL, captures stdout and returns
// the output text and any execution error.
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

// serveJSON starts a test HTTP server that always responds with the given
// status code and JSON-encoded body.
func serveJSON(t *testing.T, status int, body interface{}) *httptest.Server {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(data)
	}))
}

// -----------------------------------------------------------------------
// meta issue-types
// -----------------------------------------------------------------------

func TestIssueTypes_TableOutput(t *testing.T) {
	body := []interface{}{
		map[string]interface{}{"id": "1", "name": "Bug", "subtask": false, "description": "A software bug"},
		map[string]interface{}{"id": "2", "name": "Story", "subtask": false, "description": "A user story"},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "meta", "issue-types")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Bug") {
		t.Errorf("expected Bug in output, got: %s", out)
	}
	if !strings.Contains(out, "Story") {
		t.Errorf("expected Story in output, got: %s", out)
	}
}

func TestIssueTypes_JSONOutput(t *testing.T) {
	body := []interface{}{map[string]interface{}{"id": "3", "name": "Task", "subtask": false}}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "meta", "issue-types", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Task") {
		t.Errorf("expected Task in JSON output, got: %s", out)
	}
}

// -----------------------------------------------------------------------
// meta priorities
// -----------------------------------------------------------------------

func TestPriorities_TableOutput(t *testing.T) {
	body := []interface{}{
		map[string]interface{}{"id": "1", "name": "Highest", "description": "Highest priority"},
		map[string]interface{}{"id": "5", "name": "Lowest", "description": ""},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "meta", "priorities")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Highest") {
		t.Errorf("expected Highest in output, got: %s", out)
	}
	if !strings.Contains(out, "Lowest") {
		t.Errorf("expected Lowest in output, got: %s", out)
	}
}

// -----------------------------------------------------------------------
// meta statuses
// -----------------------------------------------------------------------

func TestStatuses_TableOutput(t *testing.T) {
	body := []interface{}{
		map[string]interface{}{
			"id":          "1",
			"name":        "Open",
			"description": "Issue is open",
			"statusCategory": map[string]interface{}{
				"id":   1,
				"key":  "new",
				"name": "To Do",
			},
		},
		map[string]interface{}{
			"id":          "10001",
			"name":        "Done",
			"description": "",
			"statusCategory": map[string]interface{}{
				"id":   3,
				"key":  "done",
				"name": "Done",
			},
		},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "meta", "statuses")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Open") {
		t.Errorf("expected Open status in output, got: %s", out)
	}
	if !strings.Contains(out, "Done") {
		t.Errorf("expected Done status in output, got: %s", out)
	}
}

// -----------------------------------------------------------------------
// meta fields
// -----------------------------------------------------------------------

func TestFields_TableOutput(t *testing.T) {
	body := []interface{}{
		map[string]interface{}{"id": "summary", "name": "Summary", "custom": false, "searchable": true},
		map[string]interface{}{"id": "customfield_10014", "name": "Sprint", "custom": true, "searchable": true},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "meta", "fields")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Summary") {
		t.Errorf("expected Summary in output, got: %s", out)
	}
	if !strings.Contains(out, "Sprint") {
		t.Errorf("expected Sprint in output, got: %s", out)
	}
}

// -----------------------------------------------------------------------
// meta resolutions
// -----------------------------------------------------------------------

func TestResolutions_TableOutput(t *testing.T) {
	body := []interface{}{
		map[string]interface{}{"id": "10000", "name": "Fixed", "description": "The issue has been fixed"},
		map[string]interface{}{"id": "10001", "name": "Won't Fix", "description": ""},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "meta", "resolutions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Fixed") {
		t.Errorf("expected Fixed in output, got: %s", out)
	}
}

// -----------------------------------------------------------------------
// meta server-info
// -----------------------------------------------------------------------

func TestServerInfo_TableOutput(t *testing.T) {
	body := map[string]interface{}{
		"serverTitle":    "My Jira",
		"baseURL":        "https://jira.example.com",
		"version":        "9.4.0",
		"deploymentType": "Server",
		"buildNumber":    900040,
		"buildDate":      "2024-01-01T00:00:00.000Z",
		"serverTime":     "2024-06-01T10:00:00.000Z",
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "meta", "server-info")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "My Jira") {
		t.Errorf("expected server title in output, got: %s", out)
	}
	if !strings.Contains(out, "9.4.0") {
		t.Errorf("expected version in output, got: %s", out)
	}
}

func TestServerInfo_JSONOutput(t *testing.T) {
	body := map[string]interface{}{
		"serverTitle": "My Jira",
		"version":     "9.4.0",
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "meta", "server-info", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "My Jira") {
		t.Errorf("expected server title in JSON output, got: %s", out)
	}
}

// -----------------------------------------------------------------------
// meta project-statuses
// -----------------------------------------------------------------------

func TestProjectStatuses_TableOutput(t *testing.T) {
	body := []interface{}{
		map[string]interface{}{
			"id":   "10002",
			"name": "Bug",
			"statuses": []map[string]interface{}{
				{"id": "1", "name": "Open", "statusCategory": map[string]string{"name": "To Do"}},
				{"id": "10001", "name": "Done", "statusCategory": map[string]string{"name": "Done"}},
			},
		},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "meta", "project-statuses", "PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Bug") {
		t.Errorf("expected issue type in output, got: %s", out)
	}
	if !strings.Contains(out, "Open") {
		t.Errorf("expected Open status in output, got: %s", out)
	}
}

func TestProjectStatuses_IssueTypeFilter(t *testing.T) {
	body := []interface{}{
		map[string]interface{}{
			"id":   "10001",
			"name": "Story",
			"statuses": []map[string]interface{}{
				{"id": "1", "name": "Open", "statusCategory": map[string]string{"name": "To Do"}},
			},
		},
		map[string]interface{}{
			"id":   "10002",
			"name": "Bug",
			"statuses": []map[string]interface{}{
				{"id": "1", "name": "Open", "statusCategory": map[string]string{"name": "To Do"}},
			},
		},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "meta", "project-statuses", "PROJ", "--issue-type", "Story")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Story") {
		t.Errorf("expected Story in output, got: %s", out)
	}
	if strings.Contains(out, "Bug") {
		t.Errorf("expected Bug to be filtered out, got: %s", out)
	}
}

func TestProjectStatuses_UnknownIssueTypeFilter(t *testing.T) {
	body := []interface{}{
		map[string]interface{}{
			"id":       "10001",
			"name":     "Story",
			"statuses": []interface{}{},
		},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "meta", "project-statuses", "PROJ", "--issue-type", "NonExistent")
	if err == nil {
		t.Error("expected error for unknown issue type filter")
	}
}

// -----------------------------------------------------------------------
// meta link-types
// -----------------------------------------------------------------------

func TestLinkTypes_TableOutput(t *testing.T) {
	body := map[string]interface{}{
		"issueLinkTypes": []map[string]interface{}{
			{"id": "10000", "name": "Blocks", "inward": "is blocked by", "outward": "blocks"},
			{"id": "10001", "name": "Relates", "inward": "relates to", "outward": "relates to"},
		},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "meta", "link-types")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Blocks") {
		t.Errorf("expected Blocks in output, got: %s", out)
	}
	if !strings.Contains(out, "is blocked by") {
		t.Errorf("expected inward label in output, got: %s", out)
	}
}

// -----------------------------------------------------------------------
// meta configuration
// -----------------------------------------------------------------------

func TestConfiguration_TableOutput(t *testing.T) {
	body := map[string]interface{}{
		"votingEnabled":           true,
		"watchingEnabled":         true,
		"unassignedIssuesAllowed": false,
		"subTasksEnabled":         true,
		"issueLinkingEnabled":     true,
		"timeTrackingEnabled":     true,
		"attachmentsEnabled":      true,
		"timeTrackingConfiguration": map[string]interface{}{
			"workingHoursPerDay": 8.0,
			"workingDaysPerWeek": 5.0,
			"timeFormat":         "pretty",
			"defaultUnit":        "minute",
		},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "meta", "configuration")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "votingEnabled") {
		t.Errorf("expected votingEnabled in output, got: %s", out)
	}
	if !strings.Contains(out, "8.0") {
		t.Errorf("expected working hours in output, got: %s", out)
	}
}

// -----------------------------------------------------------------------
// meta field-allowed-values
// -----------------------------------------------------------------------

func TestFieldAllowedValues_TableOutput(t *testing.T) {
	// The command fetches /rest/api/2/issue/{key}/editmeta
	body := map[string]interface{}{
		"fields": map[string]interface{}{
			"priority": map[string]interface{}{
				"name": "Priority",
				"allowedValues": []interface{}{
					map[string]interface{}{"id": "1", "name": "Highest", "value": "", "description": ""},
					map[string]interface{}{"id": "5", "name": "Lowest", "value": "", "description": ""},
				},
			},
		},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "meta", "field-allowed-values", "priority", "--issue", "PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Highest") {
		t.Errorf("expected Highest in output, got: %s", out)
	}
	if !strings.Contains(out, "Lowest") {
		t.Errorf("expected Lowest in output, got: %s", out)
	}
}

func TestFieldAllowedValues_FieldNotFound(t *testing.T) {
	body := map[string]interface{}{
		"fields": map[string]interface{}{},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "meta", "field-allowed-values", "customfield_99999", "--issue", "PROJ-1")
	if err == nil {
		t.Error("expected error when field is not found in edit metadata")
	}
}
