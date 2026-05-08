# jcli – Jira CLI Tool

A comprehensive, production-ready command-line interface for the **Jira REST API v2**,
written in Go.

`jcli` supports both **Jira Cloud** (`https://yourorg.atlassian.net`) and
**Jira Server / Data Center** (self-hosted) instances.

---

## Table of Contents

1. [Features](#features)
2. [Installation](#installation)
   - [Homebrew (macOS and Linux)](#homebrew-macos-and-linux)
   - [Windows](#windows)
   - [Build from source](#build-from-source)
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

Because this repository is named `jcli` rather than `homebrew-jcli`, you need
to add the tap explicitly before installing:

```bash
brew tap steveohara/jcli https://github.com/steveohara/jcli
brew install steveohara/jcli/jcli
```

After installation, Homebrew will display a message showing how to activate the
bundled Jira agent skill for AI tools. To do it manually:

```bash
mkdir -p ~/.agents/skills/jira
ln -sf "$(brew --prefix)/share/jcli/SKILL.md" ~/.agents/skills/jira/SKILL.md
```

This symlinks the skill into the global discovery path used by
[OpenCode](https://opencode.ai) and other compatible AI agent tools. Once
linked, any agent session — regardless of which project you are working in —
can use the skill to interact with Jira via `jcli`.

To update to the latest release:

```bash
brew upgrade steveohara/jcli/jcli
```

The symlink points to the Homebrew-managed file, so upgrading automatically
picks up the updated skill without relinking.

### Windows

#### Pre-built binary

Download the latest `jcli-vX.Y.Z-windows-amd64.exe` from the
[Releases](https://github.com/steveohara/jcli/releases/latest) page, rename it
to `jcli.exe`, and move it to a directory on your `PATH` (e.g.
`C:\Users\<you>\bin\`).

To verify the installation, open a new Command Prompt or PowerShell window and run:

```powershell
jcli --version
```

#### Scoop

If you use [Scoop](https://scoop.sh):

```powershell
scoop bucket add steveohara https://github.com/steveohara/jcli
scoop install jcli
```

To update:

```powershell
scoop update jcli
```

#### Agent skill on Windows

After placing `jcli.exe` on your `PATH`, activate the skill for AI agent tools
by copying `SKILL.md` from the release archive into the global discovery path.

Using PowerShell:

```powershell
# Extract SKILL.md from the release zip/tarball, then:
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.agents\skills\jira"
Copy-Item "SKILL.md" "$env:USERPROFILE\.agents\skills\jira\SKILL.md"
```

> **Note**: Windows does not support the `~/.agents/skills/` symlink approach
> used on macOS/Linux. Copy the file instead, and re-copy it after each upgrade.

### Build from source

```bash
git clone https://github.com/steveohara/jcli.git
cd jcli
go build -o jcli .
# Optional: install to $GOPATH/bin
go install .
```

On Windows, the output binary will be `jcli.exe`. Use `go build -o jcli.exe .`
to make this explicit.

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

The convenience flags cover the most common fields. Use `--fields` for anything
else, including custom fields. `--fields` values override the convenience flags
when both name the same field.

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
| `--parent` | | | Parent issue key (for sub-tasks; Jira Server/DC only) |
| `--fields` | | | JSON object of field IDs to values (see below) |
| `--properties` | | | JSON array of entity property objects (see below) |
| `--history` | | | JSON object of change history metadata (see below) |

> **Epic Link vs Parent:** On Jira Server/Data Center, use `--parent` only for
> sub-tasks. To link a Story (or other issue type) to an Epic, use the
> `customfield_10200` (Epic Link) field via `--fields`:
> ```bash
> jcli issue create --summary "My story" --type Story \
>     --fields '{"customfield_10200": "PROJ-123"}'
> ```
> Jira Cloud replaces Epic Link with a `parent` relationship and does not use
> `customfield_10200`. Check your instance type with `jcli meta server-info`.

**`--fields`** accepts any field the Jira REST API recognises, keyed by field ID.
Value format depends on the field type:

| Field type | Example value |
|------------|---------------|
| Text | `{"summary": "New title"}` |
| Named object (priority, issuetype) | `{"priority": {"name": "High"}}` |
| ID object (assignee, components) | `{"assignee": {"accountId": "abc123"}}` |
| Select custom field | `{"customfield_31004": {"id": "50628"}}` |
| Multi-select custom field | `{"customfield_10030": [{"id": "10100"}]}` |
| Number custom field | `{"customfield_10014": 5}` |
| Date | `{"duedate": "2024-06-01"}` |

Use `jcli meta fields` to list all field IDs. Use `jcli meta field-allowed-values <fieldId> --issue <KEY>` to find option IDs for select fields.

**`--properties`** sets entity properties on the issue (key/value pairs indexed by Jira but not shown in the UI):
```json
[{"key": "myapp.context", "value": {"buildNumber": 42}}]
```

**`--history`** records context in the issue change history. Useful for automated changes:
```json
{"activityDescription": "Deployed by CI", "actor": {"id": "ci-bot", "type": "automation"}}
```

```bash
jcli issue create --summary "Login page broken" --type Bug --priority High
jcli issue create --summary "New API endpoint" --type Story --project MYPROJ \
    --description "Implement /api/v2/users endpoint" --labels backend,api
jcli issue create --summary "Write unit tests" --type Sub-task --parent PROJ-10

# Set custom fields alongside convenience flags
jcli issue create --summary "Voice outage" --type Bug --priority High \
    --fields '{"customfield_31004":{"id":"50628"},"customfield_23824":{"id":"36274"}}'

# Attach an entity property
jcli issue create --summary "Deploy task" \
    --properties '[{"key":"pipeline.id","value":"build-42"}]'
```

---

#### `jcli issue update <issue-key>`

Update one or more fields of an existing issue. Only the flags you provide are
sent to the API; all other fields are left unchanged.

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
| `--fields` | | JSON object of field IDs to values (same format as `issue create`) |
| `--properties` | | JSON array of entity property objects |
| `--history` | | JSON object of change history metadata |

```bash
jcli issue update PROJ-42 --summary "Updated title"
jcli issue update PROJ-42 --priority High --assignee "5f0d3aef12345678"

# Set a custom field
jcli issue update PROJ-42 --fields '{"customfield_31004":{"id":"50628"}}'

# Mix convenience flags and --fields
jcli issue update PROJ-42 --priority High \
    --fields '{"customfield_23824":{"id":"36274"}}'

# Record change history context
jcli issue update PROJ-42 --summary "Auto-resolved" \
    --history '{"activityDescription":"Resolved by CI","actor":{"id":"ci-bot","type":"automation"}}'
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

#### `jcli meta field-search` _(Jira Cloud only)_

Search field definitions with filtering and pagination. Returns richer metadata
than `meta fields` including description, searcher key, and usage counts.

> **Note:** Uses `GET /rest/api/2/field/search` which is **not available on
> Jira Server / Data Center** (returns HTTP 404). Use `jcli meta fields` instead.

```
API: GET /rest/api/2/field/search  (Cloud only)
```

| Flag | Description |
|------|-------------|
| `--id` | Filter by field ID (repeatable) |
| `--query` | Filter by name/description substring |
| `--type` | Filter by type: `system` or `custom` |
| `--order-by` | Order by: `name`, `screensCount`, `contextsCount`, `projectsCount`, `lastUsed` |
| `--expand` | Include extra data (comma-separated): `screensCount`, `contextsCount`, `lastUsed` |
| `--project-ids` | Filter to fields used in specific project IDs |
| `--start-at` | Pagination offset (default: 0) |
| `--max-results` | Page size (default: 50) |

```bash
jcli meta field-search
jcli meta field-search --type custom
jcli meta field-search --query "sprint" --output json
jcli meta field-search --id customfield_10014 --expand screensCount,contextsCount
jcli meta field-search --order-by name --max-results 100
```

#### `jcli meta field-contexts <fieldId>` _(Jira Cloud only)_

List the contexts a custom field is configured in. Contexts determine which
projects and issue types the field applies to.

> **Note:** Uses `GET /rest/api/2/field/{fieldId}/context` which is **not
> available on Jira Server / Data Center** (returns HTTP 404).

```
API: GET /rest/api/2/field/{fieldId}/context  (Cloud only)
```

| Flag | Description |
|------|-------------|
| `--global` | Show only global (all-project) contexts |
| `--any-issue-type` | Show only contexts that apply to all issue types |
| `--context-id` | Filter by specific context IDs (repeatable) |
| `--start-at` | Pagination offset (default: 0) |
| `--max-results` | Page size (default: 50) |

```bash
jcli meta field-contexts customfield_10014
jcli meta field-contexts customfield_10014 --global
jcli meta field-contexts customfield_10014 --output json
```

#### `jcli meta field-options <fieldId>` _(Jira Cloud only)_

List the allowed option values for a custom select, radio, or checkbox field.
This is the primary way to discover valid values before setting a custom field.

> **Note:** Uses `GET /rest/api/2/field/{fieldId}/context/option` which is
> **not available on Jira Server / Data Center**. Use
> `jcli meta field-allowed-values --issue <key>` instead.

```
API: GET /rest/api/2/field/{fieldId}/context/option  (Cloud only)
```

| Flag | Description |
|------|-------------|
| `--context-id` | Limit options to a specific context ID |
| `--only-options` | Exclude cascading sub-options |
| `--start-at` | Pagination offset (default: 0) |
| `--max-results` | Page size (default: 100) |

```bash
jcli meta field-options customfield_10014
jcli meta field-options customfield_10014 --context-id 10025
jcli meta field-options customfield_10014 --only-options --output json
```

#### `jcli meta field-allowed-values <fieldId>`

List the allowed values for a field using issue edit metadata. Works on both
Jira Cloud and Jira Server / Data Center. Requires an existing issue key as
context (Jira derives allowed values per-issue from its workflow state).

```
API: GET /rest/api/2/issue/{issueKey}/editmeta
```

| Flag | Description |
|------|-------------|
| `--issue` | Issue key to read edit metadata from (required) |

```bash
jcli meta field-allowed-values assignee --issue PROJ-123
jcli meta field-allowed-values customfield_10014 --issue PROJ-123 --output json
```

---

#### `jcli meta resolutions`

List all resolution values. Use the `NAME` column for `--resolution` in
`jcli issue transition apply`.

```
API: GET /rest/api/2/resolution
```

```bash
jcli meta resolutions
jcli meta resolutions --output json
```

---

#### `jcli meta server-info`

Show version and build information for the connected Jira instance, including
`deploymentType` (Cloud vs Server).

```
API: GET /rest/api/2/serverInfo
```

```bash
jcli meta server-info
jcli meta server-info --output json
```

---

#### `jcli meta project-statuses <project-key>`

List workflow statuses available in a project, grouped by issue type. More
precise than `jcli meta statuses` which lists all instance-level statuses.

```
API: GET /rest/api/2/project/{projectIdOrKey}/statuses
```

| Flag | Description |
|------|-------------|
| `--issue-type` | Filter output to a specific issue type name |

```bash
jcli meta project-statuses PROJ
jcli meta project-statuses PROJ --issue-type Bug
jcli meta project-statuses PROJ --output json
```

---

#### `jcli meta link-types`

List all issue link type definitions. Use the `NAME` column for
`--type` in `jcli issue link create`.

```
API: GET /rest/api/2/issueLinkType
```

```bash
jcli meta link-types
jcli meta link-types --output json
```

---

#### `jcli meta configuration`

Show instance-level feature flags (voting, watching, issue linking, sub-tasks,
attachments, time tracking) and time-tracking unit settings.

```
API: GET /rest/api/2/configuration
```

```bash
jcli meta configuration
jcli meta configuration --output json
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

A skill definition for [OpenCode](https://opencode.ai) and compatible AI agent
tools is bundled with `jcli` at
[`.agents/skills/jira/SKILL.md`](.agents/skills/jira/SKILL.md).

### What the skill does

The skill is a structured reference document that teaches an AI agent how to
use `jcli` effectively. Once loaded, the agent can:

- **Find and inspect projects** — list all visible projects, retrieve full
  project details, and manage versions and components.
- **Search issues with JQL** — construct and execute Jira Query Language
  queries with pagination, field selection, and full-page fetches; covering
  common patterns like open sprints, assignee filters, date ranges, and custom
  field expressions (`cf[<id>]`).
- **Full issue CRUD** — create issues with all supported fields (type,
  priority, labels, components, fix versions, parent, due date); update
  individual fields without touching others; delete issues with or without
  sub-tasks.
- **View and control issue detail** — fetch a single issue with selective field
  projection or a full JSON dump including empty/null values; understand how to
  surface custom field values and map their IDs to human-readable names.
- **Workflow transitions** — list the available transitions for any issue and
  apply them by ID, optionally setting a resolution.
- **Sprint board interactions** — list boards, view active/future/closed
  sprints, inspect all issues in a sprint, create new sprints with goals and
  dates, and advance sprint state (`future → active → closed`).
- **Collaboration features** — add, update and delete comments; assign and
  unassign issues; manage watchers; cast and remove votes; create and delete
  issue links; upload and delete attachments.
- **Time tracking** — list, add and delete work log entries using Jira's time
  notation (`2h`, `30m`, `1d 4h`).
- **Metadata discovery** — enumerate all issue types, priorities, workflow
  statuses, and field definitions (system and custom) so the agent always uses
  valid values in commands.
- **Common workflow recipes** — daily standup queries, triage searches,
  multi-page JSON exports, bulk scripting patterns, and custom field
  inspection pipelines using `--output json`.

### Discovery and activation

The skill is picked up automatically based on where it lives:

| Location | When it is active |
|----------|------------------|
| `~/.agents/skills/jira/SKILL.md` | All agent sessions on the machine (global) |
| `.agents/skills/jira/SKILL.md` | Agent sessions inside this repository only |

When installed via Homebrew, the skill file is placed at
`$(brew --prefix)/share/jcli/SKILL.md`. Symlinking it into
`~/.agents/skills/jira/` makes it globally available:

```bash
mkdir -p ~/.agents/skills/jira
ln -sf "$(brew --prefix)/share/jcli/SKILL.md" ~/.agents/skills/jira/SKILL.md
```

When cloning the repository directly, the skill is already present at
`.agents/skills/jira/SKILL.md` and will be discovered automatically by any
compatible agent tool running inside the project directory.

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
