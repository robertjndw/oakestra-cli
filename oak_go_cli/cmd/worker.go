package cmd

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// nodeEngineInstalled returns true if the NodeEngine binary is on the PATH.
func nodeEngineInstalled() bool {
	_, err := exec.LookPath("NodeEngine")
	return err == nil
}

// workerCmd is a transparent passthrough to `sudo NodeEngine`.
// All arguments (including flags) are forwarded verbatim when no named
// subcommand matches. Named subcommands (run, start, stop, status,
// configure-gpu) take precedence over the passthrough.
var workerCmd = &cobra.Command{
	Use:   "worker <run|start|stop|status|configure-gpu|[NodeEngine args...]>",
	Short: "Manage the local NodeEngine worker node",
	Long: `Manage the local NodeEngine worker node.

Named subcommands:
  run             Run nodeengined in the foreground with --debug (troubleshooting)
  start           Start the nodeengine systemd service
  stop            Stop the nodeengine systemd service
  status          Show nodeengine systemd service status
  configure-gpu   Configure NVIDIA Container Toolkit for GPU workloads

Passthrough: any other argument is forwarded directly to 'sudo NodeEngine'.

Examples:
  oak worker run        → sudo nodeengined --debug
  oak worker start      → sudo systemctl start nodeengine
  oak worker stop       → sudo systemctl stop nodeengine
  oak worker status     → systemctl status nodeengine
  oak worker -d         → sudo NodeEngine -d        (passthrough)
  oak worker --help     → sudo NodeEngine --help    (passthrough)`,
	// DisableFlagParsing passes all args (including flags like -d) through
	// to NodeEngine without Cobra intercepting them.
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		c := exec.Command("sudo", append([]string{"NodeEngine"}, args...)...)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

var workerRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run nodeengined in the foreground with --debug (for troubleshooting)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInteractive("sudo", "nodeengined", "--debug")
	},
}

var workerStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the nodeengine systemd service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkSystemd(); err != nil {
			return err
		}
		return runInteractive("sudo", "systemctl", "start", "nodeengine")
	},
}

var workerStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the nodeengine systemd service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkSystemd(); err != nil {
			return err
		}
		return runInteractive("sudo", "systemctl", "stop", "nodeengine")
	},
}

var workerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show nodeengine systemd service status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkSystemd(); err != nil {
			return err
		}
		return runInteractive("systemctl", "status", "nodeengine", "--no-pager")
	},
}

var workerGPUCmd = &cobra.Command{
	Use:   "configure-gpu",
	Short: "Configure NVIDIA Container Toolkit for GPU workloads",
	RunE: func(cmd *cobra.Command, args []string) error {
		return configureGPU()
	},
}

func init() {
	workerCmd.AddCommand(workerRunCmd, workerStartCmd, workerStopCmd, workerStatusCmd, workerGPUCmd)
}
