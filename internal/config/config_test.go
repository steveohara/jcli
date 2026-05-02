package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/steveohara/jcli/internal/config"
)

// writeProps writes a config.properties file to dir and returns its path.
func writeProps(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, config.DefaultConfigFile)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write props: %v", err)
	}
	return path
}

func TestLoad_FromPropertiesFile(t *testing.T) {
	dir := t.TempDir()
	path := writeProps(t, dir, "server=https://jira.example.com\ntoken=mytoken\nproject=PROJ\n")

	t.Setenv(config.EnvServer, "")
	t.Setenv(config.EnvToken, "")
	t.Setenv(config.EnvProject, "")

	cfg, err := config.Load(&config.Config{ConfigFile: path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server != "https://jira.example.com" {
		t.Errorf("server = %q, want %q", cfg.Server, "https://jira.example.com")
	}
	if cfg.Token != "mytoken" {
		t.Errorf("token = %q, want %q", cfg.Token, "mytoken")
	}
	if cfg.DefaultProject != "PROJ" {
		t.Errorf("project = %q, want %q", cfg.DefaultProject, "PROJ")
	}
}

func TestLoad_TrailingSlashStripped(t *testing.T) {
	dir := t.TempDir()
	path := writeProps(t, dir, "server=https://jira.example.com/\ntoken=t\n")

	t.Setenv(config.EnvServer, "")
	t.Setenv(config.EnvToken, "")

	cfg, err := config.Load(&config.Config{ConfigFile: path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server != "https://jira.example.com" {
		t.Errorf("server = %q, want no trailing slash", cfg.Server)
	}
}

func TestLoad_EnvVarOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := writeProps(t, dir, "server=https://file.example.com\ntoken=filetoken\n")

	t.Setenv(config.EnvServer, "https://env.example.com")
	t.Setenv(config.EnvToken, "envtoken")

	cfg, err := config.Load(&config.Config{ConfigFile: path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server != "https://env.example.com" {
		t.Errorf("server = %q, want env value", cfg.Server)
	}
	if cfg.Token != "envtoken" {
		t.Errorf("token = %q, want env value", cfg.Token)
	}
}

func TestLoad_CLIOverridesEnv(t *testing.T) {
	dir := t.TempDir()
	path := writeProps(t, dir, "server=https://file.example.com\ntoken=filetoken\n")

	t.Setenv(config.EnvServer, "https://env.example.com")

	overrides := &config.Config{
		ConfigFile: path,
		Server:     "https://cli.example.com",
		Token:      "clitoken",
	}
	cfg, err := config.Load(overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server != "https://cli.example.com" {
		t.Errorf("server = %q, want CLI value", cfg.Server)
	}
	if cfg.Token != "clitoken" {
		t.Errorf("token = %q, want CLI value", cfg.Token)
	}
}

func TestLoad_MissingServerReturnsError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // prevent reading real ~/.config/jcli/config.properties
	t.Setenv(config.EnvServer, "")
	t.Setenv(config.EnvToken, "")

	_, err := config.Load(nil)
	if err == nil {
		t.Error("expected error when server is missing")
	}
}

func TestLoad_MissingTokenReturnsError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // prevent reading real ~/.config/jcli/config.properties
	t.Setenv(config.EnvServer, "https://jira.example.com")
	t.Setenv(config.EnvToken, "")

	_, err := config.Load(nil)
	if err == nil {
		t.Error("expected error when token is missing")
	}
}

func TestLoad_CommentsIgnored(t *testing.T) {
	dir := t.TempDir()
	content := `# This is a comment
! Another comment
server=https://jira.example.com
# token=ignored
token=realtoken
`
	path := writeProps(t, dir, content)

	t.Setenv(config.EnvServer, "")
	t.Setenv(config.EnvToken, "")

	cfg, err := config.Load(&config.Config{ConfigFile: path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Token != "realtoken" {
		t.Errorf("token = %q, want %q", cfg.Token, "realtoken")
	}
}

func TestLoad_ExplicitMissingFileReturnsError(t *testing.T) {
	_, err := config.Load(&config.Config{ConfigFile: "/nonexistent/path/config.properties"})
	if err == nil {
		t.Error("expected error when explicit config file does not exist")
	}
}
