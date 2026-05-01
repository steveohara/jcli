// Package client provides an HTTP client for the Jira REST API v2 and the
// Jira Agile REST API (boards/sprints).
//
// Authentication is performed using a Bearer token (Jira Cloud personal access
// token or Jira Server/Data Center personal access token).
//
// The client automatically prefixes all requests with the configured server
// URL and the appropriate API base path:
//
//   - /rest/api/2  – core Jira API
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
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.Insecure, // #nosec G402 – user-controlled flag
		},
	}
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout:   defaultTimeout,
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
		return parseAPIError(resp.StatusCode, respBody)
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
	StatusCode   int
	ErrorMessages []string `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
}

func (e *apiError) Error() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("HTTP %d", e.StatusCode))
	parts = append(parts, e.ErrorMessages...)
	for k, v := range e.Errors {
		parts = append(parts, fmt.Sprintf("%s: %s", k, v))
	}
	return strings.Join(parts, "; ")
}

func parseAPIError(statusCode int, body []byte) error {
	e := &apiError{StatusCode: statusCode}
	_ = json.Unmarshal(body, e) // best-effort
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
	Self   string      `json:"self"`
	Fields IssueFields `json:"fields"`
}

// IssueFields contains the field values of a Jira issue.
type IssueFields struct {
	Summary     string       `json:"summary"`
	Description string       `json:"description"`
	Status      NamedObj     `json:"status"`
	Priority    NamedObj     `json:"priority"`
	Assignee    *User        `json:"assignee"`
	Reporter    *User        `json:"reporter"`
	IssueType   NamedObj     `json:"issuetype"`
	Project     ProjectShort `json:"project"`
	Labels      []string     `json:"labels"`
	Components  []NamedObj   `json:"components"`
	FixVersions []NamedObj   `json:"fixVersions"`
	Created     string       `json:"created"`
	Updated     string       `json:"updated"`
	DueDate     string       `json:"duedate"`
	Votes       *Votes       `json:"votes"`
	Watches     *Watches     `json:"watches"`
	Comment     *CommentList `json:"comment"`
	TimeTracking *TimeTracking `json:"timetracking"`
	Parent      *IssueRef    `json:"parent"`
}

// IssueRef is a lightweight reference to another issue (e.g. parent).
type IssueRef struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

// NamedObj is a simple name-only sub-object used throughout the Jira API.
type NamedObj struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	Self string `json:"self,omitempty"`
}

// ProjectShort is a minimal project representation.
type ProjectShort struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
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
	Project     IDObj    `json:"project"`
	Summary     string   `json:"summary"`
	Description string   `json:"description,omitempty"`
	IssueType   NamedObj `json:"issuetype"`
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
// API: POST /rest/api/2/search
func (c *Client) SearchIssues(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	maxResults := opts.MaxResults
	if maxResults == 0 {
		maxResults = 50
	}
	payload := map[string]interface{}{
		"jql":        opts.JQL,
		"startAt":    opts.StartAt,
		"maxResults": maxResults,
	}
	if len(opts.Fields) > 0 {
		payload["fields"] = opts.Fields
	}
	var result SearchResult
	return &result, c.post(ctx, apiBase+"/search", payload, &result)
}

// -----------------------------------------------------------------------
// Issue Comments – /rest/api/2/issue/{issueIdOrKey}/comment
// -----------------------------------------------------------------------

// Comment represents a Jira issue comment.
type Comment struct {
	ID      string `json:"id"`
	Self    string `json:"self"`
	Author  *User  `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created"`
	Updated string `json:"updated"`
}

// CommentList is the paginated list of comments returned by the API.
type CommentList struct {
	Total    int       `json:"total"`
	StartAt  int       `json:"startAt"`
	MaxResults int     `json:"maxResults"`
	Comments []Comment `json:"comments"`
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
	payload := map[string]string{"body": body}
	var comment Comment
	return &comment, c.post(ctx, fmt.Sprintf("%s/issue/%s/comment", apiBase, issueKey), payload, &comment)
}

// UpdateComment updates an existing comment.
//
// API: PUT /rest/api/2/issue/{issueIdOrKey}/comment/{id}
func (c *Client) UpdateComment(ctx context.Context, issueKey, commentID, body string) (*Comment, error) {
	payload := map[string]string{"body": body}
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
	ID               string `json:"id"`
	Self             string `json:"self"`
	Author           *User  `json:"author"`
	Comment          string `json:"comment"`
	Started          string `json:"started"`
	TimeSpent        string `json:"timeSpent"`
	TimeSpentSeconds int    `json:"timeSpentSeconds"`
}

// WorklogList is a paginated list of worklogs.
type WorklogList struct {
	Total     int       `json:"total"`
	StartAt   int       `json:"startAt"`
	MaxResults int      `json:"maxResults"`
	Worklogs  []Worklog `json:"worklogs"`
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
	payload := map[string]string{
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
		return nil, parseAPIError(resp.StatusCode, respBody)
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
		payload["comment"] = map[string]string{"body": comment}
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
	AccountID    string `json:"accountId"`
	Name         string `json:"name"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Active       bool   `json:"active"`
	Self         string `json:"self"`
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

// Status represents a Jira issue status.
type Status struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    NamedObj `json:"statusCategory"`
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
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Custom      bool   `json:"custom"`
	Orderable   bool   `json:"orderable"`
	Navigable   bool   `json:"navigable"`
	Searchable  bool   `json:"searchable"`
}

// GetFields returns all field definitions.
//
// API: GET /rest/api/2/field
func (c *Client) GetFields(ctx context.Context) ([]Field, error) {
	var fields []Field
	return fields, c.get(ctx, apiBase+"/field", &fields)
}

// -----------------------------------------------------------------------
// Boards – /rest/agile/1.0/board
// -----------------------------------------------------------------------

// Board represents a Jira Agile board.
type Board struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Self     string `json:"self"`
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
