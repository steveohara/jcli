// Package board implements board and sprint related sub-commands for jcli.
package board

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/steveohara/jcli/cmd"
	"github.com/steveohara/jcli/internal/client"
	"github.com/steveohara/jcli/internal/output"
)

// BoardCmd is the parent command for board/sprint operations.
var BoardCmd = &cobra.Command{
	Use:   "board",
	Short: "Manage Jira Agile boards and sprints",
	Long: `Commands for listing boards, viewing sprints, creating sprints and listing
sprint issues.

The board and sprint commands use the Jira Agile REST API:
  Base path: /rest/agile/1.0

API reference: https://developer.atlassian.com/cloud/jira/software/rest/api-group-board/`,
}

// init registers all board sub-commands on BoardCmd.
func init() {
	BoardCmd.AddCommand(
		listCmd,
		sprintCmd,
	)
}

// -----------------------------------------------------------------------
// board list
// -----------------------------------------------------------------------

// listProject filters boards by project key; listMaxResults caps the result count.
var (
	listProject    string
	listMaxResults int
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List Agile boards",
	Long: `List all Jira Agile boards visible to the authenticated user.

Optionally filter boards by project key using --project.

API: GET /rest/agile/1.0/board
Ref: https://developer.atlassian.com/cloud/jira/software/rest/api-group-board/#api-rest-agile-1-0-board-get

Examples:
  jcli board list
  jcli board list --project PROJ
  jcli board list --max-results 100 --output json`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		projectKey := listProject
		if projectKey == "" {
			projectKey = cfg.DefaultProject
		}
		result, err := cl.GetBoards(context.Background(), projectKey, listMaxResults)
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(result)
		}
		var rows [][]string
		for _, b := range result.Values {
			projectKey := ""
			if b.Location != nil {
				projectKey = b.Location.ProjectKey
			}
			rows = append(rows, []string{fmt.Sprintf("%d", b.ID), b.Name, b.Type, projectKey})
		}
		p.Table([]string{"ID", "NAME", "TYPE", "PROJECT"}, rows)
		return nil
	},
}

// init registers flags for listCmd.
func init() {
	listCmd.Flags().StringVar(&listProject, "project", "",
		"Filter boards by project key (overrides default project)")
	listCmd.Flags().IntVar(&listMaxResults, "max-results", 50,
		"Maximum number of boards to return")
}

// -----------------------------------------------------------------------
// board sprint
// -----------------------------------------------------------------------

var sprintCmd = &cobra.Command{
	Use:   "sprint",
	Short: "Manage sprints on a board",
	Long: `Commands for listing, creating and updating sprints on a Jira Agile board,
and for listing issues within a sprint.

API reference: https://developer.atlassian.com/cloud/jira/software/rest/api-group-board/#api-rest-agile-1-0-board-boardid-sprint-get`,
}

// -----------------------------------------------------------------------
// board sprint list
// -----------------------------------------------------------------------

// sprintBoardID identifies the board whose sprints are to be listed.
// sprintState optionally filters by sprint state: active, future, or closed.
var (
	sprintBoardID int
	sprintState   string
)

var sprintListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sprints on a board",
	Long: `List all sprints for a specified board.

Use --state to filter by sprint state:
  active   – the currently active sprint
  future   – sprints not yet started
  closed   – completed sprints

API: GET /rest/agile/1.0/board/{boardId}/sprint
Ref: https://developer.atlassian.com/cloud/jira/software/rest/api-group-board/#api-rest-agile-1-0-board-boardid-sprint-get

Examples:
  jcli board sprint list --board-id 1
  jcli board sprint list --board-id 1 --state active`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		result, err := cl.GetSprints(context.Background(), sprintBoardID, sprintState)
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(result)
		}
		var rows [][]string
		for _, s := range result.Values {
			rows = append(rows, []string{
				fmt.Sprintf("%d", s.ID),
				s.Name,
				s.State,
				s.StartDate,
				s.EndDate,
				output.Truncate(s.Goal, 50),
			})
		}
		p.Table([]string{"ID", "NAME", "STATE", "START", "END", "GOAL"}, rows)
		return nil
	},
}

// init registers flags for sprintListCmd.
func init() {
	sprintListCmd.Flags().IntVar(&sprintBoardID, "board-id", 0, "Board ID (required)")
	_ = sprintListCmd.MarkFlagRequired("board-id")
	sprintListCmd.Flags().StringVar(&sprintState, "state", "",
		"Filter by sprint state: active, future, or closed")
}

// -----------------------------------------------------------------------
// board sprint create
// -----------------------------------------------------------------------

// sprintCreateName through sprintCreateBoardID are the flag variables for "board sprint create".
var (
	sprintCreateName      string
	sprintCreateGoal      string
	sprintCreateStartDate string
	sprintCreateEndDate   string
	sprintCreateBoardID   int
)

var sprintCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new sprint",
	Long: `Create a new sprint on a Jira Agile board.

API: POST /rest/agile/1.0/sprint
Ref: https://developer.atlassian.com/cloud/jira/software/rest/api-group-sprint/#api-rest-agile-1-0-sprint-post

Examples:
  jcli board sprint create --board-id 1 --name "Sprint 5"
  jcli board sprint create --board-id 1 --name "Sprint 6" \
      --start "2024-02-01T00:00:00.000Z" --end "2024-02-14T00:00:00.000Z" \
      --goal "Complete the login feature"`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		req := &client.CreateSprintRequest{
			Name:          sprintCreateName,
			Goal:          sprintCreateGoal,
			StartDate:     sprintCreateStartDate,
			EndDate:       sprintCreateEndDate,
			OriginBoardID: sprintCreateBoardID,
		}
		sprint, err := cl.CreateSprint(context.Background(), req)
		if err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Created sprint %q (ID: %d)", sprint.Name, sprint.ID))
		return nil
	},
}

// init registers flags for sprintCreateCmd.
func init() {
	sprintCreateCmd.Flags().IntVar(&sprintCreateBoardID, "board-id", 0, "Board ID to create the sprint on (required)")
	_ = sprintCreateCmd.MarkFlagRequired("board-id")
	sprintCreateCmd.Flags().StringVar(&sprintCreateName, "name", "", "Sprint name (required)")
	_ = sprintCreateCmd.MarkFlagRequired("name")
	sprintCreateCmd.Flags().StringVar(&sprintCreateGoal, "goal", "", "Sprint goal description")
	sprintCreateCmd.Flags().StringVar(&sprintCreateStartDate, "start", "", "Sprint start date/time (ISO 8601)")
	sprintCreateCmd.Flags().StringVar(&sprintCreateEndDate, "end", "", "Sprint end date/time (ISO 8601)")
}

// -----------------------------------------------------------------------
// board sprint update
// -----------------------------------------------------------------------

// sprintUpdateID through sprintUpdateEnd are the flag variables for "board sprint update".
var (
	sprintUpdateID    int
	sprintUpdateName  string
	sprintUpdateGoal  string
	sprintUpdateState string
	sprintUpdateStart string
	sprintUpdateEnd   string
)

var sprintUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a sprint",
	Long: `Update the name, goal, dates or state of a Jira sprint.

Valid state transitions:
  future  → active   (start the sprint)
  active  → closed   (complete the sprint)

API: PUT /rest/agile/1.0/sprint/{sprintId}
Ref: https://developer.atlassian.com/cloud/jira/software/rest/api-group-sprint/#api-rest-agile-1-0-sprint-sprintid-put

Examples:
  jcli board sprint update --id 5 --name "Sprint 5 (extended)"
  jcli board sprint update --id 5 --state active`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		fields := make(map[string]interface{})
		if sprintUpdateName != "" {
			fields["name"] = sprintUpdateName
		}
		if sprintUpdateGoal != "" {
			fields["goal"] = sprintUpdateGoal
		}
		if sprintUpdateState != "" {
			fields["state"] = sprintUpdateState
		}
		if sprintUpdateStart != "" {
			fields["startDate"] = sprintUpdateStart
		}
		if sprintUpdateEnd != "" {
			fields["endDate"] = sprintUpdateEnd
		}
		sprint, err := cl.UpdateSprint(context.Background(), sprintUpdateID, fields)
		if err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Updated sprint %q (ID: %d)", sprint.Name, sprint.ID))
		return nil
	},
}

// init registers flags for sprintUpdateCmd.
func init() {
	sprintUpdateCmd.Flags().IntVar(&sprintUpdateID, "id", 0, "Sprint ID to update (required)")
	_ = sprintUpdateCmd.MarkFlagRequired("id")
	sprintUpdateCmd.Flags().StringVar(&sprintUpdateName, "name", "", "New sprint name")
	sprintUpdateCmd.Flags().StringVar(&sprintUpdateGoal, "goal", "", "New sprint goal")
	sprintUpdateCmd.Flags().StringVar(&sprintUpdateState, "state", "",
		"New sprint state: active or closed")
	sprintUpdateCmd.Flags().StringVar(&sprintUpdateStart, "start", "", "New start date/time (ISO 8601)")
	sprintUpdateCmd.Flags().StringVar(&sprintUpdateEnd, "end", "", "New end date/time (ISO 8601)")
}

// -----------------------------------------------------------------------
// board sprint issues
// -----------------------------------------------------------------------

// sprintIssuesID is the sprint ID whose issues are to be listed.
var sprintIssuesID int

var sprintIssuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "List issues in a sprint",
	Long: `List all issues belonging to a specific sprint.

API: GET /rest/agile/1.0/sprint/{sprintId}/issue
Ref: https://developer.atlassian.com/cloud/jira/software/rest/api-group-sprint/#api-rest-agile-1-0-sprint-sprintid-issue-get

Example:
  jcli board sprint issues --id 5`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		result, err := cl.GetSprintIssues(context.Background(), sprintIssuesID)
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(result)
		}
		fmt.Fprintf(output.Stdout(), "Sprint %d: %d issues\n", sprintIssuesID, result.Total)
		var rows [][]string
		for _, issue := range result.Issues {
			assignee := ""
			if issue.Fields.Assignee != nil {
				assignee = issue.Fields.Assignee.DisplayName
			}
			rows = append(rows, []string{
				issue.Key,
				issue.Fields.IssueType.Name,
				issue.Fields.Status.Name,
				assignee,
				output.Truncate(issue.Fields.Summary, 60),
			})
		}
		p.Table([]string{"KEY", "TYPE", "STATUS", "ASSIGNEE", "SUMMARY"}, rows)
		return nil
	},
}

// init registers flags for sprintIssuesCmd and wires all sprint sub-commands onto sprintCmd.
func init() {
	sprintIssuesCmd.Flags().IntVar(&sprintIssuesID, "id", 0, "Sprint ID (required)")
	_ = sprintIssuesCmd.MarkFlagRequired("id")

	sprintCmd.AddCommand(sprintListCmd, sprintCreateCmd, sprintUpdateCmd, sprintIssuesCmd)
}
