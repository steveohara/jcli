// Package project contains tests for all project sub-commands.
//
// Each test spins up an httptest.Server that returns a minimal JSON fixture,
// calls runCmd with the appropriate arguments, and verifies the output or error.
// Flag variables are reset before every test to prevent bleed between runs.
package project

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

// TestMain wires ProjectCmd into the root command once for all tests in this
// package, mirroring what main() does in the binary.
func TestMain(m *testing.M) {
	cmd.RootCmd.AddCommand(ProjectCmd)
	os.Exit(m.Run())
}

// resetCmdFlags zeroes all package-level flag variables so successive tests
// do not bleed into each other. It also resets the shared global flags in the
// cmd package so that a JSON test does not affect the next table test.
func resetCmdFlags() {
	cmd.ResetGlobalFlags()
	createKey = ""
	createName = ""
	createDescription = ""
	createType = "software"
	createLead = ""
	createAssignee = "UNASSIGNED"
	updateName = ""
	updateDescription = ""
	updateLead = ""
	versionName = ""
	versionDescription = ""
	versionReleaseDate = ""
	versionIDFlag = ""
	versionReleased = false
	versionArchived = false
	componentName = ""
	componentDescription = ""
	componentIDFlag = ""
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

// projectPayload returns a minimal project JSON object.
func projectPayload(key, name string) map[string]interface{} {
	return map[string]interface{}{
		"id":          "10001",
		"key":         key,
		"name":        name,
		"projectType": "software",
		"lead":        map[string]string{"displayName": "Alice"},
		"description": "A project",
	}
}

// -----------------------------------------------------------------------
// project list
// -----------------------------------------------------------------------

func TestProjectList_TableOutput(t *testing.T) {
	body := []interface{}{projectPayload("PROJ", "My Project")}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "project", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "My Project") {
		t.Errorf("expected project name in output, got: %s", out)
	}
	if !strings.Contains(out, "PROJ") {
		t.Errorf("expected project key in output, got: %s", out)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("expected lead name in output, got: %s", out)
	}
}

func TestProjectList_JSONOutput(t *testing.T) {
	body := []interface{}{projectPayload("DEMO", "Demo")}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "project", "list", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "DEMO") {
		t.Errorf("expected project key in JSON output, got: %s", out)
	}
}

func TestProjectList_NoLead(t *testing.T) {
	body := []interface{}{
		map[string]interface{}{"id": "10002", "key": "NK", "name": "No Lead", "projectType": "software", "lead": nil},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "project", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No Lead") {
		t.Errorf("expected project name in output, got: %s", out)
	}
}

func TestProjectList_ServerError(t *testing.T) {
	ts := serveJSON(t, 500, map[string]string{"message": "internal error"})
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "project", "list")
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

// -----------------------------------------------------------------------
// project get
// -----------------------------------------------------------------------

func TestProjectGet_TableOutput(t *testing.T) {
	ts := serveJSON(t, 200, projectPayload("PROJ", "My Project"))
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "project", "get", "PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "My Project") {
		t.Errorf("expected project name in output, got: %s", out)
	}
	if !strings.Contains(out, "PROJ") {
		t.Errorf("expected project key in output, got: %s", out)
	}
}

func TestProjectGet_JSONOutput(t *testing.T) {
	ts := serveJSON(t, 200, projectPayload("PROJ", "My Project"))
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "project", "get", "PROJ", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"key"`) {
		t.Errorf("expected JSON output, got: %s", out)
	}
}

func TestProjectGet_NotFound(t *testing.T) {
	ts := serveJSON(t, 404, map[string]string{"message": "Project Does Not Exist"})
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "project", "get", "NOPE")
	if err == nil {
		t.Error("expected error for 404 response")
	}
}

// -----------------------------------------------------------------------
// project create
// -----------------------------------------------------------------------

func TestProjectCreate_Success(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := projectPayload("NEWP", "New Project")
		data, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "project", "create", "--key", "NEWP", "--name", "New Project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["key"] != "NEWP" {
		t.Errorf("expected key=NEWP in request body, got: %v", gotBody["key"])
	}
	if gotBody["name"] != "New Project" {
		t.Errorf("expected name in request body, got: %v", gotBody["name"])
	}
}

func TestProjectCreate_WithAllFlags(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := projectPayload("BPROJ", "Biz Project")
		data, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "project", "create",
		"--key", "BPROJ",
		"--name", "Biz Project",
		"--description", "A business project",
		"--type", "business",
		"--lead", "account123",
		"--assignee-type", "PROJECT_LEAD",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["projectTypeKey"] != "business" {
		t.Errorf("expected projectTypeKey=business, got: %v", gotBody["projectTypeKey"])
	}
	if gotBody["leadAccountId"] != "account123" {
		t.Errorf("expected leadAccountId in body, got: %v", gotBody["leadAccountId"])
	}
}

// -----------------------------------------------------------------------
// project update
// -----------------------------------------------------------------------

func TestProjectUpdate_Success(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := projectPayload("PROJ", "Updated Name")
		data, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "project", "update", "PROJ", "--name", "Updated Name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["name"] != "Updated Name" {
		t.Errorf("expected name in body, got: %v", gotBody["name"])
	}
}

func TestProjectUpdate_NoFlags_ReturnsError(t *testing.T) {
	ts := serveJSON(t, 200, projectPayload("PROJ", ""))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "project", "update", "PROJ")
	if err == nil {
		t.Error("expected error when no update flags provided")
	}
}

// -----------------------------------------------------------------------
// project delete
// -----------------------------------------------------------------------

func TestProjectDelete_Success(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(204)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "project", "delete", "PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE method, got: %s", gotMethod)
	}
	if !strings.Contains(gotPath, "PROJ") {
		t.Errorf("expected PROJ in request path, got: %s", gotPath)
	}
}

func TestProjectDelete_NotFound(t *testing.T) {
	ts := serveJSON(t, 404, map[string]string{"message": "Not Found"})
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "project", "delete", "GONE")
	if err == nil {
		t.Error("expected error for 404 response")
	}
}

// -----------------------------------------------------------------------
// project version list
// -----------------------------------------------------------------------

func TestVersionList_TableOutput(t *testing.T) {
	body := []interface{}{
		map[string]interface{}{"id": "10010", "name": "v1.0", "released": true, "archived": false, "releaseDate": "2024-03-01"},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "project", "version", "list", "PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "v1.0") {
		t.Errorf("expected version name in output, got: %s", out)
	}
	if !strings.Contains(out, "2024-03-01") {
		t.Errorf("expected release date in output, got: %s", out)
	}
}

// -----------------------------------------------------------------------
// project version create
// -----------------------------------------------------------------------

func TestVersionCreate_Success(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := map[string]interface{}{"id": "10011", "name": "v2.0", "released": false}
		data, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "project", "version", "create", "PROJ", "--name", "v2.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["name"] != "v2.0" {
		t.Errorf("expected name=v2.0 in body, got: %v", gotBody["name"])
	}
	if gotBody["project"] != "PROJ" {
		t.Errorf("expected project=PROJ in body, got: %v", gotBody["project"])
	}
}

// -----------------------------------------------------------------------
// project version update
// -----------------------------------------------------------------------

func TestVersionUpdate_Success(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := map[string]interface{}{"id": "10010", "name": "v1.0.1"}
		data, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "project", "version", "update", "--id", "10010", "--name", "v1.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["name"] != "v1.0.1" {
		t.Errorf("expected updated name in body, got: %v", gotBody["name"])
	}
}

// -----------------------------------------------------------------------
// project version delete
// -----------------------------------------------------------------------

func TestVersionDelete_Success(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(204)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "project", "version", "delete", "--id", "10010")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE method, got: %s", gotMethod)
	}
	if !strings.Contains(gotPath, "10010") {
		t.Errorf("expected version ID in path, got: %s", gotPath)
	}
}

// -----------------------------------------------------------------------
// project component list
// -----------------------------------------------------------------------

func TestComponentList_TableOutput(t *testing.T) {
	body := []interface{}{
		map[string]interface{}{
			"id":          "10020",
			"name":        "Backend",
			"description": "Backend services",
			"lead":        map[string]string{"displayName": "Bob"},
		},
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "project", "component", "list", "PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Backend") {
		t.Errorf("expected component name in output, got: %s", out)
	}
	if !strings.Contains(out, "Bob") {
		t.Errorf("expected lead in output, got: %s", out)
	}
}

// -----------------------------------------------------------------------
// project component create
// -----------------------------------------------------------------------

func TestComponentCreate_Success(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := map[string]interface{}{"id": "10021", "name": "Frontend"}
		data, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "project", "component", "create", "PROJ", "--name", "Frontend")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["name"] != "Frontend" {
		t.Errorf("expected name=Frontend in body, got: %v", gotBody["name"])
	}
	if gotBody["project"] != "PROJ" {
		t.Errorf("expected project=PROJ in body, got: %v", gotBody["project"])
	}
}

// -----------------------------------------------------------------------
// project component update
// -----------------------------------------------------------------------

func TestComponentUpdate_Success(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := map[string]interface{}{"id": "10020", "name": "UI Layer"}
		data, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "project", "component", "update", "--id", "10020", "--name", "UI Layer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["name"] != "UI Layer" {
		t.Errorf("expected updated name in body, got: %v", gotBody["name"])
	}
}

// -----------------------------------------------------------------------
// project component delete
// -----------------------------------------------------------------------

func TestComponentDelete_Success(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(204)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "project", "component", "delete", "--id", "10020")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE method, got: %s", gotMethod)
	}
	if !strings.Contains(gotPath, "10020") {
		t.Errorf("expected component ID in path, got: %s", gotPath)
	}
}
