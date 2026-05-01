// Package config handles loading configuration for jcli from multiple sources:
//  1. A .jcli.properties file (Java-style key=value, searched from the current
//     working directory upward and then in the user's home directory).
//  2. Environment variables (JIRA_SERVER, JIRA_PROJECT, JIRA_API_TOKEN).
//  3. Command-line flags (--server, --project, --token) which take highest
//     precedence.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// PropertiesFile is the name of the configuration file.
	PropertiesFile = ".jcli.properties"

	// EnvToken is the environment variable name for the API token.
	EnvToken = "JIRA_API_TOKEN"
	// EnvServer is the environment variable name for the Jira server URL.
	EnvServer = "JIRA_SERVER"
	// EnvProject is the environment variable name for the default project.
	EnvProject = "JIRA_PROJECT"
)

// Config holds the resolved configuration values used by all commands.
type Config struct {
	// Server is the base URL of the Jira instance, e.g. "https://myorg.atlassian.net".
	Server string
	// DefaultProject is the default Jira project key used when --project is omitted.
	DefaultProject string
	// Token is the Bearer / Personal Access Token used for authentication.
	Token string
	// OutputFormat controls how results are printed: "table", "json", or "plain".
	OutputFormat string
	// MaxResults limits list operations; 0 means use the API default.
	MaxResults int
	// Insecure skips TLS certificate verification (useful for on-premise installs
	// with self-signed certificates).
	Insecure bool
	// Verbose enables HTTP request/response tracing.
	Verbose bool
}

// Load resolves configuration from the properties file, environment variables
// and the supplied overrides (typically parsed from CLI flags).  The precedence
// order from lowest to highest is:
//
//	properties file → environment variables → overrides
func Load(overrides *Config) (*Config, error) {
	cfg := &Config{
		OutputFormat: "table",
	}

	// 1. Properties file
	props, err := findAndReadProperties()
	if err != nil {
		return nil, fmt.Errorf("reading properties file: %w", err)
	}
	applyProperties(cfg, props)

	// 2. Environment variables
	if v := os.Getenv(EnvServer); v != "" {
		cfg.Server = v
	}
	if v := os.Getenv(EnvProject); v != "" {
		cfg.DefaultProject = v
	}
	if v := os.Getenv(EnvToken); v != "" {
		cfg.Token = v
	}

	// 3. CLI overrides (non-zero values win)
	if overrides != nil {
		if overrides.Server != "" {
			cfg.Server = overrides.Server
		}
		if overrides.DefaultProject != "" {
			cfg.DefaultProject = overrides.DefaultProject
		}
		if overrides.Token != "" {
			cfg.Token = overrides.Token
		}
		if overrides.OutputFormat != "" {
			cfg.OutputFormat = overrides.OutputFormat
		}
		if overrides.MaxResults != 0 {
			cfg.MaxResults = overrides.MaxResults
		}
		if overrides.Insecure {
			cfg.Insecure = true
		}
		if overrides.Verbose {
			cfg.Verbose = true
		}
	}

	// Validate required fields
	if cfg.Server == "" {
		return nil, errors.New("Jira server URL is required: set 'server' in .jcli.properties, " +
			"the JIRA_SERVER environment variable, or use --server")
	}
	if cfg.Token == "" {
		return nil, errors.New("API token is required: set 'token' in .jcli.properties, " +
			"the JIRA_API_TOKEN environment variable, or use --token")
	}

	// Normalise server URL – strip trailing slash
	cfg.Server = strings.TrimRight(cfg.Server, "/")

	return cfg, nil
}

// applyProperties copies key/value pairs from props into cfg.
func applyProperties(cfg *Config, props map[string]string) {
	if v, ok := props["server"]; ok && v != "" {
		cfg.Server = v
	}
	if v, ok := props["project"]; ok && v != "" {
		cfg.DefaultProject = v
	}
	if v, ok := props["token"]; ok && v != "" {
		cfg.Token = v
	}
	if v, ok := props["output"]; ok && v != "" {
		cfg.OutputFormat = v
	}
}

// findAndReadProperties searches for the properties file starting from the
// current working directory and walking up to the root; it also checks the
// user home directory.  Returns an empty map (not an error) if no file is
// found.
func findAndReadProperties() (map[string]string, error) {
	candidates := propertiesFileCandidates()
	for _, path := range candidates {
		props, err := readPropertiesFile(path)
		if err == nil {
			return props, nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	return map[string]string{}, nil
}

// propertiesFileCandidates returns an ordered list of candidate paths for the
// properties file.
func propertiesFileCandidates() []string {
	var candidates []string

	// Walk up from CWD
	cwd, err := os.Getwd()
	if err == nil {
		dir := cwd
		for {
			candidates = append(candidates, filepath.Join(dir, PropertiesFile))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// Home directory
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, PropertiesFile))
	}

	return candidates
}

// readPropertiesFile parses a Java-style .properties file and returns a map of
// key/value pairs.  Lines starting with '#' or '!' are treated as comments.
// Blank lines are ignored.
func readPropertiesFile(path string) (map[string]string, error) {
	f, err := os.Open(path) // #nosec G304 – path is constructed from known candidate dirs
	if err != nil {
		return nil, err
	}
	defer f.Close()

	props := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue // skip malformed lines
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if key != "" {
			props[key] = value
		}
	}
	return props, scanner.Err()
}
