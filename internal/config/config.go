// Package config handles loading configuration for jcli from multiple sources:
//  1. A config.properties file at ~/.config/jcli/config.properties by default,
//     or at an explicit path supplied via --config / ConfigFile.
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
	"strconv"
	"strings"
)

const (
	// DefaultConfigFile is the name of the configuration file.
	DefaultConfigFile = "config.properties"

	// DefaultConfigDir is the subdirectory under XDG_CONFIG_HOME (or ~/.config)
	// where the default config file lives.
	DefaultConfigDir = "jcli"

	// EnvToken is the environment variable name for the API token.
	EnvToken = "JIRA_API_TOKEN"
	// EnvServer is the environment variable name for the Jira server URL.
	EnvServer = "JIRA_SERVER"
	// EnvProject is the environment variable name for the default project.
	EnvProject = "JIRA_PROJECT"
)

// Config holds the resolved configuration values used by all commands.
type Config struct {
	// ConfigFile is an optional explicit path to the config file.  When empty
	// the default ~/.config/jcli/config.properties is used.
	ConfigFile string
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
	// Debug prints the equivalent curl command instead of executing the request.
	Debug bool
	// Timeout is the HTTP request timeout in seconds. 0 means use the default (30s).
	Timeout int
}

// DefaultConfigPath returns the default path for the config file:
// $XDG_CONFIG_HOME/jcli/config.properties, falling back to
// ~/.config/jcli/config.properties when XDG_CONFIG_HOME is not set.
func DefaultConfigPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, DefaultConfigDir, DefaultConfigFile)
}

// Load resolves configuration from the properties file, environment variables
// and the supplied overrides (typically parsed from CLI flags).  The precedence
// order from lowest to highest is:
//
//	properties file → environment variables → overrides
//
// When overrides.ConfigFile is non-empty that path is used and an error is
// returned if the file does not exist.  Otherwise the default path is tried
// and a missing file is silently ignored.
func Load(overrides *Config) (*Config, error) {
	cfg := &Config{
		OutputFormat: "table",
	}

	// 1. Properties file
	explicitPath := ""
	if overrides != nil {
		explicitPath = overrides.ConfigFile
	}
	props, err := findAndReadProperties(explicitPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
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
		if overrides.Debug {
			cfg.Debug = true
		}
		if overrides.Timeout > 0 {
			cfg.Timeout = overrides.Timeout
		}
	}

	// Validate required fields
	if cfg.Server == "" {
		return nil, errors.New("Jira server URL is required: set 'server' in " +
			DefaultConfigPath() + ", the JIRA_SERVER environment variable, or use --server")
	}
	if cfg.Token == "" {
		return nil, errors.New("API token is required: set 'token' in " +
			DefaultConfigPath() + ", the JIRA_API_TOKEN environment variable, or use --token")
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
	if v, ok := props["timeout"]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Timeout = n
		}
	}
}

// findAndReadProperties reads the config file at the given explicit path, or
// falls back to the default path.  When an explicit path is provided and the
// file is missing, an error is returned.  When the default path is used and
// the file is missing, an empty map is returned silently.
func findAndReadProperties(explicitPath string) (map[string]string, error) {
	if explicitPath != "" {
		props, err := readPropertiesFile(explicitPath)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", explicitPath, err)
		}
		return props, nil
	}

	defaultPath := DefaultConfigPath()
	if defaultPath == "" {
		return map[string]string{}, nil
	}
	props, err := readPropertiesFile(defaultPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("%s: %w", defaultPath, err)
	}
	return props, nil
}

// readPropertiesFile parses a Java-style .properties file and returns a map of
// key/value pairs.  Lines starting with '#' or '!' are treated as comments.
// Blank lines are ignored.
func readPropertiesFile(path string) (map[string]string, error) {
	f, err := os.Open(path) // #nosec G304 – path comes from the user or a known default
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
