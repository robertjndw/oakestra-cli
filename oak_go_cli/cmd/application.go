package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oakestra/oak-go-cli/internal/api"
	"github.com/oakestra/oak-go-cli/internal/config"
	oakestra "github.com/oakestra/oakestra-cli/oakestra-go"
)

// applicationCmd is the top-level "application" / "app" / "a" command.
var applicationCmd = &cobra.Command{
	Use:     "application",
	Aliases: []string{"app", "a"},
	Short:   "Manage Oakestra applications",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	applicationCmd.AddCommand(appShowCmd)
	applicationCmd.AddCommand(appCreateCmd)
	applicationCmd.AddCommand(appDeleteCmd)
	applicationCmd.AddCommand(appSlaCmd)

	appCreateCmd.Flags().BoolVarP(&appCreateDeploy, "deploy", "d", false, "Deploy all services after creating the application")
	appCreateCmd.Flags().StringVarP(&appCreateFile, "file", "f", "", "Path to the SLA JSON file (optional; opens interactive picker if omitted)")

	appDeleteCmd.Flags().BoolVarP(&appDeleteSkipConfirm, "yes", "y", false, "Skip confirmation prompt")
}

// ─── flags ────────────────────────────────────────────────────────────────────

var appCreateDeploy bool
var appCreateFile string
var appDeleteSkipConfirm bool

// ─── app show ─────────────────────────────────────────────────────────────────

var appShowCmd = &cobra.Command{
	Use:     "show",
	Aliases: []string{"s", "list", "ls"},
	Short:   "Show current applications",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.New()
		if err != nil {
			return err
		}
		apps, _, err := client.Applications.List(cmd.Context())
		if err != nil {
			return err
		}
		if len(apps) == 0 {
			fmt.Println("No applications exist yet.")
			return nil
		}
		printApplicationsTable(apps)
		return nil
	},
}

func printApplicationsTable(apps []*oakestra.Application) {
	headers := []string{"APPLICATION ID", "NAME", "NAMESPACE", "DESCRIPTION", "SERVICES"}
	rows := make([][]string, len(apps))
	for i, a := range apps {
		rows[i] = []string{
			colorID(a.ApplicationID),
			colorName(a.ApplicationName),
			italic(a.ApplicationNamespace),
			truncate(a.ApplicationDesc, 40),
			bold(fmt.Sprintf("%d", len(a.Microservices))),
		}
	}
	printTable(headers, rows)
}

// ─── app create ───────────────────────────────────────────────────────────────

var appCreateCmd = &cobra.Command{
	Use:               "create [sla-name]",
	Aliases:           []string{"c"},
	Short:             "Create one or multiple applications from an SLA file",
	ValidArgsFunction: completeSLAFiles,
	Long: `Create applications defined in an SLA JSON file.

SLA file resolution order:
  1. The --file / -f flag (absolute or relative to current directory)
  2. A positional argument — searched in the current directory, then in ~/oak_cli/SLAs/
  3. Interactive picker from ~/oak_cli/SLAs/

Example:
  oak app create                         # interactive picker
  oak app create myapp                   # resolves myapp.json
  oak app create -f ./custom.json        # explicit path
  oak app create edge_gaming -d          # create + deploy all services`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve SLA file path.
		slaPath, err := resolveSLAPath(appCreateFile, firstArg(args))
		if err != nil {
			return err
		}

		// Load and parse the SLA.
		raw, err := os.ReadFile(slaPath)
		if err != nil {
			return fmt.Errorf("reading SLA file: %w", err)
		}
		var sla map[string]interface{}
		if err := json.Unmarshal(raw, &sla); err != nil {
			return fmt.Errorf("parsing SLA file: %w", err)
		}

		appsInSLA := extractAppNames(sla)

		client, err := api.New()
		if err != nil {
			return err
		}
		ctx := cmd.Context()

		fmt.Printf("Creating application(s) from %s …\n", slaPath)
		allApps, _, err := client.Applications.Create(ctx, sla)
		if err != nil {
			return err
		}

		// Filter to only the newly-created apps (same logic as Python).
		var newApps []*oakestra.Application
		for _, a := range allApps {
			for _, name := range appsInSLA {
				if a.ApplicationName == name {
					newApps = append(newApps, a)
					break
				}
			}
		}

		if len(newApps) == 0 {
			fmt.Println("Warning: no newly created applications found in response.")
		} else {
			fmt.Printf("✓ Created %d application(s):\n", len(newApps))
			printApplicationsTable(newApps)
		}

		if appCreateDeploy {
			fmt.Println("\nDeploying all services …")
			for _, a := range newApps {
				for _, svcID := range a.Microservices {
					if _, err := client.Services.DeployInstance(ctx, svcID); err != nil {
						fmt.Fprintf(os.Stderr, "  ✗ deploy %s: %v\n", svcID, api.Hint(err))
					} else {
						fmt.Printf("  ✓ deployed instance for service %s\n", svcID)
					}
				}
			}
		}
		return nil
	},
}

// ─── app delete ───────────────────────────────────────────────────────────────

var appDeleteCmd = &cobra.Command{
	Use:               "delete [app-id-or-name]",
	Aliases:           []string{"d", "del", "rm"},
	Short:             "Delete an application (or all if no ID given)",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeApplications,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.New()
		if err != nil {
			return err
		}
		ctx := cmd.Context()

		if len(args) == 1 {
			app, _, err := client.Applications.ResolveByNameOrID(ctx, args[0])
			if err != nil {
				return err
			}
			if _, err := client.Applications.Delete(ctx, app.ApplicationID); err != nil {
				return err
			}
			fmt.Printf("✓ Deleted application %s (%s)\n", app.ApplicationName, app.ApplicationID)
			return nil
		}

		// Delete all.
		apps, _, err := client.Applications.List(ctx)
		if err != nil {
			return err
		}
		if len(apps) == 0 {
			fmt.Println("No applications exist yet.")
			return nil
		}

		if !appDeleteSkipConfirm {
			what := fmt.Sprintf("all %d applications", len(apps))
			if len(apps) == 1 {
				what = "the active application"
			}
			fmt.Printf("Are you sure you want to delete %s? [y/N] ", what)
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer != "y" && answer != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		for _, a := range apps {
			if _, err := client.Applications.Delete(ctx, a.ApplicationID); err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ delete %s: %v\n", a.ApplicationID, api.Hint(err))
			} else {
				fmt.Printf("  ✓ Deleted application %s (%s)\n", a.ApplicationID, a.ApplicationName)
			}
		}
		return nil
	},
}

// ─── app sla ──────────────────────────────────────────────────────────────────

var appSlaCmd = &cobra.Command{
	Use:               "sla [sla-name]",
	Short:             "Display an available SLA file as formatted JSON",
	ValidArgsFunction: completeSLAFiles,
	Long: `Display the contents of an SLA file.

SLA file resolution follows the same order as 'app create'.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slaPath, err := resolveSLAPath("", firstArg(args))
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(slaPath)
		if err != nil {
			return err
		}
		var pretty interface{}
		if err := json.Unmarshal(raw, &pretty); err != nil {
			return err
		}
		out, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}

// ─── SLA resolution helpers ───────────────────────────────────────────────────

// resolveSLAPath determines the SLA file path from the various sources.
//
// Priority:
//  1. explicit flag (-f)
//  2. positional name — current directory, then ~/oak_cli/SLAs/
//  3. interactive picker from ~/oak_cli/SLAs/
func resolveSLAPath(flagPath, posArg string) (string, error) {
	// 1. Explicit flag.
	if flagPath != "" {
		if !fileExists(flagPath) {
			return "", fmt.Errorf("SLA file not found: %s", flagPath)
		}
		return flagPath, nil
	}

	// Ensure name has .json extension.
	ensureJSON := func(name string) string {
		if !strings.HasSuffix(name, ".json") {
			return name + ".json"
		}
		return name
	}

	// 2. Positional name.
	if posArg != "" {
		name := ensureJSON(posArg)

		// 2a. Current working directory.
		if path, err := filepath.Abs(name); err == nil && fileExists(path) {
			return path, nil
		}

		// 2b. ~/oak_cli/SLAs/
		slaDir := config.SLAFolder()
		path := filepath.Join(slaDir, name)
		if fileExists(path) {
			return path, nil
		}

		return "", fmt.Errorf("SLA file %q not found in current directory or %s", name, slaDir)
	}

	// 3. Interactive picker.
	return pickSLAInteractively()
}

// pickSLAInteractively lists JSON files in ~/oak_cli/SLAs/ and prompts the user.
func pickSLAInteractively() (string, error) {
	slaDir := config.SLAFolder()
	entries, err := os.ReadDir(slaDir)
	if err != nil {
		return "", fmt.Errorf("cannot read SLA directory %s: %w", slaDir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no SLA files found in %s", slaDir)
	}

	fmt.Printf("Available SLA files in %s:\n", slaDir)
	for i, f := range files {
		fmt.Printf("  [%d] %s\n", i+1, f)
	}
	fmt.Print("Select a file (number): ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())
	n, err := strconv.Atoi(choice)
	if err != nil || n < 1 || n > len(files) {
		return "", fmt.Errorf("invalid selection %q", choice)
	}
	return filepath.Join(slaDir, files[n-1]), nil
}

// ─── misc helpers ─────────────────────────────────────────────────────────────

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func firstArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func extractAppNames(sla map[string]interface{}) []string {
	var names []string
	apps, ok := sla["applications"].([]interface{})
	if !ok {
		return names
	}
	for _, a := range apps {
		aMap, ok := a.(map[string]interface{})
		if !ok {
			continue
		}
		if name, ok := aMap["application_name"].(string); ok {
			names = append(names, name)
		}
	}
	return names
}
