// Package meta implements commands that expose Jira metadata such as issue
// types, priorities, statuses and field definitions.
package meta

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveohara/jcli/cmd"
	"github.com/steveohara/jcli/internal/client"
	"github.com/steveohara/jcli/internal/output"
)

// MetaCmd is the parent command for metadata operations.
var MetaCmd = &cobra.Command{
	Use:   "meta",
	Short: "List Jira metadata (issue types, priorities, statuses, fields)",
	Long: `Commands for listing Jira configuration metadata such as issue types,
priorities, workflow statuses and custom field definitions.

These commands are useful for discovering valid values for other commands,
for example finding the correct issue type name before running 'jcli issue create'.

API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/`,
}

func init() {
	MetaCmd.AddCommand(
		issueTypesCmd,
		prioritiesCmd,
		statusesCmd,
		fieldsCmd,
		fieldSearchCmd,
		fieldContextsCmd,
		fieldOptionsCmd,
		fieldAllowedValuesCmd,
		resolutionsCmd,
		serverInfoCmd,
		projectStatusesCmd,
		linkTypesCmd,
		configurationCmd,
	)

	// field-search flags
	fieldSearchCmd.Flags().StringSlice("id", nil, "Filter by field ID (repeatable)")
	fieldSearchCmd.Flags().String("query", "", "Filter by field name/description substring")
	fieldSearchCmd.Flags().String("type", "", "Filter by field type: system or custom")
	fieldSearchCmd.Flags().String("order-by", "", "Order results by: contextsCount, lastUsed, name, screensCount, projectsCount")
	fieldSearchCmd.Flags().String("expand", "", "Include extra data: screensCount, contextsCount, lastUsed (comma-separated)")
	fieldSearchCmd.Flags().IntSlice("project-ids", nil, "Filter to fields used in these project IDs")
	fieldSearchCmd.Flags().Int("start-at", 0, "Pagination offset")
	fieldSearchCmd.Flags().Int("max-results", 50, "Maximum results to return")

	// field-contexts flags
	fieldContextsCmd.Flags().Bool("global", false, "Filter to global contexts only")
	fieldContextsCmd.Flags().Bool("any-issue-type", false, "Filter to contexts that apply to all issue types")
	fieldContextsCmd.Flags().IntSlice("context-id", nil, "Filter by specific context IDs")
	fieldContextsCmd.Flags().Int("start-at", 0, "Pagination offset")
	fieldContextsCmd.Flags().Int("max-results", 50, "Maximum results to return")

	// field-options flags
	fieldOptionsCmd.Flags().String("context-id", "", "Filter options to a specific context ID")
	fieldOptionsCmd.Flags().Bool("only-options", false, "Return only options, excluding cascading options")
	fieldOptionsCmd.Flags().Int("start-at", 0, "Pagination offset")
	fieldOptionsCmd.Flags().Int("max-results", 100, "Maximum results to return")

	// field-allowed-values flags
	fieldAllowedValuesCmd.Flags().String("issue", "", "Issue key to read edit metadata from (required)")
	_ = fieldAllowedValuesCmd.MarkFlagRequired("issue")

	// project-statuses flags
	projectStatusesCmd.Flags().String("issue-type", "", "Filter output to a specific issue type name")
}

// -----------------------------------------------------------------------
// meta issue-types
// -----------------------------------------------------------------------

var issueTypesCmd = &cobra.Command{
	Use:   "issue-types",
	Short: "List all issue types",
	Long: `List all issue types available on the Jira instance.

This is useful to discover the exact type names required for the
--type flag of 'jcli issue create'.

API: GET /rest/api/2/issuetype
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-types/#api-rest-api-2-issuetype-get

Example:
  jcli meta issue-types
  jcli meta issue-types --output json`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		types, err := cl.GetIssueTypes(context.Background())
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(types)
		}
		var rows [][]string
		for _, t := range types {
			rows = append(rows, []string{t.ID, t.Name, fmt.Sprintf("%v", t.Subtask), t.Description})
		}
		p.Table([]string{"ID", "NAME", "SUBTASK", "DESCRIPTION"}, rows)
		return nil
	},
}

// -----------------------------------------------------------------------
// meta priorities
// -----------------------------------------------------------------------

var prioritiesCmd = &cobra.Command{
	Use:   "priorities",
	Short: "List all issue priorities",
	Long: `List all issue priorities available on the Jira instance.

This is useful to discover the exact priority names required for the
--priority flag of 'jcli issue create' and 'jcli issue update'.

API: GET /rest/api/2/priority
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-priorities/#api-rest-api-2-priority-get

Example:
  jcli meta priorities`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		priorities, err := cl.GetPriorities(context.Background())
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(priorities)
		}
		var rows [][]string
		for _, pr := range priorities {
			rows = append(rows, []string{pr.ID, pr.Name, pr.Description})
		}
		p.Table([]string{"ID", "NAME", "DESCRIPTION"}, rows)
		return nil
	},
}

// -----------------------------------------------------------------------
// meta statuses
// -----------------------------------------------------------------------

var statusesCmd = &cobra.Command{
	Use:   "statuses",
	Short: "List all issue statuses",
	Long: `List all workflow statuses available on the Jira instance.

API: GET /rest/api/2/status
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-workflow-statuses/#api-rest-api-2-status-get

Example:
  jcli meta statuses`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		statuses, err := cl.GetStatuses(context.Background())
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(statuses)
		}
		var rows [][]string
		for _, s := range statuses {
			rows = append(rows, []string{s.ID, s.Name, s.Category.Name, s.Description})
		}
		p.Table([]string{"ID", "NAME", "CATEGORY", "DESCRIPTION"}, rows)
		return nil
	},
}

// -----------------------------------------------------------------------
// meta fields
// -----------------------------------------------------------------------

var fieldsCmd = &cobra.Command{
	Use:   "fields",
	Short: "List all issue fields",
	Long: `List all field definitions available on the Jira instance, including both
system fields and custom fields.

Use the field ID from this list with the --fields flag on 'jcli issue get'
or 'jcli issue search' to limit which fields are returned.

API: GET /rest/api/2/field
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-fields/#api-rest-api-2-field-get

Example:
  jcli meta fields
  jcli meta fields --output json`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		fields, err := cl.GetFields(context.Background())
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(fields)
		}
		var rows [][]string
		for _, f := range fields {
			rows = append(rows, []string{
				f.ID,
				f.Name,
				fmt.Sprintf("%v", f.Custom),
				fmt.Sprintf("%v", f.Searchable),
			})
		}
		p.Table([]string{"ID", "NAME", "CUSTOM", "SEARCHABLE"}, rows)
		return nil
	},
}

// -----------------------------------------------------------------------
// meta field-search
// -----------------------------------------------------------------------

var fieldSearchCmd = &cobra.Command{
	Use:   "field-search",
	Short: "Search field definitions – Jira Cloud only",
	Long: `Search Jira field definitions using the paginated field search endpoint.
Supports filtering by ID, name/description substring, field type, and project.

NOTE: This command uses GET /rest/api/2/field/search which is available on
Jira Cloud only. On Jira Server / Data Center it returns HTTP 404.
Use 'jcli meta fields' instead – it works on both Cloud and Server.

Use --expand to include additional metadata such as screensCount, contextsCount,
and lastUsed information.

API: GET /rest/api/2/field/search  (Jira Cloud only)
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-fields/#api-rest-api-2-field-search-get

Examples:
  jcli meta field-search
  jcli meta field-search --type custom
  jcli meta field-search --query "sprint" --output json
  jcli meta field-search --id customfield_10014 --expand screensCount,contextsCount
  jcli meta field-search --order-by name --max-results 100`,
	RunE: func(c *cobra.Command, args []string) error {
		ids, _ := c.Flags().GetStringSlice("id")
		query, _ := c.Flags().GetString("query")
		fieldType, _ := c.Flags().GetString("type")
		orderBy, _ := c.Flags().GetString("order-by")
		expand, _ := c.Flags().GetString("expand")
		projectIDs, _ := c.Flags().GetIntSlice("project-ids")
		startAt, _ := c.Flags().GetInt("start-at")
		maxResults, _ := c.Flags().GetInt("max-results")

		cl, cfg := cmd.NewClient()
		page, err := cl.SearchFields(context.Background(), client.FieldSearchOptions{
			IDs:        ids,
			Query:      query,
			Type:       fieldType,
			OrderBy:    orderBy,
			Expand:     expand,
			ProjectIDs: projectIDs,
			StartAt:    startAt,
			MaxResults: maxResults,
		})
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(page)
		}
		var rows [][]string
		for _, f := range page.Values {
			rows = append(rows, []string{
				f.ID,
				f.Name,
				fmt.Sprintf("%v", f.Custom),
				f.SearcherKey,
				fmt.Sprintf("%d", f.ScreensCount),
				fmt.Sprintf("%d", f.ContextsCount),
				f.Description,
			})
		}
		p.Table([]string{"ID", "NAME", "CUSTOM", "SEARCHER", "SCREENS", "CONTEXTS", "DESCRIPTION"}, rows)
		fmt.Printf("\nShowing %d–%d of %d\n", page.StartAt+1, page.StartAt+len(page.Values), page.Total)
		return nil
	},
}

// -----------------------------------------------------------------------
// meta field-contexts
// -----------------------------------------------------------------------

var fieldContextsCmd = &cobra.Command{
	Use:   "field-contexts <fieldId>",
	Short: "List contexts for a custom field (Jira Cloud only)",
	Long: `Returns the contexts a custom field is used in.

NOTE: This command uses a Jira Cloud-only API endpoint. It will return a 404
on Jira Server / Data Center. Use 'jcli meta field-allowed-values --issue <key>'
as a compatible alternative on Server instances.

Contexts determine which projects and issue types a custom field applies to.
A global context applies to all projects; a non-global context is scoped to
specific projects.

API: GET /rest/api/2/field/{fieldId}/context
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-custom-field-contexts/#api-rest-api-2-field-fieldid-context-get

Examples:
  jcli meta field-contexts customfield_10014
  jcli meta field-contexts customfield_10014 --global
  jcli meta field-contexts customfield_10014 --any-issue-type
  jcli meta field-contexts customfield_10014 --output json`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		fieldID := args[0]
		globalOnly, _ := c.Flags().GetBool("global")
		anyIssueType, _ := c.Flags().GetBool("any-issue-type")
		contextIDs, _ := c.Flags().GetIntSlice("context-id")
		startAt, _ := c.Flags().GetInt("start-at")
		maxResults, _ := c.Flags().GetInt("max-results")

		opts := client.FieldContextOptions{
			ContextIDs: contextIDs,
			StartAt:    startAt,
			MaxResults: maxResults,
		}
		if c.Flags().Changed("global") {
			opts.IsGlobalContext = &globalOnly
		}
		if c.Flags().Changed("any-issue-type") {
			opts.IsAnyIssueType = &anyIssueType
		}

		cl, cfg := cmd.NewClient()
		page, err := cl.GetFieldContexts(context.Background(), fieldID, opts)
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(page)
		}
		var rows [][]string
		for _, ctx := range page.Values {
			rows = append(rows, []string{
				ctx.ID,
				ctx.Name,
				fmt.Sprintf("%v", ctx.IsGlobalContext),
				fmt.Sprintf("%v", ctx.IsAnyIssueType),
				ctx.Description,
			})
		}
		p.Table([]string{"ID", "NAME", "GLOBAL", "ANY_ISSUE_TYPE", "DESCRIPTION"}, rows)
		fmt.Printf("\nShowing %d–%d of %d\n", page.StartAt+1, page.StartAt+len(page.Values), page.Total)
		return nil
	},
}

// -----------------------------------------------------------------------
// meta field-options
// -----------------------------------------------------------------------

var fieldOptionsCmd = &cobra.Command{
	Use:   "field-options <fieldId>",
	Short: "List allowed values for a custom field context (Jira Cloud only)",
	Long: `Returns the allowed option values for a custom select/radio/checkbox field.

NOTE: This command uses a Jira Cloud-only API endpoint. It will return a 404
on Jira Server / Data Center. Use 'jcli meta field-allowed-values --issue <key>'
as a compatible alternative on Server instances.

This is useful for discovering valid values before setting a custom field
on an issue. Use --context-id to limit results to a specific field context.

API: GET /rest/api/2/field/{fieldId}/context/option
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-custom-field-options/#api-rest-api-2-field-fieldid-context-option-get

Examples:
  jcli meta field-options customfield_10014
  jcli meta field-options customfield_10014 --context-id 10025
  jcli meta field-options customfield_10014 --only-options
  jcli meta field-options customfield_10014 --output json`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		fieldID := args[0]
		contextID, _ := c.Flags().GetString("context-id")
		onlyOptions, _ := c.Flags().GetBool("only-options")
		startAt, _ := c.Flags().GetInt("start-at")
		maxResults, _ := c.Flags().GetInt("max-results")

		cl, cfg := cmd.NewClient()
		page, err := cl.GetFieldOptions(context.Background(), fieldID, contextID, onlyOptions, startAt, maxResults)
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(page)
		}
		var rows [][]string
		for _, opt := range page.Values {
			disabled := ""
			if opt.Disabled {
				disabled = "disabled"
			}
			rows = append(rows, []string{opt.ID, opt.Value, disabled})
		}
		p.Table([]string{"ID", "VALUE", "STATUS"}, rows)
		fmt.Printf("\nShowing %d–%d of %d\n", page.StartAt+1, page.StartAt+len(page.Values), page.Total)
		return nil
	},
}

// -----------------------------------------------------------------------
// meta field-allowed-values
// -----------------------------------------------------------------------

var fieldAllowedValuesCmd = &cobra.Command{
	Use:   "field-allowed-values <fieldId>",
	Short: "List allowed values for a field via issue edit metadata (Cloud + Server)",
	Long: `Returns the allowed values for a field by reading the edit metadata of an
existing issue. This works on both Jira Cloud and Jira Server / Data Center.

Provide any issue key from a project where the field is active. The command
extracts the allowedValues for the specified field from the editmeta response.

This is the recommended alternative to 'field-options' and 'field-contexts'
on Jira Server / Data Center, where the context/option API endpoints are not
available.

API: GET /rest/api/2/issue/{issueKey}/editmeta
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issues/#api-rest-api-2-issue-issueidorkey-editmeta-get

Examples:
  jcli meta field-allowed-values customfield_31004 --issue PROJ-42
  jcli meta field-allowed-values priority --issue PROJ-42
  jcli meta field-allowed-values customfield_31004 --issue PROJ-42 --output json`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		fieldID := args[0]
		issueKey, _ := c.Flags().GetString("issue")

		cl, cfg := cmd.NewClient()
		meta, err := cl.GetEditMeta(context.Background(), issueKey)
		if err != nil {
			return err
		}

		field, ok := meta.Fields[fieldID]
		if !ok {
			return fmt.Errorf("field %q not found in edit metadata for %s (field may not be on this issue type or project)", fieldID, issueKey)
		}
		if len(field.AllowedValues) == 0 {
			fmt.Printf("Field %q (%s) has no allowed values (free-text or not applicable).\n", fieldID, field.Name)
			return nil
		}

		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(field.AllowedValues)
		}
		var rows [][]string
		for _, av := range field.AllowedValues {
			label := av.Value
			if label == "" {
				label = av.Name
			}
			rows = append(rows, []string{av.ID, label, av.Description})
		}
		p.Table([]string{"ID", "VALUE", "DESCRIPTION"}, rows)
		return nil
	},
}

// -----------------------------------------------------------------------
// meta resolutions
// -----------------------------------------------------------------------

var resolutionsCmd = &cobra.Command{
	Use:   "resolutions",
	Short: "List all issue resolutions",
	Long: `List all resolution values defined on the Jira instance.

Resolutions are used when closing or transitioning an issue to a "Done"
workflow state. The NAME column is the value to pass to
'jcli issue transition apply --resolution <name>'.

API: GET /rest/api/2/resolution
Works on both Jira Cloud and Jira Server / Data Center.

Examples:
  jcli meta resolutions
  jcli meta resolutions --output json`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		resolutions, err := cl.GetResolutions(context.Background())
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(resolutions)
		}
		var rows [][]string
		for _, r := range resolutions {
			rows = append(rows, []string{r.ID, r.Name, r.Description})
		}
		p.Table([]string{"ID", "NAME", "DESCRIPTION"}, rows)
		return nil
	},
}

// -----------------------------------------------------------------------
// meta server-info
// -----------------------------------------------------------------------

var serverInfoCmd = &cobra.Command{
	Use:   "server-info",
	Short: "Show Jira instance version and build information",
	Long: `Display build and runtime information about the connected Jira instance.

Useful for confirming connectivity, checking the server version before using
version-specific features, and comparing deploymentType (Cloud vs Server).

API: GET /rest/api/2/serverInfo
Works on both Jira Cloud and Jira Server / Data Center.

Examples:
  jcli meta server-info
  jcli meta server-info --output json`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		info, err := cl.GetServerInfo(context.Background())
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(info)
		}
		rows := [][]string{
			{"Title", info.ServerTitle},
			{"Base URL", info.BaseURL},
			{"Version", info.Version},
			{"Deployment Type", info.DeploymentType},
			{"Build Number", fmt.Sprintf("%d", info.BuildNumber)},
			{"Build Date", info.BuildDate},
			{"Server Time", info.ServerTime},
		}
		p.Table([]string{"PROPERTY", "VALUE"}, rows)
		return nil
	},
}

// -----------------------------------------------------------------------
// meta project-statuses
// -----------------------------------------------------------------------

var projectStatusesCmd = &cobra.Command{
	Use:   "project-statuses <project-key>",
	Short: "List statuses available in a project, grouped by issue type",
	Long: `Returns the workflow statuses available for each issue type within a project.

This is more precise than 'jcli meta statuses', which lists all statuses on
the instance regardless of which projects or issue types they apply to.

Use --issue-type to filter output to a specific issue type name.

API: GET /rest/api/2/project/{projectIdOrKey}/statuses
Works on both Jira Cloud and Jira Server / Data Center.

Examples:
  jcli meta project-statuses PROJ
  jcli meta project-statuses PROJ --issue-type Bug
  jcli meta project-statuses PROJ --output json`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		projectKey := args[0]
		issueTypeFilter, _ := c.Flags().GetString("issue-type")

		cl, cfg := cmd.NewClient()
		issueTypes, err := cl.GetProjectStatuses(context.Background(), projectKey)
		if err != nil {
			return err
		}

		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			if issueTypeFilter != "" {
				for _, it := range issueTypes {
					if strings.EqualFold(it.Name, issueTypeFilter) {
						return p.JSON(it)
					}
				}
				return fmt.Errorf("issue type %q not found in project %s", issueTypeFilter, projectKey)
			}
			return p.JSON(issueTypes)
		}

		var rows [][]string
		for _, it := range issueTypes {
			if issueTypeFilter != "" && !strings.EqualFold(it.Name, issueTypeFilter) {
				continue
			}
			for _, s := range it.Statuses {
				rows = append(rows, []string{it.Name, s.ID, s.Name, s.Category.Name})
			}
		}
		if len(rows) == 0 && issueTypeFilter != "" {
			return fmt.Errorf("issue type %q not found in project %s", issueTypeFilter, projectKey)
		}
		p.Table([]string{"ISSUE TYPE", "STATUS ID", "STATUS", "CATEGORY"}, rows)
		return nil
	},
}

// -----------------------------------------------------------------------
// meta link-types
// -----------------------------------------------------------------------

var linkTypesCmd = &cobra.Command{
	Use:   "link-types",
	Short: "List all issue link type definitions",
	Long: `List all issue link types available on the Jira instance.

The NAME column is the value to pass to 'jcli issue link create --type <name>'.
INWARD and OUTWARD are the directional labels used in the Jira UI.

API: GET /rest/api/2/issueLinkType
Works on both Jira Cloud and Jira Server / Data Center.

Examples:
  jcli meta link-types
  jcli meta link-types --output json`,
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

// -----------------------------------------------------------------------
// meta configuration
// -----------------------------------------------------------------------

var configurationCmd = &cobra.Command{
	Use:   "configuration",
	Short: "Show instance-level feature flags and time-tracking settings",
	Long: `Display the global configuration flags for the Jira instance, including
which features are enabled (voting, watching, issue linking, sub-tasks,
attachments, time tracking) and the time-tracking unit settings.

This is useful for confirming which features are available before attempting
to use them, for example checking timeTrackingEnabled before logging work.

API: GET /rest/api/2/configuration
Works on both Jira Cloud and Jira Server / Data Center.

Examples:
  jcli meta configuration
  jcli meta configuration --output json`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		conf, err := cl.GetConfiguration(context.Background())
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(conf)
		}
		tt := conf.TimeTracking
		rows := [][]string{
			{"votingEnabled", fmt.Sprintf("%v", conf.VotingEnabled)},
			{"watchingEnabled", fmt.Sprintf("%v", conf.WatchingEnabled)},
			{"unassignedIssuesAllowed", fmt.Sprintf("%v", conf.UnassignedIssuesAllowed)},
			{"subTasksEnabled", fmt.Sprintf("%v", conf.SubTasksEnabled)},
			{"issueLinkingEnabled", fmt.Sprintf("%v", conf.IssueLinkingEnabled)},
			{"timeTrackingEnabled", fmt.Sprintf("%v", conf.TimeTrackingEnabled)},
			{"attachmentsEnabled", fmt.Sprintf("%v", conf.AttachmentsEnabled)},
			{"workingHoursPerDay", fmt.Sprintf("%.1f", tt.WorkingHoursPerDay)},
			{"workingDaysPerWeek", fmt.Sprintf("%.1f", tt.WorkingDaysPerWeek)},
			{"timeFormat", tt.TimeFormat},
			{"defaultUnit", tt.DefaultUnit},
		}
		p.Table([]string{"SETTING", "VALUE"}, rows)
		return nil
	},
}

// boolPtr is a helper to get a pointer to a bool literal.
// It exists to allow flag-changed detection for optional boolean filters.
func boolPtr(b bool) *bool { return &b }

// intSliceToString converts an int slice to a comma-separated string for display.
func intSliceToString(ints []int) string {
	strs := make([]string, len(ints))
	for i, v := range ints {
		strs[i] = strconv.Itoa(v)
	}
	return strings.Join(strs, ",")
}
