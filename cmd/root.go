// Package cmd implements the root cobra command and shared flag handling for
// the jcli tool.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/steveohara/jcli/internal/client"
	"github.com/steveohara/jcli/internal/config"
	"github.com/steveohara/jcli/internal/version"
)

// globalFlags holds values parsed from global flags that are inherited by all
// sub-commands.
var globalFlags = &config.Config{}

// showVersion is set by the --version flag.
var showVersion bool

// RootCmd is the top-level cobra command for jcli.
var RootCmd = &cobra.Command{
	Use:          "jcli",
	Short:        "A CLI tool for the Jira REST API v2",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Println(version.Info())
			return nil
		}
		return cmd.Help()
	},
}

func init() {
	RootCmd.Flags().BoolVar(&showVersion, "version", false, "Print build information and exit")

	pf := RootCmd.PersistentFlags()
	pf.StringVar(&globalFlags.ConfigFile, "config", "",
		"Path to config file (default: ~/.config/jcli/config.properties)")
	pf.StringVar(&globalFlags.Server, "server", "",
		"Jira server base URL, e.g. https://myorg.atlassian.net\n"+
			"(env: JIRA_SERVER, or 'server' in config file)")
	pf.StringVar(&globalFlags.Token, "token", "",
		"Personal Access Token for Bearer authentication\n"+
			"(env: JIRA_API_TOKEN, or 'token' in config file)")
	pf.StringVar(&globalFlags.DefaultProject, "project", "",
		"Default Jira project key used when --project is omitted\n"+
			"(env: JIRA_PROJECT, or 'project' in config file)")
	pf.StringVarP(&globalFlags.OutputFormat, "output", "o", "",
		`Output format: "table" (default), "json", or "plain"`)
	pf.BoolVar(&globalFlags.Insecure, "insecure", false,
		"Skip TLS certificate verification (for self-signed certs on on-premise installs)")
	pf.BoolVarP(&globalFlags.Verbose, "verbose", "v", false,
		"Print HTTP request/response details to stderr")
	pf.BoolVar(&globalFlags.Debug, "debug", false,
		"Print the equivalent curl command instead of executing the request")
	pf.IntVar(&globalFlags.Timeout, "timeout", 0,
		"HTTP request timeout in seconds (default: 30)\n"+
			"(or 'timeout' in config file)")
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
