package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oakestra/oak-go-cli/internal/config"
	oakestra "github.com/oakestra/oakestra-cli/oakestra-go"
)

// addonFlopsCmd is the "oak addon flops" command.
var addonFlopsCmd = &cobra.Command{
	Use:     "flops",
	Aliases: []string{"fl"},
	Short:   "Manage the FLOps (Federated Learning Operations) addon",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	addonFlopsCmd.AddCommand(flopsProjectCmd)
	addonFlopsCmd.AddCommand(flopsMockDataCmd)
	addonFlopsCmd.AddCommand(flopsTrackingCmd)
	addonFlopsCmd.AddCommand(flopsResetDatabaseCmd)
	addonFlopsCmd.AddCommand(flopsRestartManagementCmd)
	addonFlopsCmd.AddCommand(flopsClearRegistryCmd)

	flopsProjectCmd.Flags().BoolVar(&flopsProjectShow, "show", false, "Only display the SLA without submitting")
	flopsMockDataCmd.Flags().BoolVar(&flopsMockShow, "show", false, "Only display the SLA without submitting")
}

var (
	flopsProjectShow bool
	flopsMockShow    bool
)

// ─── flops project ────────────────────────────────────────────────────────────

var flopsProjectCmd = &cobra.Command{
	Use:     "project <sla-file>",
	Aliases: []string{"p"},
	Short:   "Start a new FLOps project from an SLA file",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slaPath := args[0]
		if flopsProjectShow {
			return printSLAFile(slaPath)
		}
		sla, err := loadSLA(slaPath)
		if err != nil {
			return err
		}
		return flopsPost(cmd.Context(), "/api/flops/projects", sla,
			fmt.Sprintf("Init new FLOps project for SLA '%s'", slaPath))
	},
}

// ─── flops mock-data ──────────────────────────────────────────────────────────

var flopsMockDataCmd = &cobra.Command{
	Use:     "mock-data <sla-file>",
	Aliases: []string{"m", "mock"},
	Short:   "Deploy a mock-data-provider service from an SLA file",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slaPath := args[0]
		if flopsMockShow {
			return printSLAFile(slaPath)
		}
		sla, err := loadSLA(slaPath)
		if err != nil {
			return err
		}
		return flopsPost(cmd.Context(), "/api/flops/mocks", sla,
			fmt.Sprintf("Init a new FLOps mock data service for SLA '%s'", slaPath))
	},
}

// ─── flops tracking ───────────────────────────────────────────────────────────

var flopsTrackingCmd = &cobra.Command{
	Use:     "tracking [customer-id]",
	Aliases: []string{"t"},
	Short:   "Deploy the Tracking Server and get its URL",
	Long: `Deploy the Tracking Server Service if it is not yet deployed.

Returns the URL of the tracking server for the specified customer (default: Admin).`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		customerID := "Admin"
		if len(args) == 1 {
			customerID = args[0]
		}
		return flopsGet(cmd.Context(), "/api/flops/tracking", map[string]string{"customerID": customerID})
	},
}

// ─── flops reset-database ─────────────────────────────────────────────────────

var flopsResetDatabaseCmd = &cobra.Command{
	Use:     "reset-database [customer-id]",
	Aliases: []string{"redb"},
	Short:   "Reset the FLOps addon database (admin only)",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		customerID := "Admin"
		if len(args) == 1 {
			customerID = args[0]
		}
		return flopsDelete(cmd.Context(), "/api/flops/database", map[string]string{"customerID": customerID})
	},
}

// ─── flops restart-management ─────────────────────────────────────────────────

var flopsRestartManagementCmd = &cobra.Command{
	Use:     "restart-management",
	Aliases: []string{"restart", "re"},
	Short:   "Restart FLOps Management via Docker Compose",
	RunE: func(cmd *cobra.Command, args []string) error {
		composePath, err := flopsComposePath()
		if err != nil {
			return err
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		ip := cfg.SystemManagerIP
		if ip == "" {
			ip = config.DefaultSystemManagerIP
		}

		env := append(os.Environ(),
			"FLOPS_MANAGER_IP="+ip,
			"FLOPS_MQTT_BROKER_IP="+ip,
			"SYSTEM_MANAGER_IP="+ip,
			"FLOPS_IMAGE_REGISTRY_IP="+ip,
			"ARTIFACT_STORE_IP="+ip,
			"BACKEND_STORE_IP="+ip,
		)

		fmt.Println("Restarting FLOps Management (Docker Compose)…")
		down := exec.Command("docker", "compose", "-f", composePath, "down")
		down.Env = env
		down.Stdout = os.Stdout
		down.Stderr = os.Stderr
		if err := down.Run(); err != nil {
			return fmt.Errorf("docker compose down: %w", err)
		}
		up := exec.Command("docker", "compose", "-f", composePath, "up", "--build", "-d")
		up.Env = env
		up.Stdout = os.Stdout
		up.Stderr = os.Stderr
		if err := up.Run(); err != nil {
			return fmt.Errorf("docker compose up: %w", err)
		}
		fmt.Printf("%s FLOps Management restarted.\n", green("✓"))
		return nil
	},
}

// ─── flops clear-registry ─────────────────────────────────────────────────────

var flopsClearRegistryCmd = &cobra.Command{
	Use:   "clear-registry",
	Short: "Clear the FLOps Docker image registry",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := exec.Command("docker", "exec", "flops_image_registry",
			"bash", "-c", "rm -rf /var/lib/registry/*")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("clearing registry: %w", err)
		}
		fmt.Printf("%s Registry cleared.\n", green("✓"))
		return nil
	},
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// flopsBaseURL returns the Root FL Manager URL (system_manager_ip:5072).
func flopsBaseURL(cfg *config.Config) string {
	ip := cfg.SystemManagerIP
	if ip == "" {
		ip = config.DefaultSystemManagerIP
	}
	return fmt.Sprintf("http://%s:5072", ip)
}

// flopsClient builds an oakestra-go client pointed at the FL Manager (port
// 5072) instead of the System Manager (port 10000). It reuses the System
// Manager's bearer token rather than logging in separately, since both
// services accept the same Oakestra credentials.
func flopsClient(ctx context.Context) (*oakestra.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	smClient, err := oakestra.NewClient(
		oakestra.WithBaseURL(cfg.URL()),
		oakestra.WithBasicLogin(cfg.GetUsername(), cfg.GetPassword()),
		oakestra.WithUserAgent("oak-cli"),
	)
	if err != nil {
		return nil, err
	}
	token, err := smClient.Token(ctx)
	if err != nil {
		return nil, err
	}

	return oakestra.NewClient(
		oakestra.WithBaseURL(flopsBaseURL(cfg)),
		oakestra.WithToken(token),
		oakestra.WithUserAgent("oak-cli"),
	)
}

// flopsComposePath returns the path to the FLOps management docker-compose file.
// If flops_repo_path is not configured it asks the user whether to clone the
// repo automatically or set the path manually.
func flopsComposePath() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	if cfg.FlopsRepoPath != "" {
		return filepath.Join(cfg.FlopsRepoPath, "docker", "flops_management.docker_compose.yml"), nil
	}

	fmt.Println("flops_repo_path is not configured.")
	fmt.Println()
	fmt.Println("  [1] Install FLOps management repo automatically into ~/.oakestra/addon-FLOps")
	fmt.Println("  [2] I already have it — set the path with: oak config set flops_repo_path <path>")
	fmt.Print("\nChoose [1/2]: ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())
	if choice != "1" {
		return "", fmt.Errorf("set the path with: oak config set flops_repo_path <path-to-flops-repo>")
	}

	repoPath, err := flopsCloneRepo()
	if err != nil {
		return "", err
	}
	if err := config.Set("flops_repo_path", repoPath); err != nil {
		return "", fmt.Errorf("saving flops_repo_path: %w", err)
	}
	fmt.Printf("%s flops_repo_path set to %s\n", green("✓"), repoPath)
	return filepath.Join(repoPath, "docker", "flops_management.docker_compose.yml"), nil
}

// flopsCloneRepo ensures git is available and clones addon-FLOps into
// ~/.oakestra/addon-FLOps, returning the local path.
func flopsCloneRepo() (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git is not installed — please install git and try again")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	destDir := filepath.Join(home, ".oakestra", "addon-FLOps")

	if info, err := os.Stat(destDir); err == nil && info.IsDir() {
		fmt.Printf("%s Repo already exists at %s — skipping clone.\n", green("✓"), destDir)
		return destDir, nil
	}

	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		return "", fmt.Errorf("creating ~/.oakestra: %w", err)
	}

	fmt.Printf("Cloning addon-FLOps into %s…\n", destDir)
	cmd := exec.Command("git", "clone", "git@github.com:oakestra/addon-FLOps.git", destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone failed: %w", err)
	}
	fmt.Printf("%s Clone complete.\n", green("✓"))
	return destDir, nil
}

// loadSLA reads a JSON SLA file and returns it as a map.
func loadSLA(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading SLA file %q: %w", path, err)
	}
	var sla map[string]interface{}
	if err := json.Unmarshal(data, &sla); err != nil {
		return nil, fmt.Errorf("parsing SLA file %q: %w", path, err)
	}
	return sla, nil
}

// printSLAFile pretty-prints a JSON SLA file to stdout.
func printSLAFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading SLA file %q: %w", path, err)
	}
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("parsing SLA file %q: %w", path, err)
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(pretty))
	return nil
}

// The FLOps addon has no typed methods in oakestra-go, so these go through
// Client.NewRequest/Do directly (the library's escape hatch for endpoints
// it doesn't know about), which still gets us JSON encoding, error
// translation, and, via flopsClient, reuse of the System Manager's token.

func flopsPost(ctx context.Context, endpoint string, body interface{}, action string) error {
	client, err := flopsClient(ctx)
	if err != nil {
		return err
	}
	req, err := client.NewRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return err
	}
	resp, err := client.Do(ctx, req, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	fmt.Printf("%s %s\n", green("✓"), action)
	if len(resp.RawBody) > 0 {
		fmt.Println(string(resp.RawBody))
	}
	return nil
}

func flopsGet(ctx context.Context, endpoint string, params map[string]string) error {
	client, err := flopsClient(ctx)
	if err != nil {
		return err
	}
	req, err := client.NewRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if len(params) > 0 {
		q := req.URL.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}
	resp, err := client.Do(ctx, req, nil)
	if err != nil {
		return fmt.Errorf("GET %s: %w", endpoint, err)
	}
	fmt.Println(string(resp.RawBody))
	return nil
}

func flopsDelete(ctx context.Context, endpoint string, body map[string]string) error {
	client, err := flopsClient(ctx)
	if err != nil {
		return err
	}
	req, err := client.NewRequest(ctx, http.MethodDelete, endpoint, body)
	if err != nil {
		return err
	}
	if _, err := client.Do(ctx, req, nil); err != nil {
		return fmt.Errorf("DELETE %s: %w", endpoint, err)
	}
	fmt.Printf("%s FLOps database reset.\n", green("✓"))
	return nil
}
