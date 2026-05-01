package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/steveohara/jcli/internal/config"
)

func writeProps(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, config.PropertiesFile)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write props: %v", err)
	}
	return path
}

func TestLoad_FromPropertiesFile(t *testing.T) {
	dir := t.TempDir()
	writeProps(t, dir, "server=https://jira.example.com\ntoken=mytoken\nproject=PROJ\n")

	// Change working directory so the file is found
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(nil)
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
	writeProps(t, dir, "server=https://jira.example.com/\ntoken=t\n")
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	_ = os.Chdir(dir)

	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server != "https://jira.example.com" {
		t.Errorf("server = %q, want no trailing slash", cfg.Server)
	}
}

func TestLoad_EnvVarOverridesFile(t *testing.T) {
	dir := t.TempDir()
	writeProps(t, dir, "server=https://file.example.com\ntoken=filetoken\n")
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	_ = os.Chdir(dir)

	t.Setenv(config.EnvServer, "https://env.example.com")
	t.Setenv(config.EnvToken, "envtoken")

	cfg, err := config.Load(nil)
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
	writeProps(t, dir, "server=https://file.example.com\ntoken=filetoken\n")
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	_ = os.Chdir(dir)

	t.Setenv(config.EnvServer, "https://env.example.com")

	overrides := &config.Config{
		Server: "https://cli.example.com",
		Token:  "clitoken",
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
	dir := t.TempDir()
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	_ = os.Chdir(dir)

	// Clear env vars that might be set in the outer environment
	t.Setenv(config.EnvServer, "")
	t.Setenv(config.EnvToken, "")

	_, err := config.Load(nil)
	if err == nil {
		t.Error("expected error when server is missing")
	}
}

func TestLoad_MissingTokenReturnsError(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	_ = os.Chdir(dir)

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
	writeProps(t, dir, content)
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	_ = os.Chdir(dir)

	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Token != "realtoken" {
		t.Errorf("token = %q, want %q", cfg.Token, "realtoken")
	}
}
