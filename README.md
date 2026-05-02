# jcli – Jira CLI Tool

A comprehensive, production-ready command-line interface for the **Jira REST API v2**,
written in Go.

`jcli` supports both **Jira Cloud** (`https://yourorg.atlassian.net`) and
**Jira Server / Data Center** (self-hosted) instances.

---

## Table of Contents

1. [Features](#features)
2. [Installation](#installation)
3. [Configuration](#configuration)
4. [Global Flags](#global-flags)
5. [Command Reference](#command-reference)
   - [issue](#issue-commands)
   - [project](#project-commands)
   - [board & sprint](#board--sprint-commands)
   - [user](#user-commands)
   - [meta](#meta-commands)
6. [Output Formats](#output-formats)
7. [Authentication](#authentication)
8. [Examples](#examples)
9. [AI Agent Skill](#ai-agent-skill)
10. [API Reference](#api-reference)
11. [Contributing](#contributing)

---

## Features

| Category | Operations |
|----------|-----------|
| **Issues** | get, create, update, delete, search (JQL) |
| **Comments** | list, add, update, delete |
| **Transitions** | list, apply |
| **Assignments** | assign / unassign |
| **Work Logs** | list, add, delete |
| **Votes** | get, add, remove |
| **Watchers** | list, add, remove |
| **Attachments** | upload, delete |
| **Issue Links** | list types, create, delete |
| **Projects** | list, get, create, update, delete |
| **Versions** | list, create, update, delete |
| **Components** | list, create, update, delete |
| **Boards** | list |
| **Sprints** | list, create, update, list issues |
| **Users** | get, myself, search |
| **Metadata** | issue types, priorities, statuses, fields |

Additional capabilities:

- **Three output formats**: `table` (default), `json`, `plain` (tab-separated)
- **Cascading configuration**: properties file → environment variables → CLI flags
- **Verbose mode**: full HTTP request/response tracing to stderr
- **TLS skip verification**: for self-hosted instances with self-signed certificates
- **Comprehensive help**: every command documents its API mapping, parameters and examples

---

## Installation

### Homebrew (macOS and Linux)

```bash
brew install steveohara/jcli/jcli
```

To update to the latest release:

```bash
brew upgrade steveohara/jcli/jcli
```

### Build from source

```bash
git clone https://github.com/steveohara/jcli.git
cd jcli
go build -o jcli .
# Optional: install to $GOPATH/bin
go install .
```

### Requirements

- Go 1.21 or later

---

## Configuration

`jcli` resolves its configuration from three sources, with the **later sources
overriding earlier ones**:

| Priority | Source | Key names |
|----------|--------|-----------|
| 1 (lowest) | Config file | `server`, `project`, `token`, `output` |
| 2 | Environment variables | `JIRA_SERVER`, `JIRA_PROJECT`, `JIRA_API_TOKEN` |
| 3 (highest) | CLI flags | `--server`, `--project`, `--token`, `--output` |

### Config file

The default config file location is **`~/.config/jcli/config.properties`**.
On systems that set `$XDG_CONFIG_HOME`, the file is looked for at
`$XDG_CONFIG_HOME/jcli/config.properties` instead.

Use the `--config` flag to load a config file from any other path:

```bash
jcli --config /path/to/my-config.properties issue list
```

When `--config` is specified and the file does not exist, `jcli` exits with an
error.  When the default path is used and the file is absent, `jcli` silently
continues and relies on environment variables and CLI flags.

The file uses Java-style `key=value` syntax; lines beginning with `#` or `!`
are comments.

```properties
# ~/.config/jcli/config.properties

# Jira server URL (required)
server=https://myorg.atlassian.net

# Default project key (optional)
project=PROJ

# API token – omit if using JIRA_API_TOKEN env var
token=my-personal-access-token

# Default output format: table, json, or plain (optional, default: table)
output=table

# HTTP request timeout in seconds (optional, default: 30)
# timeout=30
```

### Environment variables

| Variable | Description |
|----------|-------------|
| `JIRA_SERVER` | Jira base URL |
| `JIRA_PROJECT` | Default project key |
| `JIRA_API_TOKEN` | Bearer / Personal Access Token |

---

## Global Flags

These flags are available on every command:

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | | Path to config file (default: `~/.config/jcli/config.properties`) |
| `--server` | | Jira server base URL |
| `--token` | | Personal Access Token |
| `--project` | | Default project key |
| `--output` | `-o` | Output format: `table`, `json`, `plain` |
| `--insecure` | | Skip TLS certificate verification |
| `--verbose` | `-v` | Print HTTP request/response to stderr |
| `--timeout` | | HTTP request timeout in seconds (default: 30) |
| `--help` | `-h` | Show help for any command |

---

## Command Reference

Run `jcli <command> --help` or `jcli <command> <subcommand> --help` at any time
for full flag documentation with API references.

---

### Issue Commands

**API reference**: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issues/

#### `jcli issue get <issue-key>`

Retrieve a single issue by key or numeric ID.

```
API: GET /rest/api/2/issue/{issueIdOrKey}
```

| Flag | Description |
|------|-------------|
| `--fields` | Comma-separated field IDs to include (default: all) |
| `--all-fields` | Include fields with empty or null values in JSON output (default omits them) |

```bash
jcli issue get PROJ-42
jcli issue get PROJ-42 --fields summary,status,assignee --output json
jcli issue get PROJ-42 --all-fields --output json
```

---

#### `jcli issue create`

Create a new issue in a project.

```
API: POST /rest/api/2/issue
```

| Flag | Short | Required | Description |
|------|-------|----------|-------------|
| `--summary` | `-s` | ✓ | Issue title |
| `--type` | `-t` | | Issue type (default: `Task`) |
| `--description` | `-d` | | Description body |
| `--project` | | | Project key (overrides default) |
| `--priority` | | | Priority name (e.g. `High`, `Medium`) |
| `--assignee` | | | Assignee account ID |
| `--labels` | | | Comma-separated labels |
| `--components` | | | Comma-separated component IDs |
| `--fix-versions` | | | Comma-separated version IDs |
| `--due-date` | | | Due date (`YYYY-MM-DD`) |
| `--parent` | | | Parent issue key (for sub-tasks) |

```bash
jcli issue create --summary "Login page broken" --type Bug --priority High
jcli issue create --summary "New API endpoint" --type Story --project MYPROJ \
    --description "Implement /api/v2/users endpoint" --labels backend,api
jcli issue create --summary "Write unit tests" --type Sub-task --parent PROJ-10
```

---

#### `jcli issue update <issue-key>`

Update one or more fields of an existing issue.

```
API: PUT /rest/api/2/issue/{issueIdOrKey}
```

| Flag | Short | Description |
|------|-------|-------------|
| `--summary` | `-s` | New summary |
| `--description` | `-d` | New description |
| `--priority` | | New priority name |
| `--assignee` | | New assignee account ID |
| `--due-date` | | New due date (`YYYY-MM-DD`) |
| `--labels` | | Replace labels (comma-separated) |

```bash
jcli issue update PROJ-42 --summary "Updated title"
jcli issue update PROJ-42 --priority High --assignee "5f0d3aef12345678"
```

---

#### `jcli issue delete <issue-key>`

Permanently delete an issue. **This action cannot be undone.**

```
API: DELETE /rest/api/2/issue/{issueIdOrKey}
```

| Flag | Description |
|------|-------------|
| `--delete-subtasks` | Also delete all child sub-tasks |

```bash
jcli issue delete PROJ-42
jcli issue delete PROJ-42 --delete-subtasks
```

---

#### `jcli issue search`

Search for issues using **Jira Query Language (JQL)**.

```
API: GET /rest/api/2/search
```

| Flag | Description |
|------|-------------|
| `--jql` | JQL query string |
| `--fields` | Comma-separated fields to return |
| `--all-fields` | Include fields with empty or null values in JSON output (default omits them) |
| `--start-at` | Pagination offset (default: 0) |
| `--max-results` | Page size (default: 50) |
| `--page` | Page number to fetch (1-based; computes `startAt` from `--max-results`) |
| `--all` | Fetch all pages automatically (overrides `--page` and `--start-at`) |

JQL documentation: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-jql/

```bash
jcli issue search --jql "project = PROJ AND status = 'In Progress'"
jcli issue search --jql "assignee = currentUser() ORDER BY updated DESC"
jcli issue search --jql "sprint in openSprints() AND priority = High"
jcli issue search --jql "project = PROJ" --max-results 100 --output json
jcli issue search --jql "project = PROJ" --all --output json
jcli issue search --jql "project = PROJ" --page 2 --max-results 25
jcli issue search --jql "project = PROJ" --all-fields --output json
```

---

#### `jcli issue comment`

Manage comments on issues.

```
API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-comments/
```

| Subcommand | API | Description |
|-----------|-----|-------------|
| `list <issue-key>` | GET `/comment` | List all comments |
| `add <issue-key>` | POST `/comment` | Add a comment |
| `update <issue-key>` | PUT `/comment/{id}` | Update a comment |
| `delete <issue-key>` | DELETE `/comment/{id}` | Delete a comment |

```bash
jcli issue comment list PROJ-42
jcli issue comment add PROJ-42 --body "Looking into this now."
jcli issue comment update PROJ-42 --comment-id 10001 --body "Fixed in commit abc123"
jcli issue comment delete PROJ-42 --comment-id 10001
```

---

#### `jcli issue transition`

Manage issue workflow transitions.

```
API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issues/
```

| Subcommand | API | Description |
|-----------|-----|-------------|
| `list <issue-key>` | GET `/transitions` | List available transitions |
| `apply <issue-key>` | POST `/transitions` | Apply a transition |

```bash
jcli issue transition list PROJ-42
jcli issue transition apply PROJ-42 --id 31
jcli issue transition apply PROJ-42 --id 5 --resolution "Fixed"
```

---

#### `jcli issue assign <issue-key>`

Assign or unassign an issue.

```
API: PUT /rest/api/2/issue/{issueIdOrKey}/assignee
```

```bash
jcli issue assign PROJ-42 --account-id "5f0d3aef12345678"
jcli issue assign PROJ-42 --account-id ""    # unassign
```

---

#### `jcli issue worklog`

Manage work log entries (time tracking).

```
API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-worklogs/
```

Time notation examples: `2h`, `30m`, `1d`, `2h 30m`

```bash
jcli issue worklog list PROJ-42
jcli issue worklog add PROJ-42 --time-spent "2h" \
    --started "2024-01-15T09:00:00.000+0000" --comment "Bug investigation"
jcli issue worklog delete PROJ-42 --worklog-id 10050
```

---

#### `jcli issue vote`

Manage votes on issues.

```
API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-votes/
```

```bash
jcli issue vote get PROJ-42
jcli issue vote add PROJ-42
jcli issue vote remove PROJ-42
```

---

#### `jcli issue watch`

Manage watchers on issues.

```
API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-watchers/
```

```bash
jcli issue watch list PROJ-42
jcli issue watch add PROJ-42 --account-id "5f0d3aef12345678"
jcli issue watch remove PROJ-42 --account-id "5f0d3aef12345678"
```

---

#### `jcli issue link`

Manage links between issues.

```
API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-links/
```

```bash
jcli issue link types
jcli issue link create --type "blocks" --inward PROJ-42 --outward PROJ-50
jcli issue link create --type "relates to" --inward PROJ-1 --outward PROJ-2
jcli issue link delete --link-id 10000
```

---

#### `jcli issue attach`

Manage file attachments on issues.

```
API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-attachments/
```

```bash
jcli issue attach add PROJ-42 --file /path/to/screenshot.png
jcli issue attach delete --attachment-id 10100
```

---

### Project Commands

**API reference**: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-projects/

#### `jcli project list`

List all visible projects.

```
API: GET /rest/api/2/project
```

```bash
jcli project list
jcli project list --output json
```

---

#### `jcli project get <project-key>`

Get details of a single project.

```
API: GET /rest/api/2/project/{projectIdOrKey}
```

```bash
jcli project get PROJ
```

---

#### `jcli project create`

Create a new project.

```
API: POST /rest/api/2/project
```

Project type keys: `software`, `service_desk`, `business`

```bash
jcli project create --key MYPROJ --name "My Project" --type software
jcli project create --key DEMO --name "Demo Project" --type business \
    --description "A demo project" --lead "accountId"
```

---

#### `jcli project update <project-key>`

Update project name, description or lead.

```
API: PUT /rest/api/2/project/{projectIdOrKey}
```

```bash
jcli project update PROJ --name "New Project Name"
jcli project update PROJ --description "Updated description"
```

---

#### `jcli project delete <project-key>`

Delete a project and all its issues. **This action cannot be undone.**

```
API: DELETE /rest/api/2/project/{projectIdOrKey}
```

---

#### `jcli project version`

Manage project versions (releases).

```
API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-project-versions/
```

```bash
jcli project version list PROJ
jcli project version create PROJ --name "v1.0" --release-date "2024-03-01"
jcli project version update --id 10010 --released
jcli project version delete --id 10010
```

---

#### `jcli project component`

Manage project components.

```
API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-project-components/
```

```bash
jcli project component list PROJ
jcli project component create PROJ --name "Backend" --description "Backend services"
jcli project component update --id 10020 --name "API Layer"
jcli project component delete --id 10020
```

---

### Board & Sprint Commands

**API reference**: https://developer.atlassian.com/cloud/jira/software/rest/api-group-board/

The board commands use the Jira Agile REST API (`/rest/agile/1.0`).

#### `jcli board list`

List all Agile boards.

```
API: GET /rest/agile/1.0/board
```

```bash
jcli board list
jcli board list --project PROJ
jcli board list --max-results 100
```

---

#### `jcli board sprint list`

List sprints on a board.

```
API: GET /rest/agile/1.0/board/{boardId}/sprint
```

Sprint states: `active`, `future`, `closed`

```bash
jcli board sprint list --board-id 1
jcli board sprint list --board-id 1 --state active
```

---

#### `jcli board sprint create`

Create a new sprint on a board.

```
API: POST /rest/agile/1.0/sprint
```

```bash
jcli board sprint create --board-id 1 --name "Sprint 5"
jcli board sprint create --board-id 1 --name "Sprint 6" \
    --start "2024-02-01T00:00:00.000Z" \
    --end "2024-02-14T00:00:00.000Z" \
    --goal "Complete the authentication module"
```

---

#### `jcli board sprint update`

Update a sprint's name, dates, goal or state.

```
API: PUT /rest/agile/1.0/sprint/{sprintId}
```

Valid state transitions: `future` → `active`, `active` → `closed`

```bash
jcli board sprint update --id 5 --name "Sprint 5 (extended)"
jcli board sprint update --id 5 --state active
jcli board sprint update --id 5 --state closed
```

---

#### `jcli board sprint issues`

List all issues in a sprint.

```
API: GET /rest/agile/1.0/sprint/{sprintId}/issue
```

```bash
jcli board sprint issues --id 5
jcli board sprint issues --id 5 --output json
```

---

### User Commands

**API reference**: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-users/

#### `jcli user myself`

Get the currently authenticated user.

```
API: GET /rest/api/2/myself
```

```bash
jcli user myself
jcli user myself --output json
```

---

#### `jcli user get`

Get a user by account ID.

```
API: GET /rest/api/2/user?accountId={accountId}
```

```bash
jcli user get --account-id "5f0d3aef12345678"
```

---

#### `jcli user search`

Search for users by name or email.

```
API: GET /rest/api/2/user/search?query={query}
```

```bash
jcli user search --query "john"
jcli user search --query "smith" --max-results 20
```

---

### Meta Commands

These commands list metadata useful for discovering valid values for other flags.

#### `jcli meta issue-types`

List all issue types. Use the `NAME` column for `--type` in `jcli issue create`.

```
API: GET /rest/api/2/issuetype
```

```bash
jcli meta issue-types
```

---

#### `jcli meta priorities`

List all priorities. Use the `NAME` column for `--priority`.

```
API: GET /rest/api/2/priority
```

```bash
jcli meta priorities
```

---

#### `jcli meta statuses`

List all workflow statuses.

```
API: GET /rest/api/2/status
```

```bash
jcli meta statuses
```

---

#### `jcli meta fields`

List all field definitions (system and custom).

```
API: GET /rest/api/2/field
```

```bash
jcli meta fields
jcli meta fields --output json
```

---

## Output Formats

All commands support three output formats controlled by the `--output` / `-o` flag
or the `output=` property in `~/.config/jcli/config.properties`:

| Format | Description |
|--------|-------------|
| `table` | ASCII table (default) |
| `json` | Pretty-printed JSON – full API response |
| `plain` | Tab-separated values, suitable for `awk`, `cut`, etc. |

```bash
jcli issue search --jql "project = PROJ" --output json
jcli project list --output plain | cut -f1   # extract keys only
```

---

## Authentication

### Jira Cloud

Generate a **Personal Access Token** (API token) from your Atlassian account:

1. Visit https://id.atlassian.com/manage-profile/security/api-tokens
2. Click **Create API token**
3. Copy the token and set it in `~/.config/jcli/config.properties` or `JIRA_API_TOKEN`

The token is sent as a `Bearer` token in the `Authorization` HTTP header.

### Jira Server / Data Center

Generate a **Personal Access Token** in Jira Server:

1. Click your profile avatar → **Profile**
2. Select **Personal Access Tokens** in the left sidebar
3. Click **Create token**, give it a name and optional expiry

Alternatively, for older Jira Server versions that do not support PATs, you can
base64-encode `username:password` and set it as the token value after modifying
the client to use `Basic` auth (not supported out of the box – PAT is recommended).

### TLS / Self-Signed Certificates

For on-premise installations using self-signed certificates, use `--insecure` to
skip TLS verification:

```bash
jcli --server https://jira.internal --insecure project list
```

Or add `insecure=true` to `~/.config/jcli/config.properties`.

---

## Examples

### Daily workflow

```bash
# See what's assigned to you
jcli issue search --jql "assignee = currentUser() AND status != Done ORDER BY priority DESC"

# View an issue
jcli issue get PROJ-42

# Add a comment
jcli issue comment add PROJ-42 --body "Fixed in PR #123. Ready for review."

# Move to Code Review
jcli issue transition list PROJ-42          # find the transition ID
jcli issue transition apply PROJ-42 --id 31

# Log time
jcli issue worklog add PROJ-42 --time-spent "3h" \
    --started "2024-01-15T14:00:00.000+0000" \
    --comment "Implementation and unit tests"
```

### Scripting / automation

```bash
# Create multiple issues from a file
while IFS=, read -r summary type; do
  jcli issue create --summary "$summary" --type "$type"
done < issues.csv

# Export all open issues to JSON
jcli issue search \
  --jql "project = PROJ AND status != Done" \
  --max-results 500 \
  --output json > open-issues.json

# Get your account ID
jcli user myself --output json | jq -r '.accountId'
```

---

## AI Agent Skill

An [OpenCode](https://opencode.ai) / agent skill definition is included at
[`.agents/skills/jira/SKILL.md`](.agents/skills/jira/SKILL.md).

The skill teaches AI coding agents how to use `jcli` for the full range of
Jira tasks: searching and managing issues, CRUD operations, sprint board
interactions, custom field inspection, workflow transitions, and metadata
discovery.

**Automatic discovery**: any agent tool that supports the `~/.agents/skills/`
or `.agents/skills/` convention (including OpenCode) will pick up the skill
automatically when working inside this repository, or if the skill directory
is installed globally.

---

## API Reference

`jcli` maps directly to the following Atlassian REST APIs:

| API | Documentation |
|-----|--------------|
| Jira REST API v2 (core) | https://developer.atlassian.com/cloud/jira/platform/rest/v2/intro/ |
| Jira Agile REST API | https://developer.atlassian.com/cloud/jira/software/rest/intro/ |
| Jira Software (boards/sprints) | https://developer.atlassian.com/cloud/jira/software/rest/api-group-board/ |
| Issue fields reference | https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-fields/ |
| JQL reference | https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-jql/ |
| API tokens (Cloud) | https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/ |
| Personal Access Tokens (Server) | https://confluence.atlassian.com/enterprise/using-personal-access-tokens-1026032365.html |

---

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make changes and add tests
4. Run `go test ./...` to verify
5. Submit a pull request

### Running tests

```bash
go test ./...
go test -race ./...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

### Building

```bash
make build          # builds bin/jcli for the current platform
make run            # builds and runs bin/jcli
go build -o jcli .  # build directly with go
```

### Publishing a release

The `make release` target handles the full release lifecycle:

1. Compiles cross-platform tarballs (macOS amd64/arm64, Linux amd64/arm64) and a Windows binary into `dist/`
2. Computes SHA256 checksums and regenerates `Formula/jcli.rb`
3. Commits the updated formula, creates the git tag, and pushes both
4. Creates the GitHub release and uploads all assets

```bash
make release VERSION=v1.2.0
```

You can also run the stages individually:

```bash
make dist    VERSION=v1.2.0   # build tarballs only (into dist/)
make formula VERSION=v1.2.0   # dist + update Formula/jcli.rb
```

The formula is regenerated by `scripts/update-formula.sh` — do not edit
`Formula/jcli.rb` by hand.

### Code structure

```
jcli/
├── main.go                      # Entry point
├── Makefile                     # Build, test, release targets
├── Formula/
│   └── jcli.rb                  # Homebrew formula (generated — do not edit)
├── scripts/
│   └── update-formula.sh        # Regenerates Formula/jcli.rb from dist/ checksums
├── cmd/
│   ├── root.go                  # Root command, global flags, config loading
│   ├── issue/issue.go           # All issue sub-commands
│   ├── project/project.go       # All project sub-commands
│   ├── board/board.go           # Board and sprint sub-commands
│   ├── user/user.go             # User sub-commands
│   └── meta/meta.go             # Metadata sub-commands
└── internal/
    ├── config/
    │   ├── config.go            # Configuration loading logic
    │   └── config_test.go       # Config tests
    ├── client/
    │   ├── client.go            # Jira HTTP client and all API methods
    │   └── client_test.go       # Client tests
    └── output/
        └── output.go            # Table/JSON/plain output formatting
```

---

## License

[Apache 2.0](LICENSE)
