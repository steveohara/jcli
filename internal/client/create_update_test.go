// Package client_test contains tests for the CreateIssueRequest and
// UpdateIssueRequest types, verifying that ExtraFields, Properties, and
// HistoryMetadata are marshalled correctly and that ExtraFields values
// override typed convenience fields when both name the same key.
package client_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/steveohara/jcli/internal/client"
)

// -----------------------------------------------------------------------
// CreateIssueRequest.MarshalJSON
// -----------------------------------------------------------------------

// TestCreateIssueRequest_MarshalJSON_TypedFieldsOnly verifies that a request
// with only typed convenience fields serialises to a valid "fields" object
// with no "properties" or "historyMetadata" keys.
func TestCreateIssueRequest_MarshalJSON_TypedFieldsOnly(t *testing.T) {
	req := client.CreateIssueRequest{
		Fields: client.CreateIssueFields{
			Project:   client.IDObj{Key: "PROJ"},
			Summary:   "Hello world",
			IssueType: client.NamedObj{Name: "Bug"},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	s := string(data)

	for _, want := range []string{`"summary":"Hello world"`, `"key":"PROJ"`, `"name":"Bug"`} {
		if !strings.Contains(s, want) {
			t.Errorf("marshaled JSON missing %q: %s", want, s)
		}
	}
	for _, absent := range []string{"properties", "historyMetadata"} {
		if strings.Contains(s, absent) {
			t.Errorf("marshaled JSON should not contain %q when unset: %s", absent, s)
		}
	}
}

// TestCreateIssueRequest_MarshalJSON_ExtraFields verifies that ExtraFields
// entries are merged into the "fields" object alongside typed fields.
func TestCreateIssueRequest_MarshalJSON_ExtraFields(t *testing.T) {
	req := client.CreateIssueRequest{
		Fields: client.CreateIssueFields{
			Project:   client.IDObj{Key: "PROJ"},
			Summary:   "My story",
			IssueType: client.NamedObj{Name: "Story"},
		},
		ExtraFields: map[string]interface{}{
			"customfield_31004": map[string]string{"id": "50628"},
			"customfield_10200": "EPIC-1",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	s := string(data)

	for _, want := range []string{
		`"summary":"My story"`,
		`"customfield_31004"`,
		`"customfield_10200":"EPIC-1"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("marshaled JSON missing %q: %s", want, s)
		}
	}
	// Both typed and extra fields must be inside "fields", not at the top level.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatalf("unmarshal top-level: %v", err)
	}
	if _, ok := top["fields"]; !ok {
		t.Errorf("top-level JSON missing 'fields' key: %s", s)
	}
	if _, ok := top["customfield_31004"]; ok {
		t.Errorf("customfield_31004 must be nested inside 'fields', not at top level: %s", s)
	}
}

// TestCreateIssueRequest_MarshalJSON_ExtraFieldsOverrideTyped verifies that
// an ExtraFields entry with the same key as a typed field wins.
func TestCreateIssueRequest_MarshalJSON_ExtraFieldsOverrideTyped(t *testing.T) {
	req := client.CreateIssueRequest{
		Fields: client.CreateIssueFields{
			Project:   client.IDObj{Key: "PROJ"},
			Summary:   "original title",
			IssueType: client.NamedObj{Name: "Task"},
		},
		ExtraFields: map[string]interface{}{
			"summary": "overridden title",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	s := string(data)

	if !strings.Contains(s, "overridden title") {
		t.Errorf("ExtraFields should override typed Summary: %s", s)
	}
	if strings.Contains(s, "original title") {
		t.Errorf("original title should be replaced by ExtraFields override: %s", s)
	}
}

// TestCreateIssueRequest_MarshalJSON_Properties verifies that Properties are
// serialised at the top level of the request body, not inside "fields".
func TestCreateIssueRequest_MarshalJSON_Properties(t *testing.T) {
	req := client.CreateIssueRequest{
		Fields: client.CreateIssueFields{
			Project:   client.IDObj{Key: "PROJ"},
			Summary:   "With properties",
			IssueType: client.NamedObj{Name: "Task"},
		},
		Properties: []map[string]interface{}{
			{"key": "pipeline.id", "value": "build-42"},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatalf("unmarshal top-level: %v", err)
	}
	if _, ok := top["properties"]; !ok {
		t.Errorf("top-level JSON missing 'properties' key: %s", data)
	}
	// Properties must NOT be nested inside "fields".
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(top["fields"], &fields); err != nil {
		t.Fatalf("unmarshal fields: %v", err)
	}
	if _, ok := fields["properties"]; ok {
		t.Errorf("'properties' must be at top level, not inside 'fields': %s", data)
	}

	s := string(data)
	for _, want := range []string{`"pipeline.id"`, `"build-42"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing property value %q: %s", want, s)
		}
	}
}

// TestCreateIssueRequest_MarshalJSON_HistoryMetadata verifies that
// HistoryMetadata is serialised at the top level as "historyMetadata".
func TestCreateIssueRequest_MarshalJSON_HistoryMetadata(t *testing.T) {
	req := client.CreateIssueRequest{
		Fields: client.CreateIssueFields{
			Project:   client.IDObj{Key: "PROJ"},
			Summary:   "CI issue",
			IssueType: client.NamedObj{Name: "Task"},
		},
		HistoryMetadata: map[string]interface{}{
			"activityDescription": "Created by CI",
			"actor":               map[string]string{"id": "ci-bot", "type": "automation"},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatalf("unmarshal top-level: %v", err)
	}
	if _, ok := top["historyMetadata"]; !ok {
		t.Errorf("top-level JSON missing 'historyMetadata': %s", data)
	}

	s := string(data)
	for _, want := range []string{`"activityDescription"`, `"Created by CI"`, `"ci-bot"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing historyMetadata value %q: %s", want, s)
		}
	}
}

// TestCreateIssueRequest_MarshalJSON_AllOptions verifies that all three
// optional top-level fields (ExtraFields merged into fields, Properties, and
// HistoryMetadata) are included when all are set simultaneously.
func TestCreateIssueRequest_MarshalJSON_AllOptions(t *testing.T) {
	req := client.CreateIssueRequest{
		Fields: client.CreateIssueFields{
			Project:   client.IDObj{Key: "PROJ"},
			Summary:   "Full request",
			IssueType: client.NamedObj{Name: "Bug"},
		},
		ExtraFields: map[string]interface{}{
			"customfield_99999": "extra-value",
		},
		Properties: []map[string]interface{}{
			{"key": "env", "value": "production"},
		},
		HistoryMetadata: map[string]interface{}{
			"type": "madeAutomatically",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	s := string(data)

	for _, want := range []string{
		`"customfield_99999"`, `"extra-value"`,
		`"properties"`, `"env"`, `"production"`,
		`"historyMetadata"`, `"madeAutomatically"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q: %s", want, s)
		}
	}
}

// TestCreateIssue_RequestBody verifies that CreateIssue sends the correct
// JSON body to the API, including ExtraFields merged into "fields".
func TestCreateIssue_RequestBody(t *testing.T) {
	var gotBody map[string]json.RawMessage
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","key":"PROJ-1","self":""}`))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	req := &client.CreateIssueRequest{
		Fields: client.CreateIssueFields{
			Project:   client.IDObj{Key: "PROJ"},
			Summary:   "Test body",
			IssueType: client.NamedObj{Name: "Story"},
			Priority:  &client.NamedObj{Name: "High"},
		},
		ExtraFields: map[string]interface{}{
			"customfield_31004": map[string]string{"id": "50628"},
		},
		Properties: []map[string]interface{}{
			{"key": "build", "value": "99"},
		},
		HistoryMetadata: map[string]interface{}{
			"activityDescription": "test run",
		},
	}

	if _, err := c.CreateIssue(context.Background(), req); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}

	if gotBody == nil {
		t.Fatal("server received no body")
	}
	fieldsRaw, ok := gotBody["fields"]
	if !ok {
		t.Fatalf("request body missing 'fields': %v", gotBody)
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(fieldsRaw, &fields)

	for _, key := range []string{"summary", "project", "issuetype", "priority", "customfield_31004"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("fields missing %q", key)
		}
	}
	if _, ok := gotBody["properties"]; !ok {
		t.Errorf("request body missing 'properties'")
	}
	if _, ok := gotBody["historyMetadata"]; !ok {
		t.Errorf("request body missing 'historyMetadata'")
	}
}

// -----------------------------------------------------------------------
// UpdateIssueRequest
// -----------------------------------------------------------------------

// TestUpdateIssueRequest_MarshalJSON_FieldsOnly verifies that a plain update
// with only "fields" serialises correctly and omits empty optional keys.
func TestUpdateIssueRequest_MarshalJSON_FieldsOnly(t *testing.T) {
	req := client.UpdateIssueRequest{
		Fields: map[string]interface{}{
			"summary":  "Updated title",
			"priority": map[string]string{"name": "High"},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	s := string(data)

	for _, want := range []string{`"Updated title"`, `"High"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q: %s", want, s)
		}
	}
	for _, absent := range []string{"properties", "historyMetadata"} {
		if strings.Contains(s, absent) {
			t.Errorf("should not contain %q when unset: %s", absent, s)
		}
	}
}

// TestUpdateIssueRequest_MarshalJSON_WithProperties verifies that
// Properties are included when set.
func TestUpdateIssueRequest_MarshalJSON_WithProperties(t *testing.T) {
	req := client.UpdateIssueRequest{
		Fields: map[string]interface{}{"summary": "title"},
		Properties: []map[string]interface{}{
			{"key": "app.version", "value": "2.0"},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := top["properties"]; !ok {
		t.Errorf("missing 'properties': %s", data)
	}
	s := string(data)
	if !strings.Contains(s, `"app.version"`) {
		t.Errorf("missing property key: %s", s)
	}
}

// TestUpdateIssueRequest_MarshalJSON_WithHistoryMetadata verifies that
// HistoryMetadata is included under "historyMetadata" when set.
func TestUpdateIssueRequest_MarshalJSON_WithHistoryMetadata(t *testing.T) {
	req := client.UpdateIssueRequest{
		Fields: map[string]interface{}{"summary": "auto-fix"},
		HistoryMetadata: map[string]interface{}{
			"activityDescription": "Fixed by bot",
			"actor":               map[string]string{"id": "fix-bot", "type": "automation"},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	s := string(data)

	for _, want := range []string{`"historyMetadata"`, `"Fixed by bot"`, `"fix-bot"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q: %s", want, s)
		}
	}
}

// TestUpdateIssue_RequestBody verifies that UpdateIssue sends the correct
// JSON body including fields, properties, and historyMetadata.
func TestUpdateIssue_RequestBody(t *testing.T) {
	var gotBody map[string]json.RawMessage
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	req := &client.UpdateIssueRequest{
		Fields: map[string]interface{}{
			"summary":           "New summary",
			"customfield_31004": map[string]string{"id": "50628"},
		},
		Properties: []map[string]interface{}{
			{"key": "env", "value": "staging"},
		},
		HistoryMetadata: map[string]interface{}{
			"type": "madeAutomatically",
		},
	}

	if err := c.UpdateIssue(context.Background(), "PROJ-1", req); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}

	if gotBody == nil {
		t.Fatal("server received no body")
	}
	for _, key := range []string{"fields", "properties", "historyMetadata"} {
		if _, ok := gotBody[key]; !ok {
			t.Errorf("request body missing %q", key)
		}
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(gotBody["fields"], &fields)
	for _, key := range []string{"summary", "customfield_31004"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("fields missing %q", key)
		}
	}
}

// TestUpdateIssue_EmptyRequest verifies that an UpdateIssueRequest with no
// fields, properties, or historyMetadata still serialises without error and
// produces an empty-fields body (the guard against empty requests is enforced
// at the command layer, not the client layer).
func TestUpdateIssue_EmptyRequest(t *testing.T) {
	req := client.UpdateIssueRequest{}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	// fields key should be absent (omitempty) or an empty object.
	s := string(data)
	// Must not panic or return invalid JSON.
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		t.Errorf("empty UpdateIssueRequest produced invalid JSON: %s", s)
	}
}
