// Package meta implements commands that expose Jira metadata such as issue
// types, priorities, statuses and field definitions.
package meta

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/steveohara/jcli/cmd"
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
	)
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
