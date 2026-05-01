// Package cmd implements the root cobra command and shared flag handling for
// the jcli tool.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/steveohara/jcli/internal/client"
	"github.com/steveohara/jcli/internal/config"
)

// globalFlags holds values parsed from global flags that are inherited by all
// sub-commands.
var globalFlags = &config.Config{}

// RootCmd is the top-level cobra command for jcli.
var RootCmd = &cobra.Command{
	Use:   "jcli",
	Short: "A CLI tool for the Jira REST API v2",
	Long: `jcli is a comprehensive command-line interface for the Jira REST API v2.

It supports both Jira Cloud (https://yourorg.atlassian.net) and Jira Server /
Data Center (self-hosted) instances.

Configuration is loaded from the following sources in order of precedence
(highest last wins):

  1. .jcli.properties file (searched from the current directory upward, then
     the user home directory).
  2. Environment variables: JIRA_SERVER, JIRA_PROJECT, JIRA_API_TOKEN.
  3. Global flags: --server, --project, --token.

Example .jcli.properties file:

  server=https://myorg.atlassian.net
  project=PROJ
  token=my-personal-access-token
  output=table

Getting started:

  # List all projects
  jcli project list

  # Search for issues using JQL
  jcli issue search --jql "project = PROJ AND status = Open"

  # Create an issue
  jcli issue create --summary "Fix login bug" --type Bug

  # View an issue
  jcli issue get PROJ-42

For detailed help on any command run:

  jcli <command> --help
  jcli <command> <subcommand> --help

Full API documentation: https://developer.atlassian.com/cloud/jira/platform/rest/v2/intro/
`,
	SilenceUsage: true,
}

func init() {
	pf := RootCmd.PersistentFlags()
	pf.StringVar(&globalFlags.Server, "server", "",
		"Jira server base URL, e.g. https://myorg.atlassian.net\n"+
			"(env: JIRA_SERVER, or 'server' key in .jcli.properties)")
	pf.StringVar(&globalFlags.Token, "token", "",
		"Personal Access Token for Bearer authentication\n"+
			"(env: JIRA_API_TOKEN, or 'token' key in .jcli.properties)")
	pf.StringVar(&globalFlags.DefaultProject, "project", "",
		"Default Jira project key used when --project is omitted\n"+
			"(env: JIRA_PROJECT, or 'project' key in .jcli.properties)")
	pf.StringVarP(&globalFlags.OutputFormat, "output", "o", "",
		`Output format: "table" (default), "json", or "plain"`)
	pf.BoolVar(&globalFlags.Insecure, "insecure", false,
		"Skip TLS certificate verification (for self-signed certs on on-premise installs)")
	pf.BoolVarP(&globalFlags.Verbose, "verbose", "v", false,
		"Print HTTP request/response details to stderr")
}

// NewClient resolves configuration and returns a ready-to-use API client.
// It exits with an error message when required config is missing.
func NewClient() (*client.Client, *config.Config) {
	cfg, err := config.Load(globalFlags)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	return client.New(cfg), cfg
}

// Execute runs the root command.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
