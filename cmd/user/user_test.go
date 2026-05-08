// Package user contains tests for all user sub-commands.
//
// Each test spins up an httptest.Server that returns a minimal JSON fixture,
// calls runCmd with the appropriate arguments, and verifies the output or error.
// Global flags are reset before every test to prevent bleed between runs.
package user

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

// TestMain wires UserCmd into the root command once for all tests in this
// package, mirroring what main() does in the binary.
func TestMain(m *testing.M) {
	cmd.RootCmd.AddCommand(UserCmd)
	os.Exit(m.Run())
}

// resetCmdFlags zeroes all package-level flag variables so successive tests
// do not bleed into each other. It also resets shared global flags in the cmd
// package so that a JSON test does not affect the next table test.
func resetCmdFlags() {
	cmd.ResetGlobalFlags()
	getAccountID = ""
	searchQuery = ""
	searchMaxResults = 50
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

// userPayload returns a minimal Jira user JSON object.
func userPayload(accountID, displayName, email string, active bool) map[string]interface{} {
	return map[string]interface{}{
		"accountId":    accountID,
		"displayName":  displayName,
		"emailAddress": email,
		"active":       active,
	}
}

// -----------------------------------------------------------------------
// user get
// -----------------------------------------------------------------------

func TestUserGet_TableOutput(t *testing.T) {
	body := userPayload("acc123", "Alice", "alice@example.com", true)
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "user", "get", "--account-id", "acc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("expected display name in output, got: %s", out)
	}
	if !strings.Contains(out, "alice@example.com") {
		t.Errorf("expected email in output, got: %s", out)
	}
	if !strings.Contains(out, "yes") {
		t.Errorf("expected active=yes in output, got: %s", out)
	}
}

func TestUserGet_JSONOutput(t *testing.T) {
	body := userPayload("acc123", "Alice", "alice@example.com", true)
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "user", "get", "--account-id", "acc123", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"accountId"`) {
		t.Errorf("expected JSON output, got: %s", out)
	}
}

func TestUserGet_InactiveUser(t *testing.T) {
	body := userPayload("acc456", "Bob", "bob@example.com", false)
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "user", "get", "--account-id", "acc456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "no") {
		t.Errorf("expected active=no in output, got: %s", out)
	}
}

func TestUserGet_NotFound(t *testing.T) {
	ts := serveJSON(t, 404, map[string]string{"message": "User Does Not Exist"})
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "user", "get", "--account-id", "nobody")
	if err == nil {
		t.Error("expected error for 404 response")
	}
}

// -----------------------------------------------------------------------
// user myself
// -----------------------------------------------------------------------

func TestUserMyself_TableOutput(t *testing.T) {
	body := userPayload("self123", "Me Myself", "me@example.com", true)
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "user", "myself")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Me Myself") {
		t.Errorf("expected display name in output, got: %s", out)
	}
	if !strings.Contains(out, "me@example.com") {
		t.Errorf("expected email in output, got: %s", out)
	}
}

func TestUserMyself_JSONOutput(t *testing.T) {
	body := userPayload("self123", "Me Myself", "me@example.com", true)
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "user", "myself", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Me Myself") {
		t.Errorf("expected display name in JSON output, got: %s", out)
	}
}

func TestUserMyself_Unauthorized(t *testing.T) {
	ts := serveJSON(t, 401, map[string]string{"message": "Unauthorized"})
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "user", "myself")
	if err == nil {
		t.Error("expected error for 401 response")
	}
}

// -----------------------------------------------------------------------
// user search
// -----------------------------------------------------------------------

func TestUserSearch_TableOutput(t *testing.T) {
	body := []interface{}{
		userPayload("acc1", "Alice Smith", "alice@example.com", true),
		userPayload("acc2", "Alice Jones", "alicej@example.com", false),
	}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "user", "search", "--query", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Alice Smith") {
		t.Errorf("expected first user in output, got: %s", out)
	}
	if !strings.Contains(out, "Alice Jones") {
		t.Errorf("expected second user in output, got: %s", out)
	}
}

func TestUserSearch_QueryInURL(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		data, _ := json.Marshal([]interface{}{})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	_, err := runCmd(t, ts.URL, "user", "search", "--query", "smith")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotURL, "smith") {
		t.Errorf("expected query in request URL, got: %s", gotURL)
	}
}

func TestUserSearch_JSONOutput(t *testing.T) {
	body := []interface{}{userPayload("acc1", "Alice", "alice@example.com", true)}
	ts := serveJSON(t, 200, body)
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "user", "search", "--query", "alice", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("expected user in JSON output, got: %s", out)
	}
}

func TestUserSearch_EmptyResults(t *testing.T) {
	ts := serveJSON(t, 200, []interface{}{})
	defer ts.Close()

	out, err := runCmd(t, ts.URL, "user", "search", "--query", "nobody")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// no rows, just headers
	if !strings.Contains(out, "ACCOUNT ID") {
		t.Errorf("expected table header in output, got: %s", out)
	}
}
