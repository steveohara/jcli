// Package user implements user-related sub-commands for jcli.
package user

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/steveohara/jcli/cmd"
	"github.com/steveohara/jcli/internal/client"
	"github.com/steveohara/jcli/internal/output"
)

// UserCmd is the parent command for user operations.
var UserCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage and search Jira users",
	Long: `Commands for retrieving information about Jira users, including the currently
authenticated user and searching for users by name or email.

API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-users/`,
}

func init() {
	UserCmd.AddCommand(
		getCmd,
		myselfCmd,
		searchCmd,
	)
}

// -----------------------------------------------------------------------
// user get
// -----------------------------------------------------------------------

var getAccountID string

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a user by account ID",
	Long: `Retrieve information about a Jira user by their account ID.

API: GET /rest/api/2/user?accountId={accountId}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-users/#api-rest-api-2-user-get

Example:
  jcli user get --account-id "5f0d3aef12345678"`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		u, err := cl.GetUser(context.Background(), getAccountID)
		if err != nil {
			return err
		}
		return printUser(u, cfg.OutputFormat)
	},
}

func init() {
	getCmd.Flags().StringVar(&getAccountID, "account-id", "", "Account ID of the user to retrieve (required)")
	_ = getCmd.MarkFlagRequired("account-id")
}

// -----------------------------------------------------------------------
// user myself
// -----------------------------------------------------------------------

var myselfCmd = &cobra.Command{
	Use:   "myself",
	Short: "Get the currently authenticated user",
	Long: `Retrieve information about the user authenticated via the current API token.

API: GET /rest/api/2/myself
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-myself/#api-rest-api-2-myself-get

Example:
  jcli user myself`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		u, err := cl.GetCurrentUser(context.Background())
		if err != nil {
			return err
		}
		return printUser(u, cfg.OutputFormat)
	},
}

// -----------------------------------------------------------------------
// user search
// -----------------------------------------------------------------------

var (
	searchQuery      string
	searchMaxResults int
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for users",
	Long: `Search for Jira users by display name, username or email address.

API: GET /rest/api/2/user/search?query={query}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-user-search/#api-rest-api-2-user-search-get

Examples:
  jcli user search --query "john"
  jcli user search --query "smith" --max-results 20`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		users, err := cl.SearchUsers(context.Background(), searchQuery, searchMaxResults)
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(users)
		}
		var rows [][]string
		for _, u := range users {
			rows = append(rows, []string{u.AccountID, u.DisplayName, u.EmailAddress, boolStr(u.Active)})
		}
		p.Table([]string{"ACCOUNT ID", "DISPLAY NAME", "EMAIL", "ACTIVE"}, rows)
		return nil
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchQuery, "query", "", "Name or email to search for (required)")
	_ = searchCmd.MarkFlagRequired("query")
	searchCmd.Flags().IntVar(&searchMaxResults, "max-results", 50,
		"Maximum number of results to return")
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

func printUser(u *client.User, format string) error {
	p := output.Default(format)
	if format == output.FormatJSON {
		return p.JSON(u)
	}
	p.KV([][]string{
		{"Account ID", u.AccountID},
		{"Display Name", u.DisplayName},
		{"Email", u.EmailAddress},
		{"Active", boolStr(u.Active)},
	})
	return nil
}

func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

