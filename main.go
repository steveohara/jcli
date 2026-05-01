// Command jcli is a comprehensive CLI for the Jira REST API v2.
//
// It supports both Jira Cloud and Jira Server/Data Center instances and covers
// the full range of issue, project, board, sprint and user operations exposed
// by the API.
//
// Configuration is loaded from a .jcli.properties file, environment variables
// and command-line flags (in that order of precedence, highest last).
//
// See README.md for full documentation and examples.
package main

import (
	"github.com/steveohara/jcli/cmd"
	"github.com/steveohara/jcli/cmd/board"
	"github.com/steveohara/jcli/cmd/issue"
	"github.com/steveohara/jcli/cmd/meta"
	"github.com/steveohara/jcli/cmd/project"
	"github.com/steveohara/jcli/cmd/user"
)

func main() {
	// Register all top-level sub-commands
	cmd.RootCmd.AddCommand(
		issue.IssueCmd,
		project.ProjectCmd,
		board.BoardCmd,
		user.UserCmd,
		meta.MetaCmd,
	)

	cmd.Execute()
}
