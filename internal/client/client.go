// Package client provides an HTTP client for the Jira REST API v2 and the
// Jira Agile REST API (boards/sprints).
//
// Authentication is performed using a Bearer token (Jira Cloud personal access
// token or Jira Server/Data Center personal access token).
//
// The client automatically prefixes all requests with the configured server
// URL and the appropriate API base path:
//
//   - /rest/api/2  – core Jira API (v2)
//   - /rest/agile/1.0 – Jira Agile (boards, sprints)
package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/steveohara/jcli/internal/config"
)

const (
	apiBase   = "/rest/api/2"
	agileBase = "/rest/agile/1.0"

	defaultTimeout = 30 * time.Second
)

// Client is a configured Jira API client.
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

// New creates a new Client from the supplied configuration.
func New(cfg *config.Config) *Client {
	timeout := defaultTimeout
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Second
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.Insecure, // #nosec G402 – user-controlled flag
		},
	}
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
}

// -----------------------------------------------------------------------
// Low-level HTTP helpers
// -----------------------------------------------------------------------

// do executes an HTTP request and decodes the JSON response body into out (if
// out is non-nil).  A non-2xx status code is returned as an error that
// includes the response body for debugging.
func (c *Client) do(ctx context.Context, method, apiPath string, body, out interface{}) error {
	// Build URL
	rawURL := c.cfg.Server + apiPath
	if c.cfg.Verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", method, rawURL)
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		if c.cfg.Verbose {
			fmt.Fprintf(os.Stderr, "> Body: %s\n", b)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		// Jira Server enforces XSRF protection on non-GET requests.
		req.Header.Set("X-Atlassian-Token", "no-check")
	}

	// --debug: print the equivalent curl command and exit without sending.
	if c.cfg.Debug {
		headers := []header{
			{"Authorization", "Bearer " + c.cfg.Token},
			{"Accept", "application/json"},
		}
		var bodyBytes []byte
		if body != nil {
			headers = append(headers, header{"Content-Type", "application/json"})
			headers = append(headers, header{"X-Atlassian-Token", "no-check"})
			bodyBytes, _ = json.Marshal(body)
		}
		fmt.Println(formatCurl(method, rawURL, headers, bodyBytes, c.cfg.Insecure))
		os.Exit(0)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if c.cfg.Verbose {
		fmt.Fprintf(os.Stderr, "< %s\n< Body: %s\n", resp.Status, respBody)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return parseAPIError(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// get is a convenience wrapper for GET requests.
func (c *Client) get(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

// post is a convenience wrapper for POST requests.
func (c *Client) post(ctx context.Context, path string, body, out interface{}) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

// put is a convenience wrapper for PUT requests.
func (c *Client) put(ctx context.Context, path string, body, out interface{}) error {
	return c.do(ctx, http.MethodPut, path, body, out)
}

// del is a convenience wrapper for DELETE requests.
func (c *Client) del(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// buildQuery appends url.Values to a path string.
func buildQuery(path string, params url.Values) string {
	if len(params) == 0 {
		return path
	}
	return path + "?" + params.Encode()
}

// apiError represents a structured error from the Jira API.
type apiError struct {
	StatusCode    int
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
}

func (e *apiError) Error() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("HTTP %d", e.StatusCode))
	parts = append(parts, e.ErrorMessages...)
	for k, v := range e.Errors {
		parts = append(parts, fmt.Sprintf("%s: %s", k, v))
	}
	if hint := statusHint(e.StatusCode); hint != "" {
		parts = append(parts, hint)
	}
	return strings.Join(parts, "; ")
}

// statusHint returns a short actionable hint for well-known HTTP error codes.
func statusHint(code int) string {
	switch code {
	case http.StatusUnauthorized:
		return "hint: check your API token (--token / JIRA_API_TOKEN)"
	case http.StatusForbidden:
		return "hint: you do not have permission for this action"
	case http.StatusNotFound:
		return "hint: the resource was not found – check the issue key or project"
	case http.StatusBadRequest:
		return "hint: the request was rejected – verify all required fields and values"
	default:
		return ""
	}
}

func parseAPIError(statusCode int, contentType string, body []byte) error {
	e := &apiError{StatusCode: statusCode}

	// If the server returned HTML (e.g. a proxy/gateway error page), extract
	// the readable text rather than dumping raw markup.
	if isHTML(contentType, body) {
		text := htmlToText(body, 8)
		if text == "" {
			text = http.StatusText(statusCode)
		}
		e.ErrorMessages = []string{text}
		return e
	}

	_ = json.Unmarshal(body, e) // best-effort JSON decode
	if len(e.ErrorMessages) == 0 && len(e.Errors) == 0 {
		e.ErrorMessages = []string{string(body)}
	}
	return e
}

// -----------------------------------------------------------------------
// Issues – /rest/api/2/issue
// -----------------------------------------------------------------------

// Issue represents a Jira issue as returned by the API.
type Issue struct {
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Self   string      `json:"self,omitempty"`
	Fields IssueFields `json:"fields"`

	// Raw stores the original JSON bytes received from the API.
	// It is used by --all-fields to output the response without the
	// omitempty suppression applied during normal typed marshaling.
	Raw json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes a Jira issue, capturing the raw bytes alongside the
// typed fields so that callers can round-trip the original payload when needed.
func (i *Issue) UnmarshalJSON(data []byte) error {
	// Alias breaks the recursion; Fields is still typed as IssueFields so its
	// own UnmarshalJSON (which populates Extra) is called normally.
	type rawIssue Issue
	var raw rawIssue
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*i = Issue(raw)
	i.Raw = append(json.RawMessage(nil), data...) // defensive copy
	return nil
}

// IssueFields contains the field values of a Jira issue.
type IssueFields struct {
	Summary      string        `json:"summary,omitempty"`
	Description  ADFString     `json:"description,omitempty"`
	Status       NamedObj      `json:"status,omitempty"`
	Priority     NamedObj      `json:"priority,omitempty"`
	Assignee     *User         `json:"assignee,omitempty"`
	Reporter     *User         `json:"reporter,omitempty"`
	IssueType    NamedObj      `json:"issuetype,omitempty"`
	Project      ProjectShort  `json:"project,omitempty"`
	Labels       []string      `json:"labels,omitempty"`
	Components   []NamedObj    `json:"components,omitempty"`
	FixVersions  []NamedObj    `json:"fixVersions,omitempty"`
	Created      string        `json:"created,omitempty"`
	Updated      string        `json:"updated,omitempty"`
	DueDate      string        `json:"duedate,omitempty"`
	Votes        *Votes        `json:"votes,omitempty"`
	Watches      *Watches      `json:"watches,omitempty"`
	Comment      *CommentList  `json:"comment,omitempty"`
	TimeTracking *TimeTracking `json:"timetracking,omitempty"`
	Parent       *IssueRef     `json:"parent,omitempty"`

	// Extra holds any fields not explicitly modelled above (e.g. customfield_10001).
	// Values are stored as raw JSON for lazy decoding; use FormatCustomField to render them.
	Extra map[string]json.RawMessage `json:"-"`
}

// knownIssueFields is the set of JSON field names that are explicitly decoded
// into IssueFields.  Everything else ends up in Extra.
var knownIssueFields = map[string]bool{
	"summary": true, "description": true, "status": true, "priority": true,
	"assignee": true, "reporter": true, "issuetype": true, "project": true,
	"labels": true, "components": true, "fixVersions": true, "created": true,
	"updated": true, "duedate": true, "votes": true, "watches": true,
	"comment": true, "timetracking": true, "parent": true,
}

// UnmarshalJSON decodes the Jira fields object.  Known fields are decoded into
// their typed struct fields; unrecognised fields (e.g. custom fields) are
// stored verbatim in Extra.
func (f *IssueFields) UnmarshalJSON(data []byte) error {
	// Use a type alias to call the default decoder without recursion.
	type rawFields IssueFields
	var raw rawFields
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*f = IssueFields(raw)

	// Capture all top-level keys so that custom fields can be retrieved later.
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}
	f.Extra = make(map[string]json.RawMessage, len(all))
	for k, v := range all {
		if !knownIssueFields[k] {
			f.Extra[k] = v
		}
	}
	return nil
}

// MarshalJSON serialises IssueFields including any custom fields stored in
// Extra, so that --output json round-trips all field data back to the caller.
// It also manually omits zero-value struct fields (NamedObj, ProjectShort)
// that encoding/json's omitempty cannot handle — omitempty only suppresses
// pointers, strings, slices, maps, and numeric types, not struct values.
func (f IssueFields) MarshalJSON() ([]byte, error) {
	// Serialise known struct fields via alias (Extra skipped by json:"-").
	type rawFields IssueFields
	known, err := json.Marshal(rawFields(f))
	if err != nil {
		return nil, err
	}

	// Decode into a map so we can post-process individual entries.
	var m map[string]json.RawMessage
	if err := json.Unmarshal(known, &m); err != nil {
		return nil, err
	}

	// Remove struct-typed fields that hold only zero / empty values.
	// This covers the case where the API did not return the field (e.g.
	// "priority": null) so the Go struct was left at its zero value.
	for _, key := range []string{"status", "priority", "issuetype", "project"} {
		if v, ok := m[key]; ok && isJSONObjectEmpty(v) {
			delete(m, key)
		}
	}

	// Merge in custom / extra fields.
	for k, v := range f.Extra {
		m[k] = v
	}

	return json.Marshal(m)
}

// isJSONObjectEmpty reports whether raw is a JSON object in which every value
// is either an empty string or null.  It is used to detect zero-value structs
// that were not present in the API response (e.g. NamedObj{}, ProjectShort{}).
func isJSONObjectEmpty(raw json.RawMessage) bool {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return false // not an object – leave as-is
	}
	for _, v := range m {
		switch val := v.(type) {
		case string:
			if val != "" {
				return false
			}
		case nil:
			// null counts as empty
		default:
			return false // number, bool, object, array – definitely not empty
		}
	}
	return true
}

// FormatCustomField converts a raw JSON value from IssueFields.Extra into a
// human-readable string.  It handles common Jira value shapes (plain strings,
// named objects, arrays) without requiring callers to know the field schema.
func FormatCustomField(raw json.RawMessage) string {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return formatFieldValue(v)
}

func formatFieldValue(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%v", val)
	case map[string]interface{}:
		// Prefer the most human-friendly key available.
		for _, key := range []string{"displayName", "name", "value", "key", "id"} {
			if s, ok := val[key].(string); ok {
				return s
			}
		}
		b, _ := json.Marshal(val)
		return string(b)
	case []interface{}:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			parts = append(parts, formatFieldValue(item))
		}
		return strings.Join(parts, ", ")
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// IssueRef is a lightweight reference to another issue (e.g. parent).
type IssueRef struct {
	ID  string `json:"id,omitempty"`
	Key string `json:"key,omitempty"`
}

// NamedObj is a simple name-only sub-object used throughout the Jira API.
type NamedObj struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	Self string `json:"self,omitempty"`
}

// ProjectShort is a minimal project representation.
type ProjectShort struct {
	ID   string `json:"id,omitempty"`
	Key  string `json:"key,omitempty"`
	Name string `json:"name,omitempty"`
}

// Votes holds vote count information.
type Votes struct {
	Votes    int  `json:"votes"`
	HasVoted bool `json:"hasVoted"`
}

// Watches holds watcher count information.
type Watches struct {
	WatchCount int  `json:"watchCount"`
	IsWatching bool `json:"isWatching"`
}

// TimeTracking holds time estimate information.
type TimeTracking struct {
	OriginalEstimate         string `json:"originalEstimate,omitempty"`
	RemainingEstimate        string `json:"remainingEstimate,omitempty"`
	TimeSpent                string `json:"timeSpent,omitempty"`
	OriginalEstimateSeconds  int    `json:"originalEstimateSeconds,omitempty"`
	RemainingEstimateSeconds int    `json:"remainingEstimateSeconds,omitempty"`
	TimeSpentSeconds         int    `json:"timeSpentSeconds,omitempty"`
}

// GetIssue retrieves a single Jira issue by its key or ID.
//
// API: GET /rest/api/2/issue/{issueIdOrKey}
func (c *Client) GetIssue(ctx context.Context, issueKey string, fields []string) (*Issue, error) {
	params := url.Values{}
	if len(fields) > 0 {
		params.Set("fields", strings.Join(fields, ","))
	}
	var issue Issue
	path := buildQuery(fmt.Sprintf("%s/issue/%s", apiBase, issueKey), params)
	return &issue, c.get(ctx, path, &issue)
}

// CreateIssueRequest is the payload for creating a new issue.
type CreateIssueRequest struct {
	Fields CreateIssueFields `json:"fields"`
}

// CreateIssueFields contains the field values for issue creation.
type CreateIssueFields struct {
	Project     IDObj     `json:"project"`
	Summary     string    `json:"summary"`
	Description string    `json:"description,omitempty"`
	IssueType   NamedObj  `json:"issuetype"`
	Priority    *NamedObj `json:"priority,omitempty"`
	Assignee    *IDObj    `json:"assignee,omitempty"`
	Labels      []string  `json:"labels,omitempty"`
	Components  []IDObj   `json:"components,omitempty"`
	FixVersions []IDObj   `json:"fixVersions,omitempty"`
	DueDate     string    `json:"duedate,omitempty"`
	Parent      *IDObj    `json:"parent,omitempty"`
}

// IDObj is a simple ID-only reference.
type IDObj struct {
	ID  string `json:"id,omitempty"`
	Key string `json:"key,omitempty"`
}

// CreateIssueResponse is the response from creating a new issue.
type CreateIssueResponse struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// CreateIssue creates a new Jira issue.
//
// API: POST /rest/api/2/issue
func (c *Client) CreateIssue(ctx context.Context, req *CreateIssueRequest) (*CreateIssueResponse, error) {
	var resp CreateIssueResponse
	return &resp, c.post(ctx, apiBase+"/issue", req, &resp)
}

// UpdateIssueRequest is the payload for updating an existing issue.
type UpdateIssueRequest struct {
	Fields map[string]interface{} `json:"fields"`
}

// UpdateIssue updates an existing Jira issue.
//
// API: PUT /rest/api/2/issue/{issueIdOrKey}
func (c *Client) UpdateIssue(ctx context.Context, issueKey string, req *UpdateIssueRequest) error {
	return c.put(ctx, fmt.Sprintf("%s/issue/%s", apiBase, issueKey), req, nil)
}

// DeleteIssue deletes a Jira issue.
//
// API: DELETE /rest/api/2/issue/{issueIdOrKey}
func (c *Client) DeleteIssue(ctx context.Context, issueKey string, deleteSubtasks bool) error {
	path := fmt.Sprintf("%s/issue/%s", apiBase, issueKey)
	if deleteSubtasks {
		path += "?deleteSubtasks=true"
	}
	return c.del(ctx, path)
}

// -----------------------------------------------------------------------
// Issue Search – /rest/api/2/search
// -----------------------------------------------------------------------

// SearchResult is the response from the search endpoint.
type SearchResult struct {
	Total      int     `json:"total"`
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
	Issues     []Issue `json:"issues"`
}

// SearchOptions configures a JQL search.
type SearchOptions struct {
	JQL        string
	Fields     []string
	StartAt    int
	MaxResults int
}

// SearchIssues executes a JQL search and returns matching issues.
//
// API: GET /rest/api/2/search
func (c *Client) SearchIssues(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	maxResults := opts.MaxResults
	if maxResults == 0 {
		maxResults = 50
	}
	params := url.Values{}
	params.Set("jql", opts.JQL)
	params.Set("startAt", fmt.Sprintf("%d", opts.StartAt))
	params.Set("maxResults", fmt.Sprintf("%d", maxResults))
	if len(opts.Fields) > 0 {
		params.Set("fields", strings.Join(opts.Fields, ","))
	}
	var result SearchResult
	return &result, c.get(ctx, buildQuery(apiBase+"/search", params), &result)
}

// -----------------------------------------------------------------------
// Issue Comments – /rest/api/2/issue/{issueIdOrKey}/comment
// -----------------------------------------------------------------------

// Comment represents a Jira issue comment.
type Comment struct {
	ID      string    `json:"id"`
	Self    string    `json:"self"`
	Author  *User     `json:"author"`
	Body    ADFString `json:"body"`
	Created string    `json:"created"`
	Updated string    `json:"updated"`
}

// CommentList is the paginated list of comments returned by the API.
type CommentList struct {
	Total      int       `json:"total"`
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Comments   []Comment `json:"comments"`
}

// GetComments retrieves all comments for an issue.
//
// API: GET /rest/api/2/issue/{issueIdOrKey}/comment
func (c *Client) GetComments(ctx context.Context, issueKey string) (*CommentList, error) {
	var list CommentList
	return &list, c.get(ctx, fmt.Sprintf("%s/issue/%s/comment", apiBase, issueKey), &list)
}

// AddComment adds a comment to an issue.
//
// API: POST /rest/api/2/issue/{issueIdOrKey}/comment
func (c *Client) AddComment(ctx context.Context, issueKey, body string) (*Comment, error) {
	payload := map[string]interface{}{"body": body}
	var comment Comment
	return &comment, c.post(ctx, fmt.Sprintf("%s/issue/%s/comment", apiBase, issueKey), payload, &comment)
}

// UpdateComment updates an existing comment.
//
// API: PUT /rest/api/2/issue/{issueIdOrKey}/comment/{id}
func (c *Client) UpdateComment(ctx context.Context, issueKey, commentID, body string) (*Comment, error) {
	payload := map[string]interface{}{"body": body}
	var comment Comment
	return &comment, c.put(ctx, fmt.Sprintf("%s/issue/%s/comment/%s", apiBase, issueKey, commentID), payload, &comment)
}

// DeleteComment deletes a comment.
//
// API: DELETE /rest/api/2/issue/{issueIdOrKey}/comment/{id}
func (c *Client) DeleteComment(ctx context.Context, issueKey, commentID string) error {
	return c.del(ctx, fmt.Sprintf("%s/issue/%s/comment/%s", apiBase, issueKey, commentID))
}

// -----------------------------------------------------------------------
// Issue Transitions – /rest/api/2/issue/{issueIdOrKey}/transitions
// -----------------------------------------------------------------------

// Transition represents a Jira workflow transition.
type Transition struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	To   NamedObj `json:"to"`
}

// TransitionList is the list of available transitions for an issue.
type TransitionList struct {
	Transitions []Transition `json:"transitions"`
}

// GetTransitions returns the available transitions for an issue.
//
// API: GET /rest/api/2/issue/{issueIdOrKey}/transitions
func (c *Client) GetTransitions(ctx context.Context, issueKey string) (*TransitionList, error) {
	var list TransitionList
	return &list, c.get(ctx, fmt.Sprintf("%s/issue/%s/transitions", apiBase, issueKey), &list)
}

// TransitionIssue moves an issue to a new state by transition ID.
//
// API: POST /rest/api/2/issue/{issueIdOrKey}/transitions
func (c *Client) TransitionIssue(ctx context.Context, issueKey, transitionID string, fields map[string]interface{}) error {
	payload := map[string]interface{}{
		"transition": map[string]string{"id": transitionID},
	}
	if len(fields) > 0 {
		payload["fields"] = fields
	}
	return c.post(ctx, fmt.Sprintf("%s/issue/%s/transitions", apiBase, issueKey), payload, nil)
}

// -----------------------------------------------------------------------
// Issue Assignee – /rest/api/2/issue/{issueIdOrKey}/assignee
// -----------------------------------------------------------------------

// AssignIssue assigns an issue to a user.  Pass an empty accountID to
// unassign.
//
// API: PUT /rest/api/2/issue/{issueIdOrKey}/assignee
func (c *Client) AssignIssue(ctx context.Context, issueKey, accountID string) error {
	var payload interface{}
	if accountID == "" {
		payload = map[string]interface{}{"accountId": nil}
	} else {
		payload = map[string]string{"accountId": accountID}
	}
	return c.put(ctx, fmt.Sprintf("%s/issue/%s/assignee", apiBase, issueKey), payload, nil)
}

// -----------------------------------------------------------------------
// Issue Worklog – /rest/api/2/issue/{issueIdOrKey}/worklog
// -----------------------------------------------------------------------

// Worklog represents a Jira worklog entry.
type Worklog struct {
	ID               string    `json:"id"`
	Self             string    `json:"self"`
	Author           *User     `json:"author"`
	Comment          ADFString `json:"comment"`
	Started          string    `json:"started"`
	TimeSpent        string    `json:"timeSpent"`
	TimeSpentSeconds int       `json:"timeSpentSeconds"`
}

// WorklogList is a paginated list of worklogs.
type WorklogList struct {
	Total      int       `json:"total"`
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Worklogs   []Worklog `json:"worklogs"`
}

// GetWorklogs retrieves worklogs for an issue.
//
// API: GET /rest/api/2/issue/{issueIdOrKey}/worklog
func (c *Client) GetWorklogs(ctx context.Context, issueKey string) (*WorklogList, error) {
	var list WorklogList
	return &list, c.get(ctx, fmt.Sprintf("%s/issue/%s/worklog", apiBase, issueKey), &list)
}

// AddWorklog adds a worklog entry to an issue.
//
// API: POST /rest/api/2/issue/{issueIdOrKey}/worklog
func (c *Client) AddWorklog(ctx context.Context, issueKey, timeSpent, started, comment string) (*Worklog, error) {
	payload := map[string]interface{}{
		"timeSpent": timeSpent,
		"started":   started,
		"comment":   comment,
	}
	var wl Worklog
	return &wl, c.post(ctx, fmt.Sprintf("%s/issue/%s/worklog", apiBase, issueKey), payload, &wl)
}

// DeleteWorklog deletes a worklog entry.
//
// API: DELETE /rest/api/2/issue/{issueIdOrKey}/worklog/{id}
func (c *Client) DeleteWorklog(ctx context.Context, issueKey, worklogID string) error {
	return c.del(ctx, fmt.Sprintf("%s/issue/%s/worklog/%s", apiBase, issueKey, worklogID))
}

// -----------------------------------------------------------------------
// Issue Votes – /rest/api/2/issue/{issueIdOrKey}/votes
// -----------------------------------------------------------------------

// GetVotes retrieves vote information for an issue.
//
// API: GET /rest/api/2/issue/{issueIdOrKey}/votes
func (c *Client) GetVotes(ctx context.Context, issueKey string) (*Votes, error) {
	var v Votes
	return &v, c.get(ctx, fmt.Sprintf("%s/issue/%s/votes", apiBase, issueKey), &v)
}

// AddVote casts a vote for an issue.
//
// API: POST /rest/api/2/issue/{issueIdOrKey}/votes
func (c *Client) AddVote(ctx context.Context, issueKey string) error {
	return c.post(ctx, fmt.Sprintf("%s/issue/%s/votes", apiBase, issueKey), nil, nil)
}

// RemoveVote removes the current user's vote from an issue.
//
// API: DELETE /rest/api/2/issue/{issueIdOrKey}/votes
func (c *Client) RemoveVote(ctx context.Context, issueKey string) error {
	return c.del(ctx, fmt.Sprintf("%s/issue/%s/votes", apiBase, issueKey))
}

// -----------------------------------------------------------------------
// Issue Watchers – /rest/api/2/issue/{issueIdOrKey}/watchers
// -----------------------------------------------------------------------

// WatcherList holds watcher information for an issue.
type WatcherList struct {
	Self       string `json:"self"`
	WatchCount int    `json:"watchCount"`
	IsWatching bool   `json:"isWatching"`
	Watchers   []User `json:"watchers"`
}

// GetWatchers retrieves the watcher list for an issue.
//
// API: GET /rest/api/2/issue/{issueIdOrKey}/watchers
func (c *Client) GetWatchers(ctx context.Context, issueKey string) (*WatcherList, error) {
	var w WatcherList
	return &w, c.get(ctx, fmt.Sprintf("%s/issue/%s/watchers", apiBase, issueKey), &w)
}

// AddWatcher adds a user as a watcher on an issue.
//
// API: POST /rest/api/2/issue/{issueIdOrKey}/watchers
func (c *Client) AddWatcher(ctx context.Context, issueKey, accountID string) error {
	return c.post(ctx, fmt.Sprintf("%s/issue/%s/watchers", apiBase, issueKey), accountID, nil)
}

// RemoveWatcher removes a watcher from an issue.
//
// API: DELETE /rest/api/2/issue/{issueIdOrKey}/watchers?accountId={accountId}
func (c *Client) RemoveWatcher(ctx context.Context, issueKey, accountID string) error {
	path := fmt.Sprintf("%s/issue/%s/watchers?accountId=%s", apiBase, issueKey, accountID)
	return c.del(ctx, path)
}

// -----------------------------------------------------------------------
// Issue Attachments – /rest/api/2/issue/{issueIdOrKey}/attachments
// -----------------------------------------------------------------------

// Attachment represents a file attached to a Jira issue.
type Attachment struct {
	ID       string `json:"id"`
	Self     string `json:"self"`
	Filename string `json:"filename"`
	Author   *User  `json:"author"`
	Created  string `json:"created"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
	Content  string `json:"content"`
}

// AddAttachment uploads a file as an attachment to an issue.
//
// API: POST /rest/api/2/issue/{issueIdOrKey}/attachments
func (c *Client) AddAttachment(ctx context.Context, issueKey, filePath string) ([]Attachment, error) {
	f, err := os.Open(filePath) // #nosec G304 – caller-provided path
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(fw, f); err != nil {
		return nil, err
	}
	w.Close()

	rawURL := c.cfg.Server + fmt.Sprintf("%s/issue/%s/attachments", apiBase, issueKey)

	// --debug: print the equivalent curl command and exit without sending.
	if c.cfg.Debug {
		headers := []header{
			{"Authorization", "Bearer " + c.cfg.Token},
			{"X-Atlassian-Token", "no-check"},
		}
		fmt.Println(formatCurl(http.MethodPost, rawURL, headers, nil, c.cfg.Insecure) +
			fmt.Sprintf(" \\\n  -F 'file=@%s'", filePath))
		os.Exit(0)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("X-Atlassian-Token", "no-check")
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, parseAPIError(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
	}
	var attachments []Attachment
	if err := json.Unmarshal(respBody, &attachments); err != nil {
		return nil, err
	}
	return attachments, nil
}

// DeleteAttachment deletes an attachment by ID.
//
// API: DELETE /rest/api/2/attachment/{id}
func (c *Client) DeleteAttachment(ctx context.Context, attachmentID string) error {
	return c.del(ctx, fmt.Sprintf("%s/attachment/%s", apiBase, attachmentID))
}

// -----------------------------------------------------------------------
// Issue Links – /rest/api/2/issueLink
// -----------------------------------------------------------------------

// IssueLinkType describes the type of an issue link.
type IssueLinkType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

// IssueLinkTypeList is a list of issue link types.
type IssueLinkTypeList struct {
	IssueLinkTypes []IssueLinkType `json:"issueLinkTypes"`
}

// GetIssueLinkTypes returns all available link types.
//
// API: GET /rest/api/2/issueLinkType
func (c *Client) GetIssueLinkTypes(ctx context.Context) (*IssueLinkTypeList, error) {
	var list IssueLinkTypeList
	return &list, c.get(ctx, apiBase+"/issueLinkType", &list)
}

// LinkIssues creates a link between two issues.
//
// API: POST /rest/api/2/issueLink
func (c *Client) LinkIssues(ctx context.Context, linkTypeName, inwardKey, outwardKey, comment string) error {
	payload := map[string]interface{}{
		"type":         map[string]string{"name": linkTypeName},
		"inwardIssue":  map[string]string{"key": inwardKey},
		"outwardIssue": map[string]string{"key": outwardKey},
	}
	if comment != "" {
		payload["comment"] = map[string]interface{}{"body": comment}
	}
	return c.post(ctx, apiBase+"/issueLink", payload, nil)
}

// DeleteIssueLink removes a link between issues.
//
// API: DELETE /rest/api/2/issueLink/{linkId}
func (c *Client) DeleteIssueLink(ctx context.Context, linkID string) error {
	return c.del(ctx, fmt.Sprintf("%s/issueLink/%s", apiBase, linkID))
}

// -----------------------------------------------------------------------
// Projects – /rest/api/2/project
// -----------------------------------------------------------------------

// Project represents a Jira project.
type Project struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Self        string `json:"self"`
	Description string `json:"description"`
	Lead        *User  `json:"lead"`
	ProjectType string `json:"projectTypeKey"`
	Style       string `json:"style"`
	IsPrivate   bool   `json:"isPrivate"`
}

// GetProjects returns all projects visible to the current user.
//
// API: GET /rest/api/2/project
func (c *Client) GetProjects(ctx context.Context) ([]Project, error) {
	var projects []Project
	return projects, c.get(ctx, apiBase+"/project", &projects)
}

// GetProject returns a single project by key or ID.
//
// API: GET /rest/api/2/project/{projectIdOrKey}
func (c *Client) GetProject(ctx context.Context, projectKey string) (*Project, error) {
	var project Project
	return &project, c.get(ctx, fmt.Sprintf("%s/project/%s", apiBase, projectKey), &project)
}

// CreateProjectRequest is the payload for creating a project.
type CreateProjectRequest struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	ProjectTypeKey string `json:"projectTypeKey"`
	LeadAccountID  string `json:"leadAccountId,omitempty"`
	AssigneeType   string `json:"assigneeType,omitempty"`
}

// CreateProject creates a new Jira project.
//
// API: POST /rest/api/2/project
func (c *Client) CreateProject(ctx context.Context, req *CreateProjectRequest) (*Project, error) {
	var project Project
	return &project, c.post(ctx, apiBase+"/project", req, &project)
}

// UpdateProject updates a project.
//
// API: PUT /rest/api/2/project/{projectIdOrKey}
func (c *Client) UpdateProject(ctx context.Context, projectKey string, fields map[string]interface{}) (*Project, error) {
	var project Project
	return &project, c.put(ctx, fmt.Sprintf("%s/project/%s", apiBase, projectKey), fields, &project)
}

// DeleteProject deletes a project.
//
// API: DELETE /rest/api/2/project/{projectIdOrKey}
func (c *Client) DeleteProject(ctx context.Context, projectKey string) error {
	return c.del(ctx, fmt.Sprintf("%s/project/%s", apiBase, projectKey))
}

// -----------------------------------------------------------------------
// Project Versions – /rest/api/2/project/{projectIdOrKey}/versions
// -----------------------------------------------------------------------

// Version represents a Jira project version (release).
type Version struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Released    bool   `json:"released"`
	Archived    bool   `json:"archived"`
	ReleaseDate string `json:"releaseDate"`
	ProjectID   int    `json:"projectId"`
}

// GetVersions returns all versions for a project.
//
// API: GET /rest/api/2/project/{projectIdOrKey}/versions
func (c *Client) GetVersions(ctx context.Context, projectKey string) ([]Version, error) {
	var versions []Version
	return versions, c.get(ctx, fmt.Sprintf("%s/project/%s/versions", apiBase, projectKey), &versions)
}

// CreateVersion creates a new project version.
//
// API: POST /rest/api/2/version
func (c *Client) CreateVersion(ctx context.Context, projectKey, name, description, releaseDate string) (*Version, error) {
	payload := map[string]interface{}{
		"project":     projectKey,
		"name":        name,
		"description": description,
		"releaseDate": releaseDate,
	}
	var version Version
	return &version, c.post(ctx, apiBase+"/version", payload, &version)
}

// UpdateVersion updates a project version.
//
// API: PUT /rest/api/2/version/{id}
func (c *Client) UpdateVersion(ctx context.Context, versionID string, fields map[string]interface{}) (*Version, error) {
	var version Version
	return &version, c.put(ctx, fmt.Sprintf("%s/version/%s", apiBase, versionID), fields, &version)
}

// DeleteVersion deletes a project version.
//
// API: DELETE /rest/api/2/version/{id}
func (c *Client) DeleteVersion(ctx context.Context, versionID string) error {
	return c.del(ctx, fmt.Sprintf("%s/version/%s", apiBase, versionID))
}

// -----------------------------------------------------------------------
// Project Components – /rest/api/2/project/{projectIdOrKey}/components
// -----------------------------------------------------------------------

// Component represents a Jira project component.
type Component struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Lead        *User  `json:"lead"`
	Project     string `json:"project"`
}

// GetComponents returns all components for a project.
//
// API: GET /rest/api/2/project/{projectIdOrKey}/components
func (c *Client) GetComponents(ctx context.Context, projectKey string) ([]Component, error) {
	var components []Component
	return components, c.get(ctx, fmt.Sprintf("%s/project/%s/components", apiBase, projectKey), &components)
}

// CreateComponent creates a new project component.
//
// API: POST /rest/api/2/component
func (c *Client) CreateComponent(ctx context.Context, projectKey, name, description string) (*Component, error) {
	payload := map[string]string{
		"project":     projectKey,
		"name":        name,
		"description": description,
	}
	var comp Component
	return &comp, c.post(ctx, apiBase+"/component", payload, &comp)
}

// UpdateComponent updates a project component.
//
// API: PUT /rest/api/2/component/{id}
func (c *Client) UpdateComponent(ctx context.Context, componentID string, fields map[string]interface{}) (*Component, error) {
	var comp Component
	return &comp, c.put(ctx, fmt.Sprintf("%s/component/%s", apiBase, componentID), fields, &comp)
}

// DeleteComponent deletes a project component.
//
// API: DELETE /rest/api/2/component/{id}
func (c *Client) DeleteComponent(ctx context.Context, componentID string) error {
	return c.del(ctx, fmt.Sprintf("%s/component/%s", apiBase, componentID))
}

// -----------------------------------------------------------------------
// Users – /rest/api/2/user
// -----------------------------------------------------------------------

// User represents a Jira user.
type User struct {
	AccountID    string `json:"accountId,omitempty"`
	Name         string `json:"name,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	EmailAddress string `json:"emailAddress,omitempty"`
	Active       bool   `json:"active"`
	Self         string `json:"self,omitempty"`
}

// GetUser retrieves a user by account ID.
//
// API: GET /rest/api/2/user?accountId={accountId}
func (c *Client) GetUser(ctx context.Context, accountID string) (*User, error) {
	var user User
	return &user, c.get(ctx, fmt.Sprintf("%s/user?accountId=%s", apiBase, accountID), &user)
}

// GetCurrentUser returns the currently authenticated user.
//
// API: GET /rest/api/2/myself
func (c *Client) GetCurrentUser(ctx context.Context) (*User, error) {
	var user User
	return &user, c.get(ctx, apiBase+"/myself", &user)
}

// SearchUsers searches for users.
//
// API: GET /rest/api/2/user/search?query={query}
func (c *Client) SearchUsers(ctx context.Context, query string, maxResults int) ([]User, error) {
	if maxResults == 0 {
		maxResults = 50
	}
	path := fmt.Sprintf("%s/user/search?query=%s&maxResults=%d", apiBase, url.QueryEscape(query), maxResults)
	var users []User
	return users, c.get(ctx, path, &users)
}

// -----------------------------------------------------------------------
// Issue Types – /rest/api/2/issuetype
// -----------------------------------------------------------------------

// IssueType represents a Jira issue type.
type IssueType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Subtask     bool   `json:"subtask"`
}

// GetIssueTypes returns all issue types.
//
// API: GET /rest/api/2/issuetype
func (c *Client) GetIssueTypes(ctx context.Context) ([]IssueType, error) {
	var types []IssueType
	return types, c.get(ctx, apiBase+"/issuetype", &types)
}

// -----------------------------------------------------------------------
// Priorities – /rest/api/2/priority
// -----------------------------------------------------------------------

// Priority represents a Jira issue priority.
type Priority struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// GetPriorities returns all issue priorities.
//
// API: GET /rest/api/2/priority
func (c *Client) GetPriorities(ctx context.Context) ([]Priority, error) {
	var priorities []Priority
	return priorities, c.get(ctx, apiBase+"/priority", &priorities)
}

// -----------------------------------------------------------------------
// Statuses – /rest/api/2/status
// -----------------------------------------------------------------------

// StatusCategory is the category of a Jira status. Its id field is a number
// in the REST API response (unlike most other id fields which are strings).
type StatusCategory struct {
	ID   int    `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Status represents a Jira issue status.
type Status struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    StatusCategory `json:"statusCategory"`
}

// GetStatuses returns all issue statuses.
//
// API: GET /rest/api/2/status
func (c *Client) GetStatuses(ctx context.Context) ([]Status, error) {
	var statuses []Status
	return statuses, c.get(ctx, apiBase+"/status", &statuses)
}

// -----------------------------------------------------------------------
// Fields – /rest/api/2/field
// -----------------------------------------------------------------------

// Field represents a Jira issue field definition.
type Field struct {
	ID         string `json:"id"`
	Key        string `json:"key"`
	Name       string `json:"name"`
	Custom     bool   `json:"custom"`
	Orderable  bool   `json:"orderable"`
	Navigable  bool   `json:"navigable"`
	Searchable bool   `json:"searchable"`
}

// GetFields returns all field definitions.
//
// API: GET /rest/api/2/field
func (c *Client) GetFields(ctx context.Context) ([]Field, error) {
	var fields []Field
	return fields, c.get(ctx, apiBase+"/field", &fields)
}

// FieldSearchResult represents an individual field returned by the paginated
// field search endpoint, which includes additional metadata not available from
// the simple GET /field endpoint.
type FieldSearchResult struct {
	ID            string          `json:"id"`
	Key           string          `json:"key"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Custom        bool            `json:"custom"`
	Searchable    bool            `json:"searchable"`
	Navigable     bool            `json:"navigable"`
	Orderable     bool            `json:"orderable"`
	IsLocked      bool            `json:"isLocked"`
	SearcherKey   string          `json:"searcherKey"`
	ScreensCount  int             `json:"screensCount"`
	ContextsCount int             `json:"contextsCount"`
	ProjectsCount int             `json:"projectsCount"`
	Schema        json.RawMessage `json:"schema"`
}

// FieldSearchPage is the paginated response from GET /rest/api/2/field/search.
type FieldSearchPage struct {
	Total      int                 `json:"total"`
	StartAt    int                 `json:"startAt"`
	MaxResults int                 `json:"maxResults"`
	IsLast     bool                `json:"isLast"`
	Values     []FieldSearchResult `json:"values"`
}

// FieldSearchOptions configures the paginated field search.
type FieldSearchOptions struct {
	// IDs filters to specific field IDs.
	IDs []string
	// Query filters by name/description substring.
	Query string
	// Type filters by field type: "system" or "custom".
	Type string
	// OrderBy orders results: "contextsCount", "lastUsed", "name", "screensCount", "projectsCount".
	OrderBy string
	// Expand includes additional data: "screensCount", "contextsCount", "lastUsed".
	Expand string
	// ProjectIDs filters to fields used in these projects.
	ProjectIDs []int
	StartAt    int
	MaxResults int
}

// SearchFields returns a paginated list of field definitions.
//
// API: GET /rest/api/2/field/search
func (c *Client) SearchFields(ctx context.Context, opts FieldSearchOptions) (*FieldSearchPage, error) {
	maxResults := opts.MaxResults
	if maxResults == 0 {
		maxResults = 50
	}
	params := url.Values{}
	params.Set("startAt", fmt.Sprintf("%d", opts.StartAt))
	params.Set("maxResults", fmt.Sprintf("%d", maxResults))
	for _, id := range opts.IDs {
		params.Add("id", id)
	}
	if opts.Query != "" {
		params.Set("query", opts.Query)
	}
	if opts.Type != "" {
		params.Set("type", opts.Type)
	}
	if opts.OrderBy != "" {
		params.Set("orderBy", opts.OrderBy)
	}
	if opts.Expand != "" {
		params.Set("expand", opts.Expand)
	}
	for _, pid := range opts.ProjectIDs {
		params.Add("projectIds", fmt.Sprintf("%d", pid))
	}
	var page FieldSearchPage
	return &page, c.get(ctx, buildQuery(apiBase+"/field/search", params), &page)
}

// FieldContext represents a custom field context.
type FieldContext struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	IsGlobalContext bool   `json:"isGlobalContext"`
	IsAnyIssueType  bool   `json:"isAnyIssueType"`
}

// FieldContextPage is the paginated response from GET /rest/api/2/field/{fieldId}/context.
type FieldContextPage struct {
	Total      int            `json:"total"`
	StartAt    int            `json:"startAt"`
	MaxResults int            `json:"maxResults"`
	IsLast     bool           `json:"isLast"`
	Values     []FieldContext `json:"values"`
}

// FieldContextOptions configures the context list request.
type FieldContextOptions struct {
	// IsAnyIssueType filters to contexts that apply to all issue types (true) or a subset (false).
	// nil means no filter.
	IsAnyIssueType *bool
	// IsGlobalContext filters to global contexts (true) or project-scoped (false).
	// nil means no filter.
	IsGlobalContext *bool
	// ContextIDs filters to specific context IDs.
	ContextIDs []int
	StartAt    int
	MaxResults int
}

// GetFieldContexts returns the contexts for a custom field.
//
// API: GET /rest/api/2/field/{fieldId}/context
func (c *Client) GetFieldContexts(ctx context.Context, fieldID string, opts FieldContextOptions) (*FieldContextPage, error) {
	maxResults := opts.MaxResults
	if maxResults == 0 {
		maxResults = 50
	}
	params := url.Values{}
	params.Set("startAt", fmt.Sprintf("%d", opts.StartAt))
	params.Set("maxResults", fmt.Sprintf("%d", maxResults))
	if opts.IsAnyIssueType != nil {
		params.Set("isAnyIssueType", fmt.Sprintf("%v", *opts.IsAnyIssueType))
	}
	if opts.IsGlobalContext != nil {
		params.Set("isGlobalContext", fmt.Sprintf("%v", *opts.IsGlobalContext))
	}
	for _, cid := range opts.ContextIDs {
		params.Add("contextId", fmt.Sprintf("%d", cid))
	}
	path := fmt.Sprintf("%s/field/%s/context", apiBase, fieldID)
	var page FieldContextPage
	return &page, c.get(ctx, buildQuery(path, params), &page)
}

// FieldOption represents an option (allowed value) for a custom field.
type FieldOption struct {
	ID        string `json:"id"`
	Value     string `json:"value"`
	Disabled  bool   `json:"disabled"`
	ContextID string `json:"contextId,omitempty"`
}

// FieldOptionPage is the paginated response from the field options endpoint.
type FieldOptionPage struct {
	Total      int           `json:"total"`
	StartAt    int           `json:"startAt"`
	MaxResults int           `json:"maxResults"`
	IsLast     bool          `json:"isLast"`
	Values     []FieldOption `json:"values"`
}

// GetFieldOptions returns the options (allowed values) for a custom field context.
//
// API: GET /rest/api/2/field/{fieldId}/context/option
func (c *Client) GetFieldOptions(ctx context.Context, fieldID string, contextID string, onlyOptions bool, startAt, maxResults int) (*FieldOptionPage, error) {
	if maxResults == 0 {
		maxResults = 100
	}
	params := url.Values{}
	params.Set("startAt", fmt.Sprintf("%d", startAt))
	params.Set("maxResults", fmt.Sprintf("%d", maxResults))
	if contextID != "" {
		params.Add("contextId", contextID)
	}
	if onlyOptions {
		params.Set("onlyOptions", "true")
	}
	path := fmt.Sprintf("%s/field/%s/context/option", apiBase, fieldID)
	var page FieldOptionPage
	return &page, c.get(ctx, buildQuery(path, params), &page)
}

// -----------------------------------------------------------------------
// Edit metadata – /rest/api/2/issue/{issueKey}/editmeta
// -----------------------------------------------------------------------

// EditMetaAllowedValue is one entry in the allowedValues list for a field.
// Both select-style fields (value) and object-style fields (name, id) are
// covered by this struct.
type EditMetaAllowedValue struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// EditMetaField describes a single field as returned by the editmeta endpoint.
type EditMetaField struct {
	Required      bool                   `json:"required"`
	Schema        map[string]interface{} `json:"schema"`
	Name          string                 `json:"name"`
	AllowedValues []EditMetaAllowedValue `json:"allowedValues"`
}

// EditMeta is the response from GET /rest/api/2/issue/{issueKey}/editmeta.
type EditMeta struct {
	Fields map[string]EditMetaField `json:"fields"`
}

// GetEditMeta returns the edit metadata for an existing issue, including the
// allowed values for each editable field.
//
// API: GET /rest/api/2/issue/{issueKey}/editmeta
// Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issues/#api-rest-api-2-issue-issueidorkey-editmeta-get
func (c *Client) GetEditMeta(ctx context.Context, issueKey string) (*EditMeta, error) {
	path := fmt.Sprintf("%s/issue/%s/editmeta", apiBase, issueKey)
	var meta EditMeta
	return &meta, c.get(ctx, path, &meta)
}

// -----------------------------------------------------------------------
// Resolutions – /rest/api/2/resolution
// -----------------------------------------------------------------------

// Resolution represents a Jira issue resolution.
type Resolution struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// GetResolutions returns all resolutions defined on the Jira instance.
//
// API: GET /rest/api/2/resolution
// Works on both Jira Cloud and Jira Server / Data Center.
func (c *Client) GetResolutions(ctx context.Context) ([]Resolution, error) {
	var resolutions []Resolution
	return resolutions, c.get(ctx, apiBase+"/resolution", &resolutions)
}

// -----------------------------------------------------------------------
// Server info – /rest/api/2/serverInfo
// -----------------------------------------------------------------------

// ServerInfo contains build and runtime information about the Jira instance.
type ServerInfo struct {
	BaseURL        string `json:"baseUrl"`
	Version        string `json:"version"`
	DeploymentType string `json:"deploymentType"`
	BuildNumber    int    `json:"buildNumber"`
	BuildDate      string `json:"buildDate"`
	ServerTime     string `json:"serverTime"`
	ScmInfo        string `json:"scmInfo"`
	ServerTitle    string `json:"serverTitle"`
}

// GetServerInfo returns build and runtime information about the Jira instance.
//
// API: GET /rest/api/2/serverInfo
// Works on both Jira Cloud and Jira Server / Data Center.
func (c *Client) GetServerInfo(ctx context.Context) (*ServerInfo, error) {
	var info ServerInfo
	return &info, c.get(ctx, apiBase+"/serverInfo", &info)
}

// -----------------------------------------------------------------------
// Project statuses – /rest/api/2/project/{key}/statuses
// -----------------------------------------------------------------------

// ProjectIssueTypeStatuses groups the workflow statuses available for one
// issue type within a project.
type ProjectIssueTypeStatuses struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Subtask  bool     `json:"subtask"`
	Statuses []Status `json:"statuses"`
}

// GetProjectStatuses returns the statuses available for each issue type in
// the given project.
//
// API: GET /rest/api/2/project/{projectIdOrKey}/statuses
// Works on both Jira Cloud and Jira Server / Data Center.
func (c *Client) GetProjectStatuses(ctx context.Context, projectKey string) ([]ProjectIssueTypeStatuses, error) {
	path := fmt.Sprintf("%s/project/%s/statuses", apiBase, projectKey)
	var result []ProjectIssueTypeStatuses
	return result, c.get(ctx, path, &result)
}

// -----------------------------------------------------------------------
// Instance configuration – /rest/api/2/configuration
// -----------------------------------------------------------------------

// TimeTrackingConfiguration holds time-tracking settings.
type TimeTrackingConfiguration struct {
	WorkingHoursPerDay float64 `json:"workingHoursPerDay"`
	WorkingDaysPerWeek float64 `json:"workingDaysPerWeek"`
	TimeFormat         string  `json:"timeFormat"`
	DefaultUnit        string  `json:"defaultUnit"`
}

// Configuration holds instance-level feature flags and settings.
type Configuration struct {
	VotingEnabled           bool                      `json:"votingEnabled"`
	WatchingEnabled         bool                      `json:"watchingEnabled"`
	UnassignedIssuesAllowed bool                      `json:"unassignedIssuesAllowed"`
	SubTasksEnabled         bool                      `json:"subTasksEnabled"`
	IssueLinkingEnabled     bool                      `json:"issueLinkingEnabled"`
	TimeTrackingEnabled     bool                      `json:"timeTrackingEnabled"`
	AttachmentsEnabled      bool                      `json:"attachmentsEnabled"`
	TimeTracking            TimeTrackingConfiguration `json:"timeTrackingConfiguration"`
}

// GetConfiguration returns the instance-level configuration flags.
//
// API: GET /rest/api/2/configuration
// Works on both Jira Cloud and Jira Server / Data Center.
func (c *Client) GetConfiguration(ctx context.Context) (*Configuration, error) {
	var cfg Configuration
	return &cfg, c.get(ctx, apiBase+"/configuration", &cfg)
}

// -----------------------------------------------------------------------
// Boards – /rest/agile/1.0/board
// -----------------------------------------------------------------------

// Board represents a Jira Agile board.
type Board struct {
	ID       int            `json:"id"`
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Self     string         `json:"self"`
	Location *BoardLocation `json:"location"`
}

// BoardLocation describes where a board is scoped.
type BoardLocation struct {
	ProjectID   int    `json:"projectId"`
	ProjectKey  string `json:"projectKey"`
	ProjectName string `json:"projectName"`
}

// BoardList is a paginated list of boards.
type BoardList struct {
	MaxResults int     `json:"maxResults"`
	StartAt    int     `json:"startAt"`
	Total      int     `json:"total"`
	IsLast     bool    `json:"isLast"`
	Values     []Board `json:"values"`
}

// GetBoards returns all boards visible to the current user.
//
// API: GET /rest/agile/1.0/board
func (c *Client) GetBoards(ctx context.Context, projectKey string, maxResults int) (*BoardList, error) {
	if maxResults == 0 {
		maxResults = 50
	}
	params := url.Values{}
	params.Set("maxResults", fmt.Sprintf("%d", maxResults))
	if projectKey != "" {
		params.Set("projectKeyOrId", projectKey)
	}
	var list BoardList
	return &list, c.get(ctx, buildQuery(agileBase+"/board", params), &list)
}

// -----------------------------------------------------------------------
// Sprints – /rest/agile/1.0/board/{boardId}/sprint
// -----------------------------------------------------------------------

// Sprint represents a Jira Agile sprint.
type Sprint struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	State         string `json:"state"`
	StartDate     string `json:"startDate"`
	EndDate       string `json:"endDate"`
	CompleteDate  string `json:"completeDate"`
	Goal          string `json:"goal"`
	OriginBoardID int    `json:"originBoardId"`
}

// SprintList is a paginated list of sprints.
type SprintList struct {
	MaxResults int      `json:"maxResults"`
	StartAt    int      `json:"startAt"`
	Total      int      `json:"total"`
	IsLast     bool     `json:"isLast"`
	Values     []Sprint `json:"values"`
}

// GetSprints returns all sprints for a board.
//
// API: GET /rest/agile/1.0/board/{boardId}/sprint
func (c *Client) GetSprints(ctx context.Context, boardID int, state string) (*SprintList, error) {
	params := url.Values{}
	if state != "" {
		params.Set("state", state)
	}
	var list SprintList
	path := buildQuery(fmt.Sprintf("%s/board/%d/sprint", agileBase, boardID), params)
	return &list, c.get(ctx, path, &list)
}

// GetSprintIssues returns all issues in a sprint.
//
// API: GET /rest/agile/1.0/sprint/{sprintId}/issue
func (c *Client) GetSprintIssues(ctx context.Context, sprintID int) (*SearchResult, error) {
	var result SearchResult
	return &result, c.get(ctx, fmt.Sprintf("%s/sprint/%d/issue", agileBase, sprintID), &result)
}

// CreateSprintRequest is the payload for creating a sprint.
type CreateSprintRequest struct {
	Name          string `json:"name"`
	Goal          string `json:"goal,omitempty"`
	StartDate     string `json:"startDate,omitempty"`
	EndDate       string `json:"endDate,omitempty"`
	OriginBoardID int    `json:"originBoardId"`
}

// CreateSprint creates a new sprint on a board.
//
// API: POST /rest/agile/1.0/sprint
func (c *Client) CreateSprint(ctx context.Context, req *CreateSprintRequest) (*Sprint, error) {
	var sprint Sprint
	return &sprint, c.post(ctx, agileBase+"/sprint", req, &sprint)
}

// UpdateSprint updates a sprint.
//
// API: PUT /rest/agile/1.0/sprint/{sprintId}
func (c *Client) UpdateSprint(ctx context.Context, sprintID int, fields map[string]interface{}) (*Sprint, error) {
	var sprint Sprint
	return &sprint, c.put(ctx, fmt.Sprintf("%s/sprint/%d", agileBase, sprintID), fields, &sprint)
}
