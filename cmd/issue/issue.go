// Package issue implements all issue-related sub-commands for jcli.
package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveohara/jcli/cmd"
	"github.com/steveohara/jcli/internal/client"
	"github.com/steveohara/jcli/internal/output"
)

// IssueCmd is the parent command for all issue operations.
var IssueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Manage Jira issues",
	Long: `Commands for creating, reading, updating and deleting Jira issues,
as well as managing comments, attachments, worklogs, transitions, votes and
watchers.

API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issues/`,
}

// init registers all issue sub-commands on IssueCmd.
func init() {
	IssueCmd.AddCommand(
		getCmd,
		createCmd,
		updateCmd,
		deleteCmd,
		searchCmd,
		commentCmd,
		transitionCmd,
		assignCmd,
		worklogCmd,
		voteCmd,
		watchCmd,
		linkCmd,
		attachCmd,
	)
}

// -----------------------------------------------------------------------
// issue get

// issueKVRow maps a Jira field ID to a KV table label and value extractor.
// It is used to build the key/value display for "issue get" output.
type issueKVRow struct {
	field   string
	label   string
	extract func(*client.Issue) string
}

// allIssueKVRows defines the full ordered set of rows for `issue get` table output.
// Key and ID are always shown; the remaining rows are filtered by --fields when set.
var allIssueKVRows = []issueKVRow{
	{"summary", "Summary", func(i *client.Issue) string { return i.Fields.Summary }},
	{"issuetype", "Type", func(i *client.Issue) string { return i.Fields.IssueType.Name }},
	{"status", "Status", func(i *client.Issue) string { return i.Fields.Status.Name }},
	{"priority", "Priority", func(i *client.Issue) string { return i.Fields.Priority.Name }},
	{"assignee", "Assignee", func(i *client.Issue) string {
		if i.Fields.Assignee != nil {
			return i.Fields.Assignee.DisplayName
		}
		return ""
	}},
	{"reporter", "Reporter", func(i *client.Issue) string {
		if i.Fields.Reporter != nil {
			return i.Fields.Reporter.DisplayName
		}
		return ""
	}},
	{"project", "Project", func(i *client.Issue) string { return i.Fields.Project.Key }},
	{"created", "Created", func(i *client.Issue) string { return i.Fields.Created }},
	{"updated", "Updated", func(i *client.Issue) string { return i.Fields.Updated }},
	{"duedate", "Due Date", func(i *client.Issue) string { return i.Fields.DueDate }},
	{"labels", "Labels", func(i *client.Issue) string { return strings.Join(i.Fields.Labels, ", ") }},
	{"description", "Description", func(i *client.Issue) string {
		return output.Truncate(string(i.Fields.Description), 200)
	}},
}

// issueColumn maps a Jira field ID to a search results table column header and extractor.
// It is used to build the column set for "issue search" table output.
type issueColumn struct {
	field   string
	header  string
	extract func(client.Issue) string
}

// defaultSearchColumns defines the full ordered set of columns for `issue search` table output.
var defaultSearchColumns = []issueColumn{
	{"issuetype", "TYPE", func(i client.Issue) string { return i.Fields.IssueType.Name }},
	{"priority", "PRIORITY", func(i client.Issue) string { return i.Fields.Priority.Name }},
	{"status", "STATUS", func(i client.Issue) string { return i.Fields.Status.Name }},
	{"assignee", "ASSIGNEE", func(i client.Issue) string {
		if i.Fields.Assignee != nil {
			return i.Fields.Assignee.DisplayName
		}
		return ""
	}},
	{"summary", "SUMMARY", func(i client.Issue) string { return output.Truncate(i.Fields.Summary, 60) }},
}

// -----------------------------------------------------------------------

// getFields and getAllFields are the package-level flag variables for "issue get".
// getFields holds a comma-separated list of field IDs to display.
// getAllFields, when true, bypasses omitempty and outputs every field from the raw API response.
var (
	getFields    []string
	getAllFields bool
)

var getCmd = &cobra.Command{
	Use:   "get <issue-key>",
	Short: "Get details of an issue",
	Long: `Retrieve a single Jira issue by its key or numeric ID and display its details.

API: GET /rest/api/2/issue/{issueIdOrKey}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issues/#api-rest-api-2-issue-issueidorkey-get

Examples:
  jcli issue get PROJ-42
  jcli issue get PROJ-42 --fields summary,status,assignee --output json`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		issue, err := cl.GetIssue(context.Background(), args[0], getFields)
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			if getAllFields {
				// Output the raw API response so no fields are suppressed by omitempty.
				return p.JSON(issue.Raw)
			}
			return p.JSON(issue)
		}

		// Determine which KV rows to show. Key and ID are always included.
		// When --fields is set only show rows whose field ID was requested.
		rows := allIssueKVRows
		if len(getFields) > 0 {
			fieldSet := make(map[string]bool, len(getFields))
			for _, f := range getFields {
				fieldSet[strings.ToLower(f)] = true
			}

			// Filter to known rows that were requested.
			rows = nil
			for _, row := range allIssueKVRows {
				if fieldSet[row.field] {
					rows = append(rows, row)
				}
			}

			// Append rows for any custom (unrecognised) field IDs.
			foundFields := make(map[string]bool, len(rows))
			for _, row := range rows {
				foundFields[row.field] = true
			}
			for _, f := range getFields {
				fLower := strings.ToLower(f)
				if !foundFields[fLower] {
					fieldID := fLower // new var per iteration for closure
					rows = append(rows, issueKVRow{
						field: fieldID,
						label: fieldID,
						extract: func(i *client.Issue) string {
							if raw, ok := i.Fields.Extra[fieldID]; ok {
								return client.FormatCustomField(raw)
							}
							return ""
						},
					})
				}
			}
		}

		kvPairs := [][]string{
			{"Key", issue.Key},
			{"ID", issue.ID},
		}
		for _, row := range rows {
			kvPairs = append(kvPairs, []string{row.label, row.extract(issue)})
		}
		p.KV(kvPairs)
		return nil
	},
}

// init registers flags for getCmd.
func init() {
	getCmd.Flags().StringSliceVar(&getFields, "fields", nil,
		"Comma-separated list of field IDs to include in the response.\n"+
			"By default all fields are returned. Example: --fields summary,status,assignee")
	getCmd.Flags().BoolVar(&getAllFields, "all-fields", false,
		"Include fields with empty or null values in JSON output (default omits them)")
}

// -----------------------------------------------------------------------
// issue create
// -----------------------------------------------------------------------

// createSummary through createHistory are the flag variables for "issue create".
var (
	createSummary     string
	createDescription string
	createType        string
	createPriority    string
	createAssignee    string
	createLabels      []string
	createComponents  []string
	createFixVersions []string
	createDueDate     string
	createParent      string
	createProject     string
	createFields      string
	createProperties  string
	createHistory     string
)

// fieldsHelp is the shared help text for the --fields flag used on both
// issue create and issue update.
const fieldsHelp = `JSON object of field IDs to values sent in the "fields" object of the
request body. Accepts any field the Jira REST API recognises, including
system fields and custom fields (customfield_XXXXX). Values must match
the shape the API expects for that field type:

  Text / string field:
    {"summary": "New title"}

  Named-object field (priority, issuetype, resolution, status):
    {"priority": {"name": "High"}}

  ID-object field (assignee on Cloud, components, fixVersions):
    {"assignee": {"accountId": "5f0d3aef12345678"}}
    {"components": [{"id": "10001"}]}

  Select / radio custom field (use option ID from field-allowed-values):
    {"customfield_31004": {"id": "50628"}}

  Multi-select custom field:
    {"customfield_10030": [{"id": "10100"}, {"id": "10101"}]}

  Number custom field:
    {"customfield_10014": 5}

  Date field:
    {"duedate": "2024-06-01"}

Use 'jcli meta fields' to list all field IDs and types.
Use 'jcli meta field-allowed-values <fieldId> --issue <KEY>' for option IDs.
Values in --fields override the equivalent named flags (e.g. --priority).`

// propertiesHelp is the shared help text for the --properties flag.
const propertiesHelp = `JSON array of entity property objects to attach to the issue.
Each element must have a "key" string and a "value" (any JSON value):
  [{"key": "myapp.context", "value": {"buildNumber": 42}}]
Properties are indexed and searchable via the Jira REST API but are not
visible in the Jira UI by default.`

// historyHelp is the help text for the --history flag.
const historyHelp = `JSON object written to the issue change history to record context about
who or what triggered the change. Useful when making changes on behalf of
a user or an automated system. Common fields:

  activityDescription  Human-readable description of the change activity.
  actor                Object identifying who made the change:
                         {"id": "...", "displayName": "CI Pipeline", "type": "automation"}
  cause                Object describing what caused the change:
                         {"id": "deploy-123", "type": "deployment"}
  description          Long-form description stored in the history entry.
  descriptionKey       i18n key for a localised description string.
  emailDescription     Description included in notification emails.
  extraData            Map of additional key-value pairs to store:
                         {"environment": "production", "version": "2.1.0"}
  generator            Object describing the system that made the change:
                         {"id": "jcli", "type": "cli"}
  type                 String type tag for the history entry (e.g. "madeAutomatically").

Example:
  --history '{"activityDescription":"Automated promotion","actor":{"id":"ci-bot","type":"automation"}}'`

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new issue",
	Long: `Create a new Jira issue in the specified (or default) project.

API: POST /rest/api/2/issue
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issues/#api-rest-api-2-issue-post

The convenience flags (--summary, --type, --priority, etc.) cover the most
common fields. For anything else use --fields, which accepts a JSON object
and is merged with the convenience flags. --fields values override the
convenience flags when both name the same field.

Examples:
  jcli issue create --summary "Fix login page" --type Bug
  jcli issue create --summary "New feature" --type Story --priority High \
      --description "As a user I want to..." --project MYPROJ
  jcli issue create --summary "Subtask" --type Sub-task --parent PROJ-10

  # Set custom fields alongside the convenience flags
  jcli issue create --summary "Voice outage" --type Bug --priority High \
      --fields '{"customfield_31004":{"id":"50628"},"customfield_23824":{"id":"36274"}}'

  # Override summary and set a custom field entirely through --fields
  jcli issue create --summary "placeholder" \
      --fields '{"summary":"Real title","customfield_10014":5}'

  # Attach an entity property
  jcli issue create --summary "Deploy task" \
      --properties '[{"key":"pipeline.id","value":"build-42"}]'`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		projectKey := firstNonEmpty(createProject, cfg.DefaultProject)
		if projectKey == "" {
			return fmt.Errorf("project key is required: use --project or set a default in .jcli.properties")
		}
		req := &client.CreateIssueRequest{
			Fields: client.CreateIssueFields{
				Project:     client.IDObj{Key: projectKey},
				Summary:     createSummary,
				Description: createDescription,
				IssueType:   client.NamedObj{Name: createType},
			},
		}
		if createPriority != "" {
			req.Fields.Priority = &client.NamedObj{Name: createPriority}
		}
		if createAssignee != "" {
			req.Fields.Assignee = &client.IDObj{ID: createAssignee}
		}
		req.Fields.Labels = createLabels
		for _, comp := range createComponents {
			req.Fields.Components = append(req.Fields.Components, client.IDObj{ID: comp})
		}
		for _, v := range createFixVersions {
			req.Fields.FixVersions = append(req.Fields.FixVersions, client.IDObj{ID: v})
		}
		req.Fields.DueDate = createDueDate
		if createParent != "" {
			req.Fields.Parent = &client.IDObj{Key: createParent}
		}
		if createFields != "" {
			var ef map[string]interface{}
			if err := json.Unmarshal([]byte(createFields), &ef); err != nil {
				return fmt.Errorf("invalid --fields JSON: %w", err)
			}
			req.ExtraFields = ef
		}
		if createProperties != "" {
			var props []map[string]interface{}
			if err := json.Unmarshal([]byte(createProperties), &props); err != nil {
				return fmt.Errorf("invalid --properties JSON: %w", err)
			}
			req.Properties = props
		}
		if createHistory != "" {
			var hm map[string]interface{}
			if err := json.Unmarshal([]byte(createHistory), &hm); err != nil {
				return fmt.Errorf("invalid --history JSON: %w", err)
			}
			req.HistoryMetadata = hm
		}
		resp, err := cl.CreateIssue(context.Background(), req)
		if err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Created issue %s", resp.Key))
		fmt.Println(resp.Key)
		return nil
	},
}

// init registers flags for createCmd.
func init() {
	createCmd.Flags().StringVarP(&createSummary, "summary", "s", "", "Issue summary / title (required)")
	_ = createCmd.MarkFlagRequired("summary")
	createCmd.Flags().StringVarP(&createDescription, "description", "d", "", "Issue description body")
	createCmd.Flags().StringVarP(&createType, "type", "t", "Task", "Issue type name, e.g. Bug, Story, Task, Sub-task")
	createCmd.Flags().StringVar(&createPriority, "priority", "", "Priority name, e.g. High, Medium, Low")
	createCmd.Flags().StringVar(&createAssignee, "assignee", "", "Assignee account ID")
	createCmd.Flags().StringSliceVar(&createLabels, "labels", nil, "Comma-separated list of labels")
	createCmd.Flags().StringSliceVar(&createComponents, "components", nil, "Comma-separated list of component IDs")
	createCmd.Flags().StringSliceVar(&createFixVersions, "fix-versions", nil, "Comma-separated list of fix version IDs")
	createCmd.Flags().StringVar(&createDueDate, "due-date", "", "Due date in YYYY-MM-DD format")
	createCmd.Flags().StringVar(&createParent, "parent", "", "Parent issue key for sub-tasks (Server/DC only; for Epic Link use --fields '{\"customfield_10200\":\"EPIC-1\"}')")
	createCmd.Flags().StringVar(&createProject, "project", "", "Project key (overrides default)")
	createCmd.Flags().StringVar(&createFields, "fields", "", fieldsHelp)
	createCmd.Flags().StringVar(&createProperties, "properties", "", propertiesHelp)
	createCmd.Flags().StringVar(&createHistory, "history", "", historyHelp)
}

// -----------------------------------------------------------------------
// issue update
// -----------------------------------------------------------------------

// updateSummary through updateHistory are the flag variables for "issue update".
var (
	updateSummary     string
	updateDescription string
	updatePriority    string
	updateAssignee    string
	updateDueDate     string
	updateLabels      []string
	updateFields      string
	updateProperties  string
	updateHistory     string
)

var updateCmd = &cobra.Command{
	Use:   "update <issue-key>",
	Short: "Update an existing issue",
	Long: `Update one or more fields of an existing Jira issue.

Only the flags you provide are sent to the API; all other fields are left
unchanged.

API: PUT /rest/api/2/issue/{issueIdOrKey}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issues/#api-rest-api-2-issue-issueidorkey-put

The convenience flags (--summary, --priority, etc.) cover the most common
fields. For anything else use --fields, which accepts a JSON object that is
merged with the convenience flags. --fields values override the convenience
flags when both name the same field.

Examples:
  jcli issue update PROJ-42 --summary "Updated title"
  jcli issue update PROJ-42 --priority High --assignee "<account-id>"

  # Set a custom field
  jcli issue update PROJ-42 --fields '{"customfield_31004":{"id":"50628"}}'

  # Mix convenience flags and --fields
  jcli issue update PROJ-42 --priority High \
      --fields '{"customfield_23824":{"id":"36274"}}'

  # Attach an entity property
  jcli issue update PROJ-42 --properties '[{"key":"pipeline.id","value":"build-99"}]'

  # Record change history context
  jcli issue update PROJ-42 --summary "Auto-resolved" \
      --history '{"activityDescription":"Resolved by CI","actor":{"id":"ci-bot","type":"automation"}}'`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		fields := make(map[string]interface{})
		if updateSummary != "" {
			fields["summary"] = updateSummary
		}
		if updateDescription != "" {
			fields["description"] = updateDescription
		}
		if updatePriority != "" {
			fields["priority"] = map[string]string{"name": updatePriority}
		}
		if updateAssignee != "" {
			fields["assignee"] = map[string]string{"accountId": updateAssignee}
		}
		if updateDueDate != "" {
			fields["duedate"] = updateDueDate
		}
		if len(updateLabels) > 0 {
			fields["labels"] = updateLabels
		}
		if updateFields != "" {
			var ef map[string]interface{}
			if err := json.Unmarshal([]byte(updateFields), &ef); err != nil {
				return fmt.Errorf("invalid --fields JSON: %w", err)
			}
			for k, v := range ef {
				fields[k] = v
			}
		}
		req := &client.UpdateIssueRequest{Fields: fields}
		if updateProperties != "" {
			var props []map[string]interface{}
			if err := json.Unmarshal([]byte(updateProperties), &props); err != nil {
				return fmt.Errorf("invalid --properties JSON: %w", err)
			}
			req.Properties = props
		}
		if updateHistory != "" {
			var hm map[string]interface{}
			if err := json.Unmarshal([]byte(updateHistory), &hm); err != nil {
				return fmt.Errorf("invalid --history JSON: %w", err)
			}
			req.HistoryMetadata = hm
		}
		if len(req.Fields) == 0 && len(req.Properties) == 0 && len(req.HistoryMetadata) == 0 {
			return fmt.Errorf("no update values specified; use --summary, --fields, --properties, or --history")
		}
		if err := cl.UpdateIssue(context.Background(), args[0], req); err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Updated issue %s", args[0]))
		return nil
	},
}

// init registers flags for updateCmd.
func init() {
	updateCmd.Flags().StringVarP(&updateSummary, "summary", "s", "", "New summary / title")
	updateCmd.Flags().StringVarP(&updateDescription, "description", "d", "", "New description body")
	updateCmd.Flags().StringVar(&updatePriority, "priority", "", "New priority name")
	updateCmd.Flags().StringVar(&updateAssignee, "assignee", "", "New assignee account ID")
	updateCmd.Flags().StringVar(&updateDueDate, "due-date", "", "New due date (YYYY-MM-DD)")
	updateCmd.Flags().StringSliceVar(&updateLabels, "labels", nil, "Replace labels with this comma-separated list")
	updateCmd.Flags().StringVar(&updateFields, "fields", "", fieldsHelp)
	updateCmd.Flags().StringVar(&updateProperties, "properties", "", propertiesHelp)
	updateCmd.Flags().StringVar(&updateHistory, "history", "", historyHelp)
}

// -----------------------------------------------------------------------
// issue delete
// -----------------------------------------------------------------------

// deleteSubtasks controls whether sub-tasks are also deleted when an issue is deleted.
var deleteSubtasks bool

var deleteCmd = &cobra.Command{
	Use:   "delete <issue-key>",
	Short: "Delete an issue",
	Long: `Permanently delete a Jira issue.  This action cannot be undone.

Use --delete-subtasks to also delete all child sub-tasks in one operation.

API: DELETE /rest/api/2/issue/{issueIdOrKey}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issues/#api-rest-api-2-issue-issueidorkey-delete

Examples:
  jcli issue delete PROJ-42
  jcli issue delete PROJ-42 --delete-subtasks`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		if err := cl.DeleteIssue(context.Background(), args[0], deleteSubtasks); err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Deleted issue %s", args[0]))
		return nil
	},
}

// init registers flags for deleteCmd.
func init() {
	deleteCmd.Flags().BoolVar(&deleteSubtasks, "delete-subtasks", false,
		"Also delete all sub-tasks associated with this issue")
}

// -----------------------------------------------------------------------
// issue search
// -----------------------------------------------------------------------

// searchJQL through searchAllFields are the flag variables for "issue search".
// searchPage provides a 1-based convenience alias for --start-at.
// searchAll enables automatic pagination to fetch every matching issue.
// searchAllFields bypasses omitempty and emits the raw API response bytes per issue.
var (
	searchJQL        string
	searchFields     []string
	searchStartAt    int
	searchMaxResults int
	searchPage       int
	searchAll        bool
	searchAllFields  bool
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for issues using JQL",
	Long: `Search for Jira issues using Jira Query Language (JQL).

Results are paginated; use --start-at and --max-results to control the page.

API: GET /rest/api/2/search
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-search/#api-rest-api-2-search-get

JQL documentation: https://support.atlassian.com/jira-service-management-cloud/docs/use-jql-to-filter-issues/

Examples:
  jcli issue search --jql "project = PROJ AND status = 'In Progress'"
  jcli issue search --jql "assignee = currentUser() ORDER BY updated DESC"
  jcli issue search --jql "project = PROJ" --max-results 100 --output json`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		if searchJQL == "" && cfg.DefaultProject != "" {
			searchJQL = fmt.Sprintf("project = %s", cfg.DefaultProject)
		}

		maxResults := searchMaxResults
		if maxResults == 0 {
			maxResults = 50
		}

		// --page overrides --start-at (1-based page number).
		startAt := searchStartAt
		if searchPage > 0 {
			startAt = (searchPage - 1) * maxResults
		}

		opts := client.SearchOptions{
			JQL:        searchJQL,
			Fields:     searchFields,
			StartAt:    startAt,
			MaxResults: maxResults,
		}

		var result *client.SearchResult
		if searchAll {
			// Auto-paginate: collect every issue across all pages.
			result = &client.SearchResult{}
			for {
				page, err := cl.SearchIssues(context.Background(), opts)
				if err != nil {
					return err
				}
				result.Issues = append(result.Issues, page.Issues...)
				result.Total = page.Total
				result.MaxResults = page.MaxResults
				if len(page.Issues) == 0 || len(result.Issues) >= page.Total {
					break
				}
				opts.StartAt += len(page.Issues)
			}
		} else {
			var err error
			result, err = cl.SearchIssues(context.Background(), opts)
			if err != nil {
				return err
			}
		}

		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			if searchAllFields {
				// Rebuild the result using the raw per-issue bytes so that
				// omitempty suppression is bypassed for every issue object.
				rawIssues := make([]json.RawMessage, len(result.Issues))
				for i, iss := range result.Issues {
					rawIssues[i] = iss.Raw
				}
				return p.JSON(map[string]interface{}{
					"total":      result.Total,
					"startAt":    result.StartAt,
					"maxResults": result.MaxResults,
					"issues":     rawIssues,
				})
			}
			return p.JSON(result)
		}

		// Determine which columns to show. KEY is always first.
		// When --fields is set only show columns whose field ID was requested,
		// plus a pass-through column for any unrecognised (custom) field IDs.
		cols := defaultSearchColumns
		if len(searchFields) > 0 {
			fieldSet := make(map[string]bool, len(searchFields))
			for _, f := range searchFields {
				fieldSet[strings.ToLower(f)] = true
			}

			// Filter to known columns that were requested.
			cols = nil
			for _, col := range defaultSearchColumns {
				if fieldSet[col.field] {
					cols = append(cols, col)
				}
			}

			// Append columns for any custom (unrecognised) field IDs.
			foundFields := make(map[string]bool, len(cols))
			for _, col := range cols {
				foundFields[col.field] = true
			}
			for _, f := range searchFields {
				fLower := strings.ToLower(f)
				if !foundFields[fLower] {
					fieldID := fLower // new var per iteration for closure
					cols = append(cols, issueColumn{
						field:  fieldID,
						header: strings.ToUpper(fieldID),
						extract: func(i client.Issue) string {
							if raw, ok := i.Fields.Extra[fieldID]; ok {
								return client.FormatCustomField(raw)
							}
							return ""
						},
					})
				}
			}
		}

		headers := make([]string, 0, len(cols)+1)
		headers = append(headers, "KEY")
		for _, col := range cols {
			headers = append(headers, col.header)
		}

		fmt.Fprintf(output.Stdout(), "Found %d issues (showing %d)\n", result.Total, len(result.Issues))
		var rows [][]string
		for _, issue := range result.Issues {
			row := []string{issue.Key}
			for _, col := range cols {
				row = append(row, col.extract(issue))
			}
			rows = append(rows, row)
		}
		p.Table(headers, rows)
		return nil
	},
}

// init registers flags for searchCmd.
func init() {
	searchCmd.Flags().StringVar(&searchJQL, "jql", "",
		"JQL query string, e.g. \"project = PROJ AND status = Open\"")
	searchCmd.Flags().StringSliceVar(&searchFields, "fields", nil,
		"Comma-separated list of field IDs to include in the response")
	searchCmd.Flags().IntVar(&searchStartAt, "start-at", 0,
		"Index of the first result to return (0-based, for pagination)")
	searchCmd.Flags().IntVar(&searchMaxResults, "max-results", 50,
		"Maximum number of results to return per page")
	searchCmd.Flags().IntVar(&searchPage, "page", 0,
		"Page number to fetch (1-based); overrides --start-at")
	searchCmd.Flags().BoolVar(&searchAll, "all", false,
		"Fetch all pages automatically (overrides --page and --start-at)")
	searchCmd.Flags().BoolVar(&searchAllFields, "all-fields", false,
		"Include fields with empty or null values in JSON output (default omits them)")
}

// -----------------------------------------------------------------------
// issue comment
// -----------------------------------------------------------------------

var commentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Manage issue comments",
	Long: `Commands for listing, adding, updating and deleting comments on a Jira issue.

API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-comments/`,
}

// commentBody holds the text for a new or updated comment.
// commentIDFlag holds the ID of the comment being updated or deleted.
var (
	commentBody   string
	commentIDFlag string
)

var commentListCmd = &cobra.Command{
	Use:   "list <issue-key>",
	Short: "List all comments on an issue",
	Long: `Retrieve all comments for a Jira issue.

API: GET /rest/api/2/issue/{issueIdOrKey}/comment
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-comments/#api-rest-api-2-issue-issueidorkey-comment-get

Example:
  jcli issue comment list PROJ-42`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		list, err := cl.GetComments(context.Background(), args[0])
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(list)
		}
		var rows [][]string
		for _, comment := range list.Comments {
			author := ""
			if comment.Author != nil {
				author = comment.Author.DisplayName
			}
			rows = append(rows, []string{comment.ID, author, comment.Created, output.Truncate(string(comment.Body), 80)})
		}
		p.Table([]string{"ID", "AUTHOR", "CREATED", "BODY"}, rows)
		return nil
	},
}

var commentAddCmd = &cobra.Command{
	Use:   "add <issue-key>",
	Short: "Add a comment to an issue",
	Long: `Add a new comment to a Jira issue.

API: POST /rest/api/2/issue/{issueIdOrKey}/comment
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-comments/#api-rest-api-2-issue-issueidorkey-comment-post

Examples:
  jcli issue comment add PROJ-42 --body "This is a comment"`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		comment, err := cl.AddComment(context.Background(), args[0], commentBody)
		if err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Added comment %s to %s", comment.ID, args[0]))
		return nil
	},
}

var commentUpdateCmd = &cobra.Command{
	Use:   "update <issue-key>",
	Short: "Update an existing comment",
	Long: `Update the body of an existing comment on a Jira issue.

API: PUT /rest/api/2/issue/{issueIdOrKey}/comment/{id}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-comments/#api-rest-api-2-issue-issueidorkey-comment-id-put

Examples:
  jcli issue comment update PROJ-42 --comment-id 10001 --body "Updated text"`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		_, err := cl.UpdateComment(context.Background(), args[0], commentIDFlag, commentBody)
		if err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Updated comment %s on %s", commentIDFlag, args[0]))
		return nil
	},
}

var commentDeleteCmd = &cobra.Command{
	Use:   "delete <issue-key>",
	Short: "Delete a comment",
	Long: `Delete a comment from a Jira issue.

API: DELETE /rest/api/2/issue/{issueIdOrKey}/comment/{id}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-comments/#api-rest-api-2-issue-issueidorkey-comment-id-delete

Examples:
  jcli issue comment delete PROJ-42 --comment-id 10001`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		if err := cl.DeleteComment(context.Background(), args[0], commentIDFlag); err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Deleted comment %s from %s", commentIDFlag, args[0]))
		return nil
	},
}

// init registers flags for comment sub-commands and wires them onto commentCmd.
func init() {
	commentAddCmd.Flags().StringVarP(&commentBody, "body", "b", "", "Comment body text (required)")
	_ = commentAddCmd.MarkFlagRequired("body")

	commentUpdateCmd.Flags().StringVar(&commentIDFlag, "comment-id", "", "ID of the comment to update (required)")
	_ = commentUpdateCmd.MarkFlagRequired("comment-id")
	commentUpdateCmd.Flags().StringVarP(&commentBody, "body", "b", "", "New comment body text (required)")
	_ = commentUpdateCmd.MarkFlagRequired("body")

	commentDeleteCmd.Flags().StringVar(&commentIDFlag, "comment-id", "", "ID of the comment to delete (required)")
	_ = commentDeleteCmd.MarkFlagRequired("comment-id")

	commentCmd.AddCommand(commentListCmd, commentAddCmd, commentUpdateCmd, commentDeleteCmd)
}

// -----------------------------------------------------------------------
// issue transition
// -----------------------------------------------------------------------

var transitionCmd = &cobra.Command{
	Use:   "transition",
	Short: "Manage issue transitions (workflow)",
	Long: `Commands for listing available workflow transitions and moving an issue
to a new status.

API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issues/#api-rest-api-2-issue-issueidorkey-transitions-get`,
}

var transitionListCmd = &cobra.Command{
	Use:   "list <issue-key>",
	Short: "List available transitions for an issue",
	Long: `Show all workflow transitions that can be applied to an issue in its
current status.

API: GET /rest/api/2/issue/{issueIdOrKey}/transitions
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issues/#api-rest-api-2-issue-issueidorkey-transitions-get

Example:
  jcli issue transition list PROJ-42`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		list, err := cl.GetTransitions(context.Background(), args[0])
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(list)
		}
		var rows [][]string
		for _, t := range list.Transitions {
			rows = append(rows, []string{t.ID, t.Name, t.To.Name})
		}
		p.Table([]string{"ID", "NAME", "TO STATUS"}, rows)
		return nil
	},
}

// transitionID is the workflow transition ID to apply.
// transitionResolution is an optional resolution name to set when closing an issue.
var (
	transitionID         string
	transitionResolution string
)

var transitionApplyCmd = &cobra.Command{
	Use:   "apply <issue-key>",
	Short: "Apply a transition to an issue",
	Long: `Move an issue to a new workflow state by applying a transition.

Use "jcli issue transition list <issue-key>" to discover available transition IDs.

API: POST /rest/api/2/issue/{issueIdOrKey}/transitions
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issues/#api-rest-api-2-issue-issueidorkey-transitions-post

Examples:
  jcli issue transition apply PROJ-42 --id 31
  jcli issue transition apply PROJ-42 --id 5 --resolution "Fixed"`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		var fields map[string]interface{}
		if transitionResolution != "" {
			fields = map[string]interface{}{
				"resolution": map[string]string{"name": transitionResolution},
			}
		}
		if err := cl.TransitionIssue(context.Background(), args[0], transitionID, fields); err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Transitioned %s with transition %s", args[0], transitionID))
		return nil
	},
}

// init registers flags for transition sub-commands and wires them onto transitionCmd.
func init() {
	transitionApplyCmd.Flags().StringVar(&transitionID, "id", "", "Transition ID (required; see 'transition list')")
	_ = transitionApplyCmd.MarkFlagRequired("id")
	transitionApplyCmd.Flags().StringVar(&transitionResolution, "resolution", "",
		"Resolution name to set (e.g. Fixed, Won't Fix)")
	transitionCmd.AddCommand(transitionListCmd, transitionApplyCmd)
}

// -----------------------------------------------------------------------
// issue assign
// -----------------------------------------------------------------------

// assignAccountID is the account ID of the user to assign (or empty to unassign).
var assignAccountID string

var assignCmd = &cobra.Command{
	Use:   "assign <issue-key>",
	Short: "Assign an issue to a user",
	Long: `Assign a Jira issue to a specific user by their account ID.

Pass an empty --account-id to unassign the issue.

API: PUT /rest/api/2/issue/{issueIdOrKey}/assignee
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issues/#api-rest-api-2-issue-issueidorkey-assignee-put

Examples:
  jcli issue assign PROJ-42 --account-id "5f0d3aef12345678"
  jcli issue assign PROJ-42 --account-id ""   # unassign`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		if err := cl.AssignIssue(context.Background(), args[0], assignAccountID); err != nil {
			return err
		}
		if assignAccountID == "" {
			output.Success(fmt.Sprintf("Unassigned issue %s", args[0]))
		} else {
			output.Success(fmt.Sprintf("Assigned issue %s to %s", args[0], assignAccountID))
		}
		return nil
	},
}

// init registers flags for assignCmd.
func init() {
	assignCmd.Flags().StringVar(&assignAccountID, "account-id", "",
		"Account ID of the user to assign (empty string to unassign)")
}

// -----------------------------------------------------------------------
// issue worklog
// -----------------------------------------------------------------------

var worklogCmd = &cobra.Command{
	Use:   "worklog",
	Short: "Manage issue work logs",
	Long: `Commands for listing, adding and deleting work log entries on a Jira issue.

API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-worklogs/`,
}

var worklogListCmd = &cobra.Command{
	Use:   "list <issue-key>",
	Short: "List work logs for an issue",
	Long: `Retrieve all work log entries for a Jira issue.

API: GET /rest/api/2/issue/{issueIdOrKey}/worklog
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-worklogs/#api-rest-api-2-issue-issueidorkey-worklog-get

Example:
  jcli issue worklog list PROJ-42`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		list, err := cl.GetWorklogs(context.Background(), args[0])
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(list)
		}
		var rows [][]string
		for _, wl := range list.Worklogs {
			author := ""
			if wl.Author != nil {
				author = wl.Author.DisplayName
			}
			rows = append(rows, []string{wl.ID, author, wl.Started, wl.TimeSpent, output.Truncate(string(wl.Comment), 60)})
		}
		p.Table([]string{"ID", "AUTHOR", "STARTED", "TIME SPENT", "COMMENT"}, rows)
		return nil
	},
}

// worklogTimeSpent through worklogIDFlag are the flag variables for worklog sub-commands.
var (
	worklogTimeSpent string
	worklogStarted   string
	worklogComment   string
	worklogIDFlag    string
)

var worklogAddCmd = &cobra.Command{
	Use:   "add <issue-key>",
	Short: "Add a work log entry",
	Long: `Log time spent working on an issue.

The time spent value uses Jira's time notation, e.g.:
  "2h 30m"  – 2 hours and 30 minutes
  "1d"      – 1 day (8 hours by default)
  "30m"     – 30 minutes

The started date/time must be in ISO 8601 format:
  "2024-01-15T09:00:00.000+0000"

API: POST /rest/api/2/issue/{issueIdOrKey}/worklog
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-worklogs/#api-rest-api-2-issue-issueidorkey-worklog-post

Examples:
  jcli issue worklog add PROJ-42 --time-spent "2h" --started "2024-01-15T09:00:00.000+0000"
  jcli issue worklog add PROJ-42 --time-spent "30m" --comment "Fixed the bug"`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		wl, err := cl.AddWorklog(context.Background(), args[0], worklogTimeSpent, worklogStarted, worklogComment)
		if err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Added worklog %s to %s (%s)", wl.ID, args[0], wl.TimeSpent))
		return nil
	},
}

var worklogDeleteCmd = &cobra.Command{
	Use:   "delete <issue-key>",
	Short: "Delete a work log entry",
	Long: `Delete a work log entry from an issue.

API: DELETE /rest/api/2/issue/{issueIdOrKey}/worklog/{id}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-worklogs/#api-rest-api-2-issue-issueidorkey-worklog-id-delete

Example:
  jcli issue worklog delete PROJ-42 --worklog-id 10050`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		if err := cl.DeleteWorklog(context.Background(), args[0], worklogIDFlag); err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Deleted worklog %s from %s", worklogIDFlag, args[0]))
		return nil
	},
}

// init registers flags for worklog sub-commands and wires them onto worklogCmd.
func init() {
	worklogAddCmd.Flags().StringVar(&worklogTimeSpent, "time-spent", "", "Time spent, e.g. \"2h 30m\" (required)")
	_ = worklogAddCmd.MarkFlagRequired("time-spent")
	worklogAddCmd.Flags().StringVar(&worklogStarted, "started", "", "Start date/time in ISO 8601 format")
	worklogAddCmd.Flags().StringVar(&worklogComment, "comment", "", "Comment describing the work done")

	worklogDeleteCmd.Flags().StringVar(&worklogIDFlag, "worklog-id", "", "ID of the worklog entry to delete (required)")
	_ = worklogDeleteCmd.MarkFlagRequired("worklog-id")

	worklogCmd.AddCommand(worklogListCmd, worklogAddCmd, worklogDeleteCmd)
}

// -----------------------------------------------------------------------
// issue vote
// -----------------------------------------------------------------------

var voteCmd = &cobra.Command{
	Use:   "vote",
	Short: "Manage votes on an issue",
	Long: `Commands for viewing, casting and removing votes on a Jira issue.

API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-votes/`,
}

var voteGetCmd = &cobra.Command{
	Use:   "get <issue-key>",
	Short: "Get vote information for an issue",
	Long: `Show the current vote count and whether the authenticated user has voted.

API: GET /rest/api/2/issue/{issueIdOrKey}/votes
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-votes/#api-rest-api-2-issue-issueidorkey-votes-get

Example:
  jcli issue vote get PROJ-42`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		votes, err := cl.GetVotes(context.Background(), args[0])
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(votes)
		}
		p.KV([][]string{
			{"Votes", fmt.Sprintf("%d", votes.Votes)},
			{"Has Voted", fmt.Sprintf("%v", votes.HasVoted)},
		})
		return nil
	},
}

var voteAddCmd = &cobra.Command{
	Use:   "add <issue-key>",
	Short: "Cast a vote for an issue",
	Long: `Vote for a Jira issue as the currently authenticated user.

API: POST /rest/api/2/issue/{issueIdOrKey}/votes
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-votes/#api-rest-api-2-issue-issueidorkey-votes-post

Example:
  jcli issue vote add PROJ-42`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		if err := cl.AddVote(context.Background(), args[0]); err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Voted for issue %s", args[0]))
		return nil
	},
}

var voteRemoveCmd = &cobra.Command{
	Use:   "remove <issue-key>",
	Short: "Remove your vote from an issue",
	Long: `Remove the currently authenticated user's vote from a Jira issue.

API: DELETE /rest/api/2/issue/{issueIdOrKey}/votes
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-votes/#api-rest-api-2-issue-issueidorkey-votes-delete

Example:
  jcli issue vote remove PROJ-42`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		if err := cl.RemoveVote(context.Background(), args[0]); err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Removed vote from issue %s", args[0]))
		return nil
	},
}

// init registers vote sub-commands on voteCmd.
func init() {
	voteCmd.AddCommand(voteGetCmd, voteAddCmd, voteRemoveCmd)
}

// -----------------------------------------------------------------------
// issue watch
// -----------------------------------------------------------------------

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Manage watchers on an issue",
	Long: `Commands for listing, adding and removing watchers from a Jira issue.

API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-watchers/`,
}

var watchListCmd = &cobra.Command{
	Use:   "list <issue-key>",
	Short: "List watchers on an issue",
	Long: `Show all users who are watching a Jira issue.

API: GET /rest/api/2/issue/{issueIdOrKey}/watchers
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-watchers/#api-rest-api-2-issue-issueidorkey-watchers-get

Example:
  jcli issue watch list PROJ-42`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		watchers, err := cl.GetWatchers(context.Background(), args[0])
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(watchers)
		}
		fmt.Fprintf(output.Stdout(), "Watch count: %d\n", watchers.WatchCount)
		var rows [][]string
		for _, u := range watchers.Watchers {
			rows = append(rows, []string{u.AccountID, u.DisplayName, u.EmailAddress})
		}
		p.Table([]string{"ACCOUNT ID", "DISPLAY NAME", "EMAIL"}, rows)
		return nil
	},
}

// watchAccountID is the account ID of the user to add or remove as a watcher.
var watchAccountID string

var watchAddCmd = &cobra.Command{
	Use:   "add <issue-key>",
	Short: "Add a watcher to an issue",
	Long: `Add a user as a watcher on a Jira issue.

API: POST /rest/api/2/issue/{issueIdOrKey}/watchers
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-watchers/#api-rest-api-2-issue-issueidorkey-watchers-post

Example:
  jcli issue watch add PROJ-42 --account-id "5f0d3aef12345678"`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		if err := cl.AddWatcher(context.Background(), args[0], watchAccountID); err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Added watcher %s to %s", watchAccountID, args[0]))
		return nil
	},
}

var watchRemoveCmd = &cobra.Command{
	Use:   "remove <issue-key>",
	Short: "Remove a watcher from an issue",
	Long: `Remove a user from the watcher list of a Jira issue.

API: DELETE /rest/api/2/issue/{issueIdOrKey}/watchers?accountId={accountId}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-watchers/#api-rest-api-2-issue-issueidorkey-watchers-delete

Example:
  jcli issue watch remove PROJ-42 --account-id "5f0d3aef12345678"`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		if err := cl.RemoveWatcher(context.Background(), args[0], watchAccountID); err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Removed watcher %s from %s", watchAccountID, args[0]))
		return nil
	},
}

// init registers flags for watch sub-commands and wires them onto watchCmd.
func init() {
	watchAddCmd.Flags().StringVar(&watchAccountID, "account-id", "", "Account ID of the user to add as watcher (required)")
	_ = watchAddCmd.MarkFlagRequired("account-id")
	watchRemoveCmd.Flags().StringVar(&watchAccountID, "account-id", "", "Account ID of the user to remove (required)")
	_ = watchRemoveCmd.MarkFlagRequired("account-id")
	watchCmd.AddCommand(watchListCmd, watchAddCmd, watchRemoveCmd)
}

// -----------------------------------------------------------------------
// issue link
// -----------------------------------------------------------------------

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Manage issue links",
	Long: `Commands for listing link types and creating/deleting links between issues.

API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-links/`,
}

var linkTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "List available issue link types",
	Long: `Show all link type names that can be used when linking issues.

API: GET /rest/api/2/issueLinkType
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-link-types/#api-rest-api-2-issuelinktype-get

Example:
  jcli issue link types`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		list, err := cl.GetIssueLinkTypes(context.Background())
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(list)
		}
		var rows [][]string
		for _, lt := range list.IssueLinkTypes {
			rows = append(rows, []string{lt.ID, lt.Name, lt.Inward, lt.Outward})
		}
		p.Table([]string{"ID", "NAME", "INWARD", "OUTWARD"}, rows)
		return nil
	},
}

// linkTypeName through linkIDFlag are the flag variables for link sub-commands.
var (
	linkTypeName string
	linkInward   string
	linkOutward  string
	linkComment  string
	linkIDFlag   string
)

var linkCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a link between two issues",
	Long: `Create a directional link between two Jira issues.

Use "jcli issue link types" to see available link type names.

API: POST /rest/api/2/issueLink
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-links/#api-rest-api-2-issuelink-post

Examples:
  jcli issue link create --type "blocks" --inward PROJ-42 --outward PROJ-50
  jcli issue link create --type "relates to" --inward PROJ-1 --outward PROJ-2`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		if err := cl.LinkIssues(context.Background(), linkTypeName, linkInward, linkOutward, linkComment); err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Linked %s %s %s", linkInward, linkTypeName, linkOutward))
		return nil
	},
}

var linkDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an issue link",
	Long: `Delete an existing link between issues by its link ID.

API: DELETE /rest/api/2/issueLink/{linkId}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-links/#api-rest-api-2-issuelink-linkid-delete

Example:
  jcli issue link delete --link-id 10000`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		if err := cl.DeleteIssueLink(context.Background(), linkIDFlag); err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Deleted issue link %s", linkIDFlag))
		return nil
	},
}

// init registers flags for link sub-commands and wires them onto linkCmd.
func init() {
	linkCreateCmd.Flags().StringVar(&linkTypeName, "type", "", "Link type name (required; see 'link types')")
	_ = linkCreateCmd.MarkFlagRequired("type")
	linkCreateCmd.Flags().StringVar(&linkInward, "inward", "", "Inward issue key (required)")
	_ = linkCreateCmd.MarkFlagRequired("inward")
	linkCreateCmd.Flags().StringVar(&linkOutward, "outward", "", "Outward issue key (required)")
	_ = linkCreateCmd.MarkFlagRequired("outward")
	linkCreateCmd.Flags().StringVar(&linkComment, "comment", "", "Optional comment to add to the link")

	linkDeleteCmd.Flags().StringVar(&linkIDFlag, "link-id", "", "Link ID to delete (required)")
	_ = linkDeleteCmd.MarkFlagRequired("link-id")

	linkCmd.AddCommand(linkTypesCmd, linkCreateCmd, linkDeleteCmd)
}

// -----------------------------------------------------------------------
// issue attach
// -----------------------------------------------------------------------

var attachCmd = &cobra.Command{
	Use:   "attach",
	Short: "Manage issue attachments",
	Long: `Commands for uploading and deleting attachments on a Jira issue.

API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-attachments/`,
}

// attachFilePath is the local filesystem path of the file to upload.
var attachFilePath string

var attachAddCmd = &cobra.Command{
	Use:   "add <issue-key>",
	Short: "Upload a file attachment to an issue",
	Long: `Upload a file and attach it to a Jira issue.

API: POST /rest/api/2/issue/{issueIdOrKey}/attachments
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-attachments/#api-rest-api-2-issue-issueidorkey-attachments-post

Example:
  jcli issue attach add PROJ-42 --file /path/to/screenshot.png`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		attachments, err := cl.AddAttachment(context.Background(), args[0], attachFilePath)
		if err != nil {
			return err
		}
		for _, a := range attachments {
			output.Success(fmt.Sprintf("Uploaded attachment %s (ID: %s)", a.Filename, a.ID))
		}
		return nil
	},
}

// attachDeleteID is the attachment ID to delete.
var attachDeleteID string

var attachDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an attachment",
	Long: `Delete a file attachment from a Jira issue by its attachment ID.

API: DELETE /rest/api/2/attachment/{id}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-attachments/#api-rest-api-2-attachment-id-delete

Example:
  jcli issue attach delete --attachment-id 10100`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		if err := cl.DeleteAttachment(context.Background(), attachDeleteID); err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Deleted attachment %s", attachDeleteID))
		return nil
	},
}

// init registers flags for attach sub-commands and wires them onto attachCmd.
func init() {
	attachAddCmd.Flags().StringVar(&attachFilePath, "file", "", "Path to the file to upload (required)")
	_ = attachAddCmd.MarkFlagRequired("file")

	attachDeleteCmd.Flags().StringVar(&attachDeleteID, "attachment-id", "", "ID of the attachment to delete (required)")
	_ = attachDeleteCmd.MarkFlagRequired("attachment-id")

	attachCmd.AddCommand(attachAddCmd, attachDeleteCmd)
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// firstNonEmpty returns the first non-empty string from the provided values.
// It is used to apply a precedence chain, e.g. CLI flag → config default.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
