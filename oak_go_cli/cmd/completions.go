package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/oakestra/oak-go-cli/internal/api"
	"github.com/oakestra/oak-go-cli/internal/config"
	oakestra "github.com/oakestra/oakestra-cli/oakestra-go"
)

// completionTimeout bounds how long shell completion will wait on the
// System Manager. Completions must never hang the user's shell on an
// unreachable orchestrator.
const completionTimeout = 3 * time.Second

// completeApplications returns shell completions for application names and IDs.
// Each entry is formatted as "value\tdescription" so the shell shows context.
func completeApplications(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	client, err := api.New()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), completionTimeout)
	defer cancel()
	apps, _, err := client.Applications.List(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, a := range apps {
		out = append(out,
			fmt.Sprintf("%s\t%s | %s", a.ApplicationName, a.ApplicationNamespace, a.ApplicationID),
			fmt.Sprintf("%s\t%s | %s", a.ApplicationID, a.ApplicationName, a.ApplicationNamespace),
		)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeServices returns shell completions for service names and IDs.
func completeServices(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	client, err := api.New()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), completionTimeout)
	defer cancel()
	svcs, _, err := client.Services.List(ctx, nil)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, s := range svcs {
		appName := s.GetApplicationName()
		out = append(out,
			fmt.Sprintf("%s\t%s | %s", s.MicroserviceName, appName, s.MicroserviceID),
			fmt.Sprintf("%s\t%s | %s", s.MicroserviceID, s.MicroserviceName, appName),
		)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeServiceThenInstances completes the first arg as a service name/ID and
// the second arg as an instance number of that service.
func completeServiceThenInstances(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return completeServices(cmd, args, toComplete)
	}
	if len(args) == 1 {
		return instanceCompletions(cmd, args[0])
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// instanceCompletions fetches running instances for a given service arg.
func instanceCompletions(cmd *cobra.Command, serviceArg string) ([]string, cobra.ShellCompDirective) {
	client, err := api.New()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), completionTimeout)
	defer cancel()
	svc, _, err := client.Services.ResolveByNameOrID(ctx, serviceArg)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, inst := range svc.InstanceList {
		out = append(out, fmt.Sprintf("%s\t%s | %s",
			strconv.Itoa(inst.InstanceNumber), inst.Status, inst.HostIP))
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeScale handles the three positional args of `oak s scale`:
//
//	arg 0 → "up" or "down"
//	arg 1 → service name / ID
//	arg 2 → count (no completion)
func completeScale(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		return []string{
			"up\tDeploy new instances",
			"down\tUndeploy existing instances",
		}, cobra.ShellCompDirectiveNoFileComp
	case 1:
		return completeServices(cmd, args, toComplete)
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeSLAFiles returns SLA file names (without .json) from ~/oak_cli/SLAs/,
// and also enables file-completion so local .json files in the CWD are offered too.
func completeSLAFiles(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	slaDir := config.SLAFolder()
	entries, err := os.ReadDir(slaDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			name := strings.TrimSuffix(e.Name(), ".json")
			out = append(out, fmt.Sprintf("%s\t%s", name, slaDir))
		}
	}
	// ShellCompDirectiveDefault keeps file completion active for local paths.
	return out, cobra.ShellCompDirectiveDefault
}

// completeClusters returns shell completions for cluster names and IDs.
func completeClusters(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	client, err := api.New()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), completionTimeout)
	defer cancel()
	clusters, _, err := client.Clusters.List(ctx, &oakestra.ClusterListOptions{ActiveOnly: false})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, c := range clusters {
		out = append(out,
			fmt.Sprintf("%s\t%s", c.ClusterName, c.ClusterID),
			fmt.Sprintf("%s\t%s", c.ClusterID, c.ClusterName),
		)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeConfigKeys completes the first arg of `oak config set` with known key names.
func completeConfigKeys(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return []string{
			"system_manager_ip\tIP of the Oakestra root orchestrator",
			"root_orchestrator_address\tAlias for system_manager_ip",
			"cluster_manager_ip\tIP of the Cluster Orchestrator",
			"cluster_name\tName of the local cluster",
			"cluster_location\tLocation of the local cluster",
			"main_oak_repo_path\tPath to the main Oakestra repository",
			"flops_repo_path\tPath to the FLOps addon repository",
		}, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}
