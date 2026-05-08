// Package board contains tests for all board and sprint sub-commands.
//
// Each test spins up an httptest.Server that returns a minimal JSON fixture,
// then calls runCmd with the appropriate arguments and verifies the output or
// error. Flag variables are reset via resetCmdFlags before every test so that
// successive runs do not bleed into each other.
package board

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

// TestMain wires BoardCmd into the root command once for all tests in this
// package, mirroring what main() does in the binary.
func TestMain(m *testing.M) {
	cmd.RootCmd.AddCommand(BoardCmd)
	os.Exit(m.Run())
}

// resetCmdFlags zeroes all package-level flag variables so successive tests
// do not bleed into each other.  It also resets the shared global flags in
// the cmd package (e.g. --output) so that a JSON test does not affect the
// next table test.
func resetCmdFlags() {
	cmd.ResetGlobalFlags()
	listProject = ""
	listMaxResults = 50
	sprintBoardID = 0
	sprintState = ""
	sprintCreateName = ""
	sprintCreateGoal = ""
	sprintCreateStartDate = ""
	sprintCreateEndDate = ""
	sprintCreateBoardID = 0
	sprintUpdateID = 0
	sprintUpdateName = ""
	sprintUpdateGoal = ""
	sprintUpdateState = ""
	sprintUpdateStart = ""
	sprintUpdateEnd = ""
	sprintIssuesID = 0
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

// serveJSON starts a test HTTP server that responds to all requests with the
// given status code and JSON-encoded body.
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
// board list
// -----------------------------------------------------------------------

func TestBoardList_TableOutput(t *testing.T) {
	body := map[string]interface{}{
		"values": []map[string]interface{}{
			{"id": 1, "name": "My Board", "type": "scrum", "location": map[string]string{"projectKey": "PROJ"}},
		},
		"total": 1,
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "board", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "My Board") {
		t.Errorf("expected board name in output, got: %s", out)
	}
	if !strings.Contains(out, "PROJ") {
		t.Errorf("expected project key in output, got: %s", out)
	}
}

func TestBoardList_JSONOutput(t *testing.T) {
	body := map[string]interface{}{
		"values": []map[string]interface{}{
			{"id": 2, "name": "Board2", "type": "kanban"},
		},
		"total": 1,
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "board", "list", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Board2") {
		t.Errorf("expected board name in JSON output, got: %s", out)
	}
}

func TestBoardList_WithProjectFilter(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		body := map[string]interface{}{"values": []interface{}{}, "total": 0}
		data, _ := json.Marshal(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "board", "list", "--project", "MYPROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotURL, "MYPROJ") {
		t.Errorf("expected project key in request URL, got: %s", gotURL)
	}
}

func TestBoardList_ServerError(t *testing.T) {
	ts := serveJSON(t, 500, map[string]string{"message": "internal error"})
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "board", "list")
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
}

// -----------------------------------------------------------------------
// board sprint list
// -----------------------------------------------------------------------

func TestSprintList_TableOutput(t *testing.T) {
	body := map[string]interface{}{
		"values": []map[string]interface{}{
			{"id": 10, "name": "Sprint 1", "state": "active", "startDate": "2024-01-01", "endDate": "2024-01-14", "goal": "Ship feature"},
		},
		"total": 1,
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "board", "sprint", "list", "--board-id", "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Sprint 1") {
		t.Errorf("expected sprint name in output, got: %s", out)
	}
	if !strings.Contains(out, "active") {
		t.Errorf("expected sprint state in output, got: %s", out)
	}
}

func TestSprintList_WithStateFilter(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		body := map[string]interface{}{"values": []interface{}{}, "total": 0}
		data, _ := json.Marshal(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "board", "sprint", "list", "--board-id", "5", "--state", "future")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotURL, "future") {
		t.Errorf("expected state filter in URL, got: %s", gotURL)
	}
}

// -----------------------------------------------------------------------
// board sprint create
// -----------------------------------------------------------------------

func TestSprintCreate_Success(t *testing.T) {
	body := map[string]interface{}{"id": 20, "name": "Sprint 5", "state": "future"}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "board", "sprint", "create", "--board-id", "1", "--name", "Sprint 5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSprintCreate_WithAllFlags(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := map[string]interface{}{"id": 21, "name": "Sprint 6", "state": "future"}
		data, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "board", "sprint", "create",
		"--board-id", "2",
		"--name", "Sprint 6",
		"--goal", "Complete login",
		"--start", "2024-02-01T00:00:00.000Z",
		"--end", "2024-02-14T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["name"] != "Sprint 6" {
		t.Errorf("expected name=Sprint 6, got: %v", gotBody["name"])
	}
	if gotBody["goal"] != "Complete login" {
		t.Errorf("expected goal in body, got: %v", gotBody["goal"])
	}
}

// -----------------------------------------------------------------------
// board sprint update
// -----------------------------------------------------------------------

func TestSprintUpdate_Success(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := map[string]interface{}{"id": 10, "name": "Sprint 1 updated", "state": "active"}
		data, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "board", "sprint", "update", "--id", "10", "--name", "Sprint 1 updated")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["name"] != "Sprint 1 updated" {
		t.Errorf("expected updated name in body, got: %v", gotBody["name"])
	}
}

func TestSprintUpdate_StateTransition(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := map[string]interface{}{"id": 10, "name": "Sprint 1", "state": "closed"}
		data, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "board", "sprint", "update", "--id", "10", "--state", "closed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["state"] != "closed" {
		t.Errorf("expected state=closed in body, got: %v", gotBody["state"])
	}
}

// -----------------------------------------------------------------------
// board sprint issues
// -----------------------------------------------------------------------

func TestSprintIssues_TableOutput(t *testing.T) {
	body := map[string]interface{}{
		"total": 2,
		"issues": []map[string]interface{}{
			{
				"key": "PROJ-1",
				"fields": map[string]interface{}{
					"summary":   "First issue",
					"issuetype": map[string]string{"name": "Story"},
					"status":    map[string]string{"name": "In Progress"},
					"assignee":  map[string]string{"displayName": "Alice"},
				},
			},
			{
				"key": "PROJ-2",
				"fields": map[string]interface{}{
					"summary":   "Second issue",
					"issuetype": map[string]string{"name": "Bug"},
					"status":    map[string]string{"name": "Open"},
					"assignee":  nil,
				},
			},
		},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "board", "sprint", "issues", "--id", "5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "PROJ-1") {
		t.Errorf("expected PROJ-1 in output, got: %s", out)
	}
	if !strings.Contains(out, "PROJ-2") {
		t.Errorf("expected PROJ-2 in output, got: %s", out)
	}
	if !strings.Contains(out, "Sprint 5: 2 issues") {
		t.Errorf("expected summary line in output, got: %s", out)
	}
}

func TestSprintIssues_JSONOutput(t *testing.T) {
	body := map[string]interface{}{
		"total":  1,
		"issues": []map[string]interface{}{{"key": "PROJ-3", "fields": map[string]interface{}{}}},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "board", "sprint", "issues", "--id", "5", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "PROJ-3") {
		t.Errorf("expected PROJ-3 in JSON output, got: %s", out)
	}
}
