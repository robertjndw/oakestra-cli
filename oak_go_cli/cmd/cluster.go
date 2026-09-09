package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/oakestra/oak-go-cli/internal/api"
	oakestra "github.com/oakestra/oakestra-cli/oakestra-go"
)

var clusterCmd = &cobra.Command{
	Use:     "cluster",
	Aliases: []string{"cl"},
	Short:   "Show Oakestra cluster information",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	clusterCmd.AddCommand(clusterListCmd)
	clusterCmd.AddCommand(clusterInfoCmd)

	clusterListCmd.Flags().BoolVarP(&clusterListAll, "all", "a", false,
		"Show all registered clusters, including those not currently connected")
}

var clusterListAll bool

// ─── cluster list ─────────────────────────────────────────────────────────────

var clusterListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "Show connected clusters",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.New()
		if err != nil {
			return err
		}
		clusters, _, err := client.Clusters.List(cmd.Context(), &oakestra.ClusterListOptions{ActiveOnly: !clusterListAll})
		if err != nil {
			return err
		}
		if len(clusters) == 0 {
			if clusterListAll {
				fmt.Println("No clusters registered.")
			} else {
				fmt.Println("No active clusters. Use --all to see all registered clusters.")
			}
			return nil
		}
		printClustersTable(clusters)
		return nil
	},
}

// ─── cluster info ─────────────────────────────────────────────────────────────

var clusterInfoCmd = &cobra.Command{
	Use:               "info <cluster-name-or-id>",
	Aliases:           []string{"i"},
	Short:             "Show detailed information about a specific cluster",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeClusters,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.New()
		if err != nil {
			return err
		}
		cluster, _, err := client.Clusters.FindByNameOrID(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		printClusterDetail(cluster)
		return nil
	},
}

// ─── display helpers ──────────────────────────────────────────────────────────

func clusterStatusDot(active bool) string {
	if active {
		return green("● active")
	}
	return red("○ inactive")
}

// portDisplay renders a cluster port, falling back to a dim placeholder when
// the API didn't return one (StringOrNumber.String() is "" in that case).
func portDisplay(port string) string {
	if port == "" {
		return dim("—")
	}
	return port
}

func printClustersTable(clusters []oakestra.Cluster) {
	headers := []string{"CLUSTER ID", "NAME", "IP", "PORT", "STATUS", "NODES", "CPU%", "MEM%"}
	rows := make([][]string, len(clusters))
	for i, c := range clusters {
		rows[i] = []string{
			colorID(c.ClusterID),
			colorName(c.ClusterName),
			green(c.ClusterIP),
			portDisplay(c.Port.String()),
			clusterStatusDot(c.Active),
			bold(fmt.Sprintf("%d", c.ActiveNodes)),
			fmt.Sprintf("%.1f%%", c.CPUPercent),
			fmt.Sprintf("%.1f%%", c.MemoryPercent),
		}
	}
	printTable(headers, rows)
}

func printClusterDetail(c *oakestra.Cluster) {
	// Format slice fields as comma-separated strings.
	join := func(ss []string) string {
		if len(ss) == 0 {
			return dim("—")
		}
		return strings.Join(ss, ", ")
	}

	lastSeen := dim("—")
	if c.LastModifiedTimestamp > 0 {
		t := time.Unix(int64(c.LastModifiedTimestamp), 0)
		lastSeen = t.Format("2006-01-02 15:04:05")
	}

	printKV([][2]string{
		{"Cluster ID:", colorID(c.ClusterID)},
		{"Name:", colorName(c.ClusterName)},
		{"Candidate name:", c.CandidateName},
		{"IP:", green(c.ClusterIP)},
		{"Port:", portDisplay(c.Port.String())},
		{"Location:", c.ClusterLocation},
		{"Status:", clusterStatusDot(c.Active)},
		{"Active nodes:", bold(fmt.Sprintf("%d", c.ActiveNodes))},
		{"", ""},
		{"vCPUs:", fmt.Sprintf("%d vCPUs", c.VCPUs)},
		{"CPU usage:", fmt.Sprintf("%.1f%%", c.CPUPercent)},
		{"Memory:", fmt.Sprintf("%d MB total", c.MemoryInMB)},
		{"Memory usage:", fmt.Sprintf("%.1f%%", c.MemoryPercent)},
		{"vGPUs:", fmt.Sprintf("%d vGPUs", c.VGPUs)},
		{"GPU usage:", fmt.Sprintf("%.1f%%", c.GPUPercent)},
		{"GPU temp:", fmt.Sprintf("%.1f°C", c.GPUTemp)},
		{"vRAM:", fmt.Sprintf("%d vRAM", c.VRAM)},
		{"VRAM usage:", fmt.Sprintf("%.1f%%", c.VRAMPercent)},
		{"", ""},
		{"Virtualization:", join(c.Virtualization)},
		{"CSI drivers:", join(c.CSIDrivers)},
		{"GPU drivers:", join(c.GPUDrivers)},
		{"Supported addons:", join(c.SupportedAddons)},
		{"", ""},
		{"Last seen:", lastSeen},
	})
}
