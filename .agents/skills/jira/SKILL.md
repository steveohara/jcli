---
name: jira
description: Interact with Jira using the jcli command-line tool. Use this skill for any Jira tasks: finding and viewing projects, searching issues with JQL, creating/reading/updating/deleting issues, managing custom fields, viewing sprint boards, transitioning issue status, logging work, managing comments, and discovering field metadata and definitions.
license: Apache-2.0
compatibility: opencode
---

# Jira Skill (jcli)

This skill covers everything a Jira user needs via the `jcli` command-line tool —
a Go-based CLI for the Jira REST API v2 that supports both Jira Cloud and Jira
Server/Data Center.

---

## Prerequisites & Configuration

`jcli` resolves config from three sources (later overrides earlier):

1. **Config file** — `~/.config/jcli/config.properties` (Java-style `key=value`)
2. **Environment variables** — `JIRA_SERVER`, `JIRA_PROJECT`, `JIRA_API_TOKEN`
3. **CLI flags** — `--server`, `--project`, `--token`

Minimal config file:

```properties
server=https://myorg.atlassian.net
token=my-personal-access-token
project=PROJ
output=table
```

For Jira Cloud, generate a token at https://id.atlassian.com/manage-profile/security/api-tokens.
For Jira Server/Data Center, use a Personal Access Token from your profile settings.

**Global flags** available on every command:

| Flag | Description |
|------|-------------|
| `--server` | Jira base URL |
| `--token` | Personal Access Token |
| `--project` | Default project key |
| `-o, --output` | `table` (default), `json`, or `plain` |
| `--insecure` | Skip TLS verification (self-hosted with self-signed certs) |
| `-v, --verbose` | Print HTTP request/response to stderr |
| `--debug` | Print equivalent curl command instead of executing |
| `--timeout` | HTTP timeout in seconds (default: 30) |

---

## Output Formats

All commands support three formats:

| Format | Description | Best for |
|--------|-------------|----------|
| `table` | ASCII table (default) | Human reading |
| `json` | Full API response, pretty-printed | Scripting, custom fields |
| `plain` | Tab-separated values | `awk`, `cut`, shell pipelines |

Use `--output json` whenever you need custom fields or the full API response.

---

## Projects

### List all projects

```bash
jcli project list
jcli project list --output json
```

### Get project details

```bash
jcli project get PROJ
jcli project get PROJ --output json
```

### Create a project

Project type keys: `software`, `service_desk`, `business`

```bash
jcli project create --key MYPROJ --name "My Project" --type software
jcli project create --key DEMO --name "Demo Project" --type business \
    --description "A demo project" --lead "<accountId>"
```

### Update a project

```bash
jcli project update PROJ --name "New Name"
jcli project update PROJ --description "Updated description"
```

### Delete a project

**Irreversible — deletes the project and all its issues.**

```bash
jcli project delete PROJ
```

### Manage versions (releases)

```bash
jcli project version list PROJ
jcli project version create PROJ --name "v1.0" --release-date "2024-03-01"
jcli project version update --id 10010 --released
jcli project version delete --id 10010
```

### Manage components

```bash
jcli project component list PROJ
jcli project component create PROJ --name "Backend" --description "Backend services"
jcli project component update --id 10020 --name "API Layer"
jcli project component delete --id 10020
```

---

## Issues

### Get a single issue

```bash
jcli issue get PROJ-42
# Specific fields only
jcli issue get PROJ-42 --fields summary,status,assignee,priority
# Full JSON with all fields including custom fields
jcli issue get PROJ-42 --output json
# Include fields with empty/null values
jcli issue get PROJ-42 --all-fields --output json
```

To see custom field values, use `--output json`. Custom fields appear under keys
like `customfield_10014` (story points), `customfield_10016` (sprint), etc.
Use `jcli meta fields` to discover the IDs and names of all custom fields.

### Search issues with JQL

```bash
# Basic search
jcli issue search --jql "project = PROJ AND status = 'In Progress'"

# Assigned to current user, ordered by priority
jcli issue search --jql "assignee = currentUser() ORDER BY priority DESC"

# Active sprint issues
jcli issue search --jql "sprint in openSprints() AND project = PROJ"

# High priority bugs not yet done
jcli issue search --jql "project = PROJ AND type = Bug AND priority = High AND status != Done"

# Recently updated issues
jcli issue search --jql "project = PROJ AND updated >= -7d ORDER BY updated DESC"

# Custom field filter (e.g. story points > 3)
jcli issue search --jql "project = PROJ AND cf[10014] > 3"

# Return specific fields
jcli issue search --jql "project = PROJ" --fields summary,status,assignee,priority

# Pagination
jcli issue search --jql "project = PROJ" --max-results 25 --page 2
jcli issue search --jql "project = PROJ" --all --output json   # fetch all pages

# Full JSON with custom fields
jcli issue search --jql "project = PROJ AND status = Open" --output json
```

Common JQL functions:
- `currentUser()` — the authenticated user
- `openSprints()` — currently active sprints
- `closedSprints()` — completed sprints
- `membersOf("group-name")` — members of a group
- `-7d`, `-30d` — relative date expressions

### Create an issue

```bash
# Basic task
jcli issue create --summary "Fix login page"

# Bug with priority
jcli issue create --summary "Login page broken" --type Bug --priority High

# Story with description and labels
jcli issue create --summary "New API endpoint" --type Story --project MYPROJ \
    --description "Implement /api/v2/users endpoint" --labels backend,api

# Sub-task under a parent
jcli issue create --summary "Write unit tests" --type Sub-task --parent PROJ-10

# Full options
jcli issue create \
    --summary "Implement feature X" \
    --type Story \
    --priority Medium \
    --description "Detailed description here" \
    --project PROJ \
    --assignee "5f0d3aef12345678" \
    --labels backend,api \
    --components "10001,10002" \
    --fix-versions "10010" \
    --due-date "2024-03-15"
```

Issue type names (use `jcli meta issue-types` to see all available):
`Bug`, `Story`, `Task`, `Sub-task`, `Epic`

### Update an issue

Only the flags you provide are sent; other fields are left unchanged.

```bash
jcli issue update PROJ-42 --summary "Updated title"
jcli issue update PROJ-42 --priority High --assignee "5f0d3aef12345678"
jcli issue update PROJ-42 --description "New description text"
jcli issue update PROJ-42 --due-date "2024-04-01"
jcli issue update PROJ-42 --labels "backend,reviewed"
```

### Delete an issue

**Irreversible.**

```bash
jcli issue delete PROJ-42
jcli issue delete PROJ-42 --delete-subtasks   # also removes child sub-tasks
```

### Transition issue status (workflow)

```bash
# First, see available transitions for the issue
jcli issue transition list PROJ-42

# Apply a transition by its ID
jcli issue transition apply PROJ-42 --id 31

# Apply with a resolution
jcli issue transition apply PROJ-42 --id 5 --resolution "Fixed"
```

### Assign / unassign

```bash
# Assign — get account IDs with: jcli user search --query "name"
jcli issue assign PROJ-42 --account-id "5f0d3aef12345678"

# Unassign
jcli issue assign PROJ-42 --account-id ""
```

### Comments

```bash
jcli issue comment list PROJ-42
jcli issue comment add PROJ-42 --body "Looking into this now."
jcli issue comment update PROJ-42 --comment-id 10001 --body "Fixed in commit abc123"
jcli issue comment delete PROJ-42 --comment-id 10001
```

### Work logs (time tracking)

Time notation: `2h`, `30m`, `1d`, `2h 30m`

```bash
jcli issue worklog list PROJ-42
jcli issue worklog add PROJ-42 --time-spent "2h" \
    --started "2024-01-15T09:00:00.000+0000" \
    --comment "Bug investigation and fix"
jcli issue worklog delete PROJ-42 --worklog-id 10050
```

### Links between issues

```bash
# See available link types
jcli issue link types

# Create a link
jcli issue link create --type "blocks" --inward PROJ-42 --outward PROJ-50
jcli issue link create --type "relates to" --inward PROJ-1 --outward PROJ-2

# Delete a link
jcli issue link delete --link-id 10000
```

### Attachments

```bash
jcli issue attach add PROJ-42 --file /path/to/screenshot.png
jcli issue attach delete --attachment-id 10100
```

### Votes

```bash
jcli issue vote get PROJ-42
jcli issue vote add PROJ-42
jcli issue vote remove PROJ-42
```

### Watchers

```bash
jcli issue watch list PROJ-42
jcli issue watch add PROJ-42 --account-id "5f0d3aef12345678"
jcli issue watch remove PROJ-42 --account-id "5f0d3aef12345678"
```

---

## Boards & Sprints

Board and sprint commands use the Jira Agile REST API (`/rest/agile/1.0`).

### List boards

```bash
jcli board list
jcli board list --project PROJ
jcli board list --max-results 100 --output json
```

### List sprints on a board

Sprint states: `active`, `future`, `closed`

```bash
jcli board sprint list --board-id 1
jcli board sprint list --board-id 1 --state active
jcli board sprint list --board-id 1 --state future
jcli board sprint list --board-id 1 --state closed
```

### See issues in a sprint

```bash
jcli board sprint issues --id 5
jcli board sprint issues --id 5 --output json
```

### Create a sprint

```bash
jcli board sprint create --board-id 1 --name "Sprint 5"
jcli board sprint create --board-id 1 --name "Sprint 6" \
    --start "2024-02-01T00:00:00.000Z" \
    --end   "2024-02-14T00:00:00.000Z" \
    --goal  "Complete the authentication module"
```

### Update a sprint (name, dates, goal, state)

Valid state transitions: `future` → `active`, `active` → `closed`

```bash
jcli board sprint update --id 5 --name "Sprint 5 (extended)"
jcli board sprint update --id 5 --goal "Ship v2 API"
jcli board sprint update --id 5 --state active     # start the sprint
jcli board sprint update --id 5 --state closed     # close the sprint
```

---

## Users

```bash
# Get the currently authenticated user (useful to find your own accountId)
jcli user myself
jcli user myself --output json

# Look up a user by account ID
jcli user get --account-id "5f0d3aef12345678"

# Search users by name or email
jcli user search --query "john"
jcli user search --query "smith" --max-results 20 --output json
```

---

## Metadata & Field Definitions

These commands help discover valid values before creating or filtering issues.

### Issue types

```bash
jcli meta issue-types
```

Returns: `ID`, `NAME`, `DESCRIPTION`, `SUBTASK` — use `NAME` for `--type` in `issue create`.

### Priorities

```bash
jcli meta priorities
```

Returns: `ID`, `NAME` — use `NAME` for `--priority`.

### Workflow statuses

```bash
jcli meta statuses
```

Returns all statuses across all workflows: `ID`, `NAME`, `CATEGORY`.

### Fields (system and custom)

```bash
jcli meta fields
jcli meta fields --output json
```

Returns every field definition: `ID`, `NAME`, `TYPE`, `CUSTOM`, `NAVIGABLE`.

- **System fields** have IDs like `summary`, `status`, `assignee`, `priority`.
- **Custom fields** have IDs like `customfield_10014`. Find the name-to-ID mapping
  here, then use the ID in `--fields` or JQL `cf[<numeric-id>]` filters.

To discover what custom fields an issue actually has populated:

```bash
jcli issue get PROJ-42 --output json
```

---

## Common Workflows

### Daily standup — see your current work

```bash
jcli issue search \
    --jql "assignee = currentUser() AND status != Done ORDER BY priority DESC"
```

### Find everything in the active sprint for a project

```bash
# Step 1: find the board
jcli board list --project PROJ

# Step 2: find the active sprint (note the sprint ID)
jcli board sprint list --board-id <id> --state active

# Step 3: see all issues in that sprint
jcli board sprint issues --id <sprint-id>

# Alternative: use JQL directly
jcli issue search --jql "project = PROJ AND sprint in openSprints()"
```

### Triage: find unassigned open bugs

```bash
jcli issue search \
    --jql "project = PROJ AND type = Bug AND assignee is EMPTY AND status = Open \
           ORDER BY priority ASC, created ASC"
```

### Move an issue through the workflow

```bash
# List available transitions
jcli issue transition list PROJ-42

# Apply the desired transition
jcli issue transition apply PROJ-42 --id <transition-id>
```

### Export all open issues to JSON (e.g. for reporting)

```bash
jcli issue search \
    --jql "project = PROJ AND status != Done" \
    --all \
    --output json > open-issues.json
```

### Log time on multiple issues from a script

```bash
while IFS=, read -r key hours; do
  jcli issue worklog add "$key" --time-spent "${hours}h" \
      --started "$(date -u +%Y-%m-%dT%H:%M:%S.000+0000)"
done < timelog.csv
```

### Inspect custom fields on a project's issues

```bash
# 1. Find all field definitions and their IDs
jcli meta fields --output json | jq '.[] | select(.custom == true) | {id, name}'

# 2. Retrieve an issue with all custom field values
jcli issue get PROJ-42 --all-fields --output json | jq '.fields | with_entries(select(.key | startswith("customfield")))'

# 3. Search using a custom field (use numeric part of the ID in cf[])
jcli issue search --jql "project = PROJ AND cf[10014] > 5"
```

### Get your account ID (needed for assignee flags)

```bash
jcli user myself --output json | jq -r '.accountId'
```

---

## Tips

- **Always check `--help`**: every command documents its API endpoint, all flags,
  and usage examples — e.g. `jcli issue create --help`.
- **Use `--debug`** to see the equivalent `curl` command before executing,
  useful for verifying what will be sent to the API.
- **Use `--output json`** whenever you need custom fields, since the `table`
  format only shows a curated subset of fields.
- **Use `plain` output** for shell pipelines: `jcli project list --output plain | cut -f1`
  extracts just the project keys.
- **JQL** is the most powerful search tool. Combine `project`, `type`, `status`,
  `assignee`, `sprint`, `priority`, `labels`, `fixVersion`, `cf[]` and date
  functions for precise results.
