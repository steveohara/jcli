// Package project implements project-related sub-commands for jcli.
package project

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/steveohara/jcli/cmd"
	"github.com/steveohara/jcli/internal/client"
	"github.com/steveohara/jcli/internal/output"
)

// ProjectCmd is the parent command for all project operations.
var ProjectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage Jira projects",
	Long: `Commands for listing, getting, creating, updating and deleting Jira projects,
along with managing their versions and components.

API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-projects/`,
}

// init registers all project sub-commands on ProjectCmd.
func init() {
	ProjectCmd.AddCommand(
		listCmd,
		getCmd,
		createCmd,
		updateCmd,
		deleteCmd,
		versionCmd,
		componentCmd,
	)
}

// -----------------------------------------------------------------------
// project list
// -----------------------------------------------------------------------

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	Long: `List all Jira projects that are visible to the currently authenticated user.

API: GET /rest/api/2/project
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-projects/#api-rest-api-2-project-get

Examples:
  jcli project list
  jcli project list --output json`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		projects, err := cl.GetProjects(context.Background())
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(projects)
		}
		var rows [][]string
		for _, proj := range projects {
			lead := ""
			if proj.Lead != nil {
				lead = proj.Lead.DisplayName
			}
			rows = append(rows, []string{proj.Key, proj.Name, proj.ProjectType, lead})
		}
		p.Table([]string{"KEY", "NAME", "TYPE", "LEAD"}, rows)
		return nil
	},
}

// -----------------------------------------------------------------------
// project get
// -----------------------------------------------------------------------

var getCmd = &cobra.Command{
	Use:   "get <project-key>",
	Short: "Get details of a project",
	Long: `Retrieve details of a single Jira project by its key or numeric ID.

API: GET /rest/api/2/project/{projectIdOrKey}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-projects/#api-rest-api-2-project-projectidorkey-get

Examples:
  jcli project get PROJ
  jcli project get PROJ --output json`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		proj, err := cl.GetProject(context.Background(), args[0])
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(proj)
		}
		lead := ""
		if proj.Lead != nil {
			lead = proj.Lead.DisplayName
		}
		p.KV([][]string{
			{"Key", proj.Key},
			{"ID", proj.ID},
			{"Name", proj.Name},
			{"Type", proj.ProjectType},
			{"Lead", lead},
			{"Description", proj.Description},
		})
		return nil
	},
}

// -----------------------------------------------------------------------
// project create
// -----------------------------------------------------------------------

// createKey through createAssignee are the flag variables for "project create".
var (
	createKey         string
	createName        string
	createDescription string
	createType        string
	createLead        string
	createAssignee    string
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new project",
	Long: `Create a new Jira project.

The project type key determines the flavour of the project:
  software   – Jira Software project (Scrum / Kanban)
  service_desk – Jira Service Management project
  business   – Jira Work Management project

API: POST /rest/api/2/project
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-projects/#api-rest-api-2-project-post

Examples:
  jcli project create --key MYPROJ --name "My Project" --type software
  jcli project create --key DEMO --name "Demo" --type business --lead "accountId"`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		req := &client.CreateProjectRequest{
			Key:            createKey,
			Name:           createName,
			Description:    createDescription,
			ProjectTypeKey: createType,
			LeadAccountID:  createLead,
			AssigneeType:   createAssignee,
		}
		proj, err := cl.CreateProject(context.Background(), req)
		if err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Created project %s (ID: %s)", proj.Key, proj.ID))
		_ = cfg
		return nil
	},
}

// init registers flags for createCmd.
func init() {
	createCmd.Flags().StringVar(&createKey, "key", "", "Project key (unique, uppercase letters, required)")
	_ = createCmd.MarkFlagRequired("key")
	createCmd.Flags().StringVar(&createName, "name", "", "Project name (required)")
	_ = createCmd.MarkFlagRequired("name")
	createCmd.Flags().StringVar(&createDescription, "description", "", "Project description")
	createCmd.Flags().StringVar(&createType, "type", "software",
		"Project type key: software, service_desk, or business")
	createCmd.Flags().StringVar(&createLead, "lead", "", "Account ID of the project lead")
	createCmd.Flags().StringVar(&createAssignee, "assignee-type", "UNASSIGNED",
		"Default assignee type: UNASSIGNED or PROJECT_LEAD")
}

// -----------------------------------------------------------------------
// project update
// -----------------------------------------------------------------------

// updateName through updateLead are the flag variables for "project update".
var (
	updateName        string
	updateDescription string
	updateLead        string
)

var updateCmd = &cobra.Command{
	Use:   "update <project-key>",
	Short: "Update a project",
	Long: `Update the name, description or lead of a Jira project.

Only the flags you provide are sent; all other fields are left unchanged.

API: PUT /rest/api/2/project/{projectIdOrKey}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-projects/#api-rest-api-2-project-projectidorkey-put

Examples:
  jcli project update PROJ --name "New Name"
  jcli project update PROJ --description "Better description"`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		fields := make(map[string]interface{})
		if updateName != "" {
			fields["name"] = updateName
		}
		if updateDescription != "" {
			fields["description"] = updateDescription
		}
		if updateLead != "" {
			fields["leadAccountId"] = updateLead
		}
		if len(fields) == 0 {
			return fmt.Errorf("no update fields specified")
		}
		proj, err := cl.UpdateProject(context.Background(), args[0], fields)
		if err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Updated project %s", proj.Key))
		return nil
	},
}

// init registers flags for updateCmd.
func init() {
	updateCmd.Flags().StringVar(&updateName, "name", "", "New project name")
	updateCmd.Flags().StringVar(&updateDescription, "description", "", "New project description")
	updateCmd.Flags().StringVar(&updateLead, "lead", "", "New project lead account ID")
}

// -----------------------------------------------------------------------
// project delete
// -----------------------------------------------------------------------

var deleteCmd = &cobra.Command{
	Use:   "delete <project-key>",
	Short: "Delete a project",
	Long: `Permanently delete a Jira project and all its issues. This cannot be undone.

API: DELETE /rest/api/2/project/{projectIdOrKey}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-projects/#api-rest-api-2-project-projectidorkey-delete

Example:
  jcli project delete OLDPROJ`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		if err := cl.DeleteProject(context.Background(), args[0]); err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Deleted project %s", args[0]))
		return nil
	},
}

// -----------------------------------------------------------------------
// project version
// -----------------------------------------------------------------------

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Manage project versions (releases)",
	Long: `Commands for listing, creating, updating and deleting versions (releases) of a
Jira project.

API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-project-versions/`,
}

var versionListCmd = &cobra.Command{
	Use:   "list <project-key>",
	Short: "List versions of a project",
	Long: `Retrieve all versions (releases) for a Jira project.

API: GET /rest/api/2/project/{projectIdOrKey}/versions
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-project-versions/#api-rest-api-2-project-projectidorkey-versions-get

Example:
  jcli project version list PROJ`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		versions, err := cl.GetVersions(context.Background(), args[0])
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(versions)
		}
		var rows [][]string
		for _, v := range versions {
			rows = append(rows, []string{
				v.ID, v.Name,
				fmt.Sprintf("%v", v.Released),
				fmt.Sprintf("%v", v.Archived),
				v.ReleaseDate,
			})
		}
		p.Table([]string{"ID", "NAME", "RELEASED", "ARCHIVED", "RELEASE DATE"}, rows)
		return nil
	},
}

// versionName through versionArchived are the flag variables for version sub-commands.
var (
	versionName        string
	versionDescription string
	versionReleaseDate string
	versionIDFlag      string
	versionReleased    bool
	versionArchived    bool
)

var versionCreateCmd = &cobra.Command{
	Use:   "create <project-key>",
	Short: "Create a project version",
	Long: `Create a new version (release) for a Jira project.

API: POST /rest/api/2/version
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-project-versions/#api-rest-api-2-version-post

Examples:
  jcli project version create PROJ --name "v1.0" --release-date "2024-03-01"
  jcli project version create PROJ --name "v2.0" --description "Major release"`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		v, err := cl.CreateVersion(context.Background(), args[0], versionName, versionDescription, versionReleaseDate)
		if err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Created version %s (ID: %s)", v.Name, v.ID))
		return nil
	},
}

var versionUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a project version",
	Long: `Update the name, description, release date or status of a project version.

API: PUT /rest/api/2/version/{id}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-project-versions/#api-rest-api-2-version-id-put

Examples:
  jcli project version update --id 10010 --name "v1.0.1"
  jcli project version update --id 10010 --released`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		fields := make(map[string]interface{})
		if versionName != "" {
			fields["name"] = versionName
		}
		if versionDescription != "" {
			fields["description"] = versionDescription
		}
		if versionReleaseDate != "" {
			fields["releaseDate"] = versionReleaseDate
		}
		fields["released"] = versionReleased
		fields["archived"] = versionArchived
		v, err := cl.UpdateVersion(context.Background(), versionIDFlag, fields)
		if err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Updated version %s", v.Name))
		return nil
	},
}

var versionDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a project version",
	Long: `Delete a version from a project.

API: DELETE /rest/api/2/version/{id}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-project-versions/#api-rest-api-2-version-id-delete

Example:
  jcli project version delete --id 10010`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		if err := cl.DeleteVersion(context.Background(), versionIDFlag); err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Deleted version %s", versionIDFlag))
		return nil
	},
}

// init registers flags for version sub-commands and wires them onto versionCmd.
func init() {
	versionCreateCmd.Flags().StringVar(&versionName, "name", "", "Version name (required)")
	_ = versionCreateCmd.MarkFlagRequired("name")
	versionCreateCmd.Flags().StringVar(&versionDescription, "description", "", "Version description")
	versionCreateCmd.Flags().StringVar(&versionReleaseDate, "release-date", "", "Release date in YYYY-MM-DD format")

	versionUpdateCmd.Flags().StringVar(&versionIDFlag, "id", "", "Version ID to update (required)")
	_ = versionUpdateCmd.MarkFlagRequired("id")
	versionUpdateCmd.Flags().StringVar(&versionName, "name", "", "New version name")
	versionUpdateCmd.Flags().StringVar(&versionDescription, "description", "", "New description")
	versionUpdateCmd.Flags().StringVar(&versionReleaseDate, "release-date", "", "New release date (YYYY-MM-DD)")
	versionUpdateCmd.Flags().BoolVar(&versionReleased, "released", false, "Mark the version as released")
	versionUpdateCmd.Flags().BoolVar(&versionArchived, "archived", false, "Mark the version as archived")

	versionDeleteCmd.Flags().StringVar(&versionIDFlag, "id", "", "Version ID to delete (required)")
	_ = versionDeleteCmd.MarkFlagRequired("id")

	versionCmd.AddCommand(versionListCmd, versionCreateCmd, versionUpdateCmd, versionDeleteCmd)
}

// -----------------------------------------------------------------------
// project component
// -----------------------------------------------------------------------

var componentCmd = &cobra.Command{
	Use:   "component",
	Short: "Manage project components",
	Long: `Commands for listing, creating, updating and deleting components of a
Jira project.

API reference: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-project-components/`,
}

var componentListCmd = &cobra.Command{
	Use:   "list <project-key>",
	Short: "List components of a project",
	Long: `Retrieve all components for a Jira project.

API: GET /rest/api/2/project/{projectIdOrKey}/components
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-project-components/#api-rest-api-2-project-projectidorkey-components-get

Example:
  jcli project component list PROJ`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, cfg := cmd.NewClient()
		components, err := cl.GetComponents(context.Background(), args[0])
		if err != nil {
			return err
		}
		p := output.Default(cfg.OutputFormat)
		if cfg.OutputFormat == output.FormatJSON {
			return p.JSON(components)
		}
		var rows [][]string
		for _, comp := range components {
			lead := ""
			if comp.Lead != nil {
				lead = comp.Lead.DisplayName
			}
			rows = append(rows, []string{comp.ID, comp.Name, comp.Description, lead})
		}
		p.Table([]string{"ID", "NAME", "DESCRIPTION", "LEAD"}, rows)
		return nil
	},
}

// componentName through componentIDFlag are the flag variables for component sub-commands.
var (
	componentName        string
	componentDescription string
	componentIDFlag      string
)

var componentCreateCmd = &cobra.Command{
	Use:   "create <project-key>",
	Short: "Create a project component",
	Long: `Create a new component for a Jira project.

API: POST /rest/api/2/component
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-project-components/#api-rest-api-2-component-post

Example:
  jcli project component create PROJ --name "Backend" --description "Backend services"`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		comp, err := cl.CreateComponent(context.Background(), args[0], componentName, componentDescription)
		if err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Created component %s (ID: %s)", comp.Name, comp.ID))
		return nil
	},
}

var componentUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a project component",
	Long: `Update the name or description of a project component.

API: PUT /rest/api/2/component/{id}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-project-components/#api-rest-api-2-component-id-put

Example:
  jcli project component update --id 10020 --name "Frontend" --description "UI layer"`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		fields := make(map[string]interface{})
		if componentName != "" {
			fields["name"] = componentName
		}
		if componentDescription != "" {
			fields["description"] = componentDescription
		}
		comp, err := cl.UpdateComponent(context.Background(), componentIDFlag, fields)
		if err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Updated component %s", comp.Name))
		return nil
	},
}

var componentDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a project component",
	Long: `Delete a component from a Jira project.

API: DELETE /rest/api/2/component/{id}
Ref: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-project-components/#api-rest-api-2-component-id-delete

Example:
  jcli project component delete --id 10020`,
	RunE: func(c *cobra.Command, args []string) error {
		cl, _ := cmd.NewClient()
		if err := cl.DeleteComponent(context.Background(), componentIDFlag); err != nil {
			return err
		}
		output.Success(fmt.Sprintf("Deleted component %s", componentIDFlag))
		return nil
	},
}

// init registers flags for component sub-commands and wires them onto componentCmd.
func init() {
	componentCreateCmd.Flags().StringVar(&componentName, "name", "", "Component name (required)")
	_ = componentCreateCmd.MarkFlagRequired("name")
	componentCreateCmd.Flags().StringVar(&componentDescription, "description", "", "Component description")

	componentUpdateCmd.Flags().StringVar(&componentIDFlag, "id", "", "Component ID to update (required)")
	_ = componentUpdateCmd.MarkFlagRequired("id")
	componentUpdateCmd.Flags().StringVar(&componentName, "name", "", "New component name")
	componentUpdateCmd.Flags().StringVar(&componentDescription, "description", "", "New component description")

	componentDeleteCmd.Flags().StringVar(&componentIDFlag, "id", "", "Component ID to delete (required)")
	_ = componentDeleteCmd.MarkFlagRequired("id")

	componentCmd.AddCommand(componentListCmd, componentCreateCmd, componentUpdateCmd, componentDeleteCmd)
}
