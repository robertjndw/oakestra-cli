package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	"github.com/oakestra/oak-go-cli/internal/api"
	oakestra "github.com/oakestra/oakestra-cli/oakestra-go"
)

// serviceCmd is the top-level "service" / "s" command.
var serviceCmd = &cobra.Command{
	Use:     "service",
	Aliases: []string{"s"},
	Short:   "Manage Oakestra microservices",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	serviceCmd.AddCommand(svcShowCmd)
	serviceCmd.AddCommand(svcInspectCmd)
	serviceCmd.AddCommand(svcLogsCmd)
	serviceCmd.AddCommand(svcDeployCmd)
	serviceCmd.AddCommand(svcUndeployCmd)
	serviceCmd.AddCommand(svcScaleCmd)

	// show flags
	svcShowCmd.Flags().StringVarP(&svcShowAppID, "app-id", "i", "", "Filter by application ID")
	svcShowCmd.Flags().StringVarP(&svcShowAppName, "app-name", "n", "", "Filter by application name")

	// deploy flags
	svcDeployCmd.Flags().BoolVar(&svcDeployAll, "all", false, "Deploy one new instance for every registered service")

	// inspect / logs follow flag
	svcInspectCmd.Flags().BoolVarP(&svcFollow, "follow", "f", false, "Refresh every 5 seconds")
	svcLogsCmd.Flags().BoolVarP(&svcLogsFollow, "follow", "f", false, "Refresh every 5 seconds")
}

// ─── flags ────────────────────────────────────────────────────────────────────

var (
	svcShowAppID   string
	svcShowAppName string
	svcDeployAll   bool
	svcFollow      bool
	svcLogsFollow  bool
)

// ─── service show ─────────────────────────────────────────────────────────────

var svcShowCmd = &cobra.Command{
	Use:     "show",
	Aliases: []string{"s", "list", "ls"},
	Short:   "Show current services",
	Long: `Show microservices registered in Oakestra.

  --app-id   <id>    filter by applicationID
  --app-name <name>  filter by application name`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.New()
		if err != nil {
			return err
		}
		ctx := cmd.Context()

		appID := svcShowAppID
		if svcShowAppName != "" && appID == "" {
			if appID, err = resolveAppIDByName(ctx, client, svcShowAppName); err != nil {
				return err
			}
		}

		svcs, _, err := client.Services.List(ctx, &oakestra.ServiceListOptions{ApplicationID: appID})
		if err != nil {
			return err
		}
		if len(svcs) == 0 {
			fmt.Println("No services exist yet.")
			return nil
		}
		printServicesTable(svcs)
		return nil
	},
}

// ─── service inspect ─────────────────────────────────────────────────────────
//
//	oak s inspect <id|name>         → instance list
//	oak s inspect <id|name> <num>   → detailed view of instance <num>
//	-f                              → refresh every 5 s

var svcInspectCmd = &cobra.Command{
	Use:     "inspect <service-id|name> [instance-number]",
	Aliases: []string{"i"},
	Short:   "Inspect a service or a specific instance",
	Long: `Inspect a service or a specific instance.

  oak s inspect <id|name>       list all instances
  oak s inspect <id|name> 0     detailed view of instance 0
  -f / --follow                 refresh every 5 seconds`,
	Args:              cobra.RangeArgs(1, 2),
	ValidArgsFunction: completeServiceThenInstances,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.New()
		if err != nil {
			return err
		}
		ctx := cmd.Context()

		instanceNum := -1
		if len(args) == 2 {
			n, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("instance number must be an integer, got %q", args[1])
			}
			instanceNum = n
		}

		run := func() error {
			svc, _, err := client.Services.ResolveByNameOrID(ctx, args[0])
			if err != nil {
				return err
			}
			if instanceNum < 0 {
				printServiceInstanceList(svc)
			} else {
				printInstanceDetail(svc, instanceNum)
			}
			return nil
		}

		if !svcFollow {
			return run()
		}
		runFollow(run)
		return nil
	},
}

// ─── service logs ─────────────────────────────────────────────────────────────
//
//	oak s logs <id|name> <instance-number>
//	-f  → refresh every 5 s

var svcLogsCmd = &cobra.Command{
	Use:   "logs <service-id|name> <instance-number>",
	Short: "Show logs of a specific service instance",
	Long: `Show the logs of a specific service instance.

  oak s logs <id|name> 0      show logs for instance 0
  -f / --follow               refresh every 5 seconds`,
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: completeServiceThenInstances,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.New()
		if err != nil {
			return err
		}
		ctx := cmd.Context()
		instanceNum, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("instance number must be an integer, got %q", args[1])
		}

		run := func() error {
			svc, _, err := client.Services.ResolveByNameOrID(ctx, args[0])
			if err != nil {
				return err
			}
			inst, ok := findInstance(svc, instanceNum)
			if !ok {
				return fmt.Errorf("instance %d not found for service %s", instanceNum, svc.MicroserviceName)
			}
			fmt.Printf("%s  %s  instance %s\n\n",
				colorName(svc.MicroserviceName),
				colorID(svc.MicroserviceID),
				bold(strconv.Itoa(instanceNum)),
			)
			logs := StripANSI(inst.Logs)
			if logs == "" {
				fmt.Println(dim("(no logs available)"))
			} else {
				fmt.Println(logs)
			}
			return nil
		}

		if !svcLogsFollow {
			return run()
		}
		runFollow(run)
		return nil
	},
}

// ─── service deploy ───────────────────────────────────────────────────────────
//
//	oak s deploy <id|name>    deploy one instance
//	oak s deploy --all        deploy one instance per service

var svcDeployCmd = &cobra.Command{
	Use:     "deploy [service-id|name]",
	Aliases: []string{"d"},
	Short:   "Deploy a new service instance",
	Long: `Deploy a new instance of a service.

  oak s deploy <id|name>    deploy one instance
  oak s deploy --all        deploy one instance for every registered service

To deploy multiple instances at once, use: oak s scale up <id|name> <count>`,
	Args:              cobra.RangeArgs(0, 1),
	ValidArgsFunction: completeServices,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.New()
		if err != nil {
			return err
		}
		ctx := cmd.Context()

		if svcDeployAll {
			return deployAll(ctx, client)
		}
		if len(args) == 0 {
			return fmt.Errorf("provide a service ID/name or use --all")
		}

		svc, _, err := client.Services.ResolveByNameOrID(ctx, args[0])
		if err != nil {
			return err
		}
		if _, err := client.Services.DeployInstance(ctx, svc.MicroserviceID); err != nil {
			return err
		}
		fmt.Printf("✓ Deployed new instance for %s (%s)\n",
			colorName(svc.MicroserviceName), colorID(svc.MicroserviceID))
		return nil
	},
}

// deployAll sends one deploy request per registered service.
func deployAll(ctx context.Context, client *oakestra.Client) error {
	svcs, _, err := client.Services.List(ctx, nil)
	if err != nil {
		return err
	}
	if len(svcs) == 0 {
		fmt.Println("No services exist yet.")
		return nil
	}
	fmt.Printf("Deploying 1 instance for each of %d service(s)…\n", len(svcs))
	bar := newBar(len(svcs), "Deploying")
	var failed int
	for _, svc := range svcs {
		if _, err := client.Services.DeployInstance(ctx, svc.MicroserviceID); err != nil {
			fmt.Fprintf(os.Stderr, "\n  ✗ %s: %v\n", svc.MicroserviceName, api.Hint(err))
			failed++
		}
		bar.Add(1) //nolint:errcheck
	}
	bar.Finish() //nolint:errcheck
	fmt.Printf("\nDone — %d succeeded, %d failed.\n", len(svcs)-failed, failed)
	return nil
}

// ─── service undeploy ─────────────────────────────────────────────────────────
//
//	oak s undeploy <id|name>          undeploy ALL instances
//	oak s undeploy <id|name> <num>    undeploy instance <num>

var svcUndeployCmd = &cobra.Command{
	Use:     "undeploy <service-id|name> [instance-number]",
	Aliases: []string{"u", "down"},
	Short:   "Undeploy a service instance",
	Long: `Undeploy service instances.

  oak s undeploy <id|name>      undeploy ALL instances
  oak s undeploy <id|name> 0    undeploy instance 0

To undeploy multiple specific instances, use: oak s scale down <id|name> <count>`,
	Args:              cobra.RangeArgs(1, 2),
	ValidArgsFunction: completeServiceThenInstances,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.New()
		if err != nil {
			return err
		}
		ctx := cmd.Context()
		svc, _, err := client.Services.ResolveByNameOrID(ctx, args[0])
		if err != nil {
			return err
		}

		if len(args) == 1 {
			return undeployAll(ctx, client, svc)
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 0 {
			return fmt.Errorf("instance number must be a non-negative integer, got %q", args[1])
		}
		return undeployOne(ctx, client, svc, n)
	},
}

func undeployAll(ctx context.Context, client *oakestra.Client, svc *oakestra.Service) error {
	instances := svc.InstanceList
	if len(instances) == 0 {
		fmt.Printf("Service %s has no running instances.\n", colorName(svc.MicroserviceName))
		return nil
	}
	fmt.Printf("Undeploying all %d instance(s) of %s…\n", len(instances), colorName(svc.MicroserviceName))
	bar := newBar(len(instances), "Undeploying")
	var failed int
	for _, inst := range instances {
		if _, err := client.Services.UndeployInstance(ctx, svc.MicroserviceID, inst.InstanceNumber); err != nil {
			fmt.Fprintf(os.Stderr, "\n  ✗ instance %d: %v\n", inst.InstanceNumber, api.Hint(err))
			failed++
		}
		bar.Add(1) //nolint:errcheck
		time.Sleep(100 * time.Millisecond)
	}
	bar.Finish() //nolint:errcheck
	fmt.Printf("\nDone — %d undeployed, %d failed.\n", len(instances)-failed, failed)
	return nil
}

func undeployOne(ctx context.Context, client *oakestra.Client, svc *oakestra.Service, instanceID int) error {
	if _, err := client.Services.UndeployInstance(ctx, svc.MicroserviceID, instanceID); err != nil {
		return err
	}
	fmt.Printf("✓ Undeployed instance %d of %s (%s)\n",
		instanceID, colorName(svc.MicroserviceName), colorID(svc.MicroserviceID))
	return nil
}

// ─── service scale ────────────────────────────────────────────────────────────
//
//	oak s scale up   <id|name> <count>  → deploy count instances
//	oak s scale down <id|name> <count>  → undeploy count instances

var svcScaleCmd = &cobra.Command{
	Use:   "scale <up|down> <service-id|name> <count>",
	Short: "Scale a service up or down by deploying/undeploying multiple instances",
	Long: `Scale a service by deploying or undeploying several instances at once.

  oak s scale up   <id|name> 3    deploy 3 new instances
  oak s scale down <id|name> 2    undeploy 2 instances (picks from running list)`,
	Args:              cobra.ExactArgs(3),
	ValidArgsFunction: completeScale,
	RunE: func(cmd *cobra.Command, args []string) error {
		direction := args[0]
		if direction != "up" && direction != "down" {
			return fmt.Errorf("direction must be 'up' or 'down', got %q", direction)
		}
		count, err := strconv.Atoi(args[2])
		if err != nil || count < 1 {
			return fmt.Errorf("count must be a positive integer, got %q", args[2])
		}

		client, err := api.New()
		if err != nil {
			return err
		}
		ctx := cmd.Context()
		svc, _, err := client.Services.ResolveByNameOrID(ctx, args[1])
		if err != nil {
			return err
		}

		if direction == "up" {
			return scaleUp(ctx, client, svc, count)
		}
		return scaleDown(ctx, client, svc, count)
	},
}

func scaleUp(ctx context.Context, client *oakestra.Client, svc *oakestra.Service, count int) error {
	fmt.Printf("Scaling up %s by %d instance(s)…\n", colorName(svc.MicroserviceName), count)
	bar := newBar(count, "Deploying")
	var failed int
	for i := 0; i < count; i++ {
		if _, err := client.Services.DeployInstance(ctx, svc.MicroserviceID); err != nil {
			fmt.Fprintf(os.Stderr, "\n  ✗ deploy %d/%d: %v\n", i+1, count, api.Hint(err))
			failed++
		}
		bar.Add(1) //nolint:errcheck
		time.Sleep(100 * time.Millisecond)
	}
	bar.Finish() //nolint:errcheck
	fmt.Printf("\nDone — %d deployed, %d failed.\n", count-failed, failed)
	return nil
}

func scaleDown(ctx context.Context, client *oakestra.Client, svc *oakestra.Service, count int) error {
	instances := svc.InstanceList
	if len(instances) == 0 {
		fmt.Printf("Service %s has no running instances to remove.\n", colorName(svc.MicroserviceName))
		return nil
	}
	toRemove := instances
	if count < len(instances) {
		toRemove = instances[:count]
	}
	fmt.Printf("Scaling down %s by %d of %d instance(s)…\n",
		colorName(svc.MicroserviceName), len(toRemove), len(instances))
	bar := newBar(len(toRemove), "Undeploying")
	var failed int
	for _, inst := range toRemove {
		if _, err := client.Services.UndeployInstance(ctx, svc.MicroserviceID, inst.InstanceNumber); err != nil {
			fmt.Fprintf(os.Stderr, "\n  ✗ instance %d: %v\n", inst.InstanceNumber, api.Hint(err))
			failed++
		}
		bar.Add(1) //nolint:errcheck
		time.Sleep(100 * time.Millisecond)
	}
	bar.Finish() //nolint:errcheck
	fmt.Printf("\nDone — %d undeployed, %d failed.\n", len(toRemove)-failed, failed)
	return nil
}

// ─── display helpers ──────────────────────────────────────────────────────────

func printServicesTable(svcs []oakestra.Service) {
	headers := []string{"SERVICE ID", "NAME", "NAMESPACE", "APPLICATION", "INSTANCES", "STATUS"}
	rows := make([][]string, len(svcs))
	for i := range svcs {
		s := &svcs[i]
		rows[i] = []string{
			colorID(s.MicroserviceID),
			colorName(s.MicroserviceName),
			italic(s.MicroserviceNamespace),
			blue(s.GetApplicationName()),
			bold(fmt.Sprintf("%d", len(s.InstanceList))),
			colorStatus(serviceStatusText(s)),
		}
	}
	printTable(headers, rows)
}

// printServiceInstanceList shows the instance table for a service.
func printServiceInstanceList(svc *oakestra.Service) {
	fmt.Printf("%s  %s  %s\n\n",
		colorName(svc.MicroserviceName),
		colorID(svc.MicroserviceID),
		colorStatus(serviceStatusText(svc)),
	)
	if len(svc.InstanceList) == 0 {
		fmt.Println(dim("No running instances."))
		return
	}
	headers := []string{"#", "STATUS", "HOST IP", "PUBLIC IP", "CPU %", "MEM %", "DISK"}
	rows := make([][]string, len(svc.InstanceList))
	for i, inst := range svc.InstanceList {
		//convert disk string to float
		disk, err := strconv.ParseFloat(inst.Disk, 64)
		if err != nil {
			disk = 0
		}
		diskMB := fmt.Sprintf("%.1f MB", disk/1024/1024)
		if inst.Disk == "0" {
			diskMB = dim("—")
		}
		rows[i] = []string{
			bold(strconv.Itoa(inst.InstanceNumber)),
			colorStatus(inst.Status),
			inst.HostIP,
			inst.PublicIP,
			inst.CPUPercent,
			inst.MemoryPercent,
			diskMB,
		}
	}
	printTable(headers, rows)
}

// printInstanceDetail shows an expanded view of a single instance.
func printInstanceDetail(svc *oakestra.Service, instanceNum int) {
	inst, ok := findInstance(svc, instanceNum)
	if !ok {
		fmt.Printf("Instance %d not found for service %s.\n",
			instanceNum, colorName(svc.MicroserviceName))
		return
	}

	// Service-level info.
	printKV([][2]string{
		{"Service:", colorName(svc.MicroserviceName)},
		{"Service ID:", colorID(svc.MicroserviceID)},
		{"Application:", blue(svc.GetApplicationName()) + " / " + italic(svc.GetApplicationNamespace())},
		{"Namespace:", italic(svc.MicroserviceNamespace)},
		{"Image:", svc.Code},
		{"Virtualization:", svc.Virtualization},
		{"Port:", svc.Port},
		{"Round-Robin IP:", svc.RRip},
		{"Service Status:", colorStatus(svc.Status)},
	})

	// Instance-level info.
	fmt.Printf("\n%s\n", bold(fmt.Sprintf("Instance %d:", instanceNum)))

	disk, err := strconv.ParseFloat(inst.Disk, 64)
	if err != nil {
		disk = 0
	}
	diskMB := fmt.Sprintf("%.1f MB", disk/1024/1024)
	if inst.Disk == "0" {
		diskMB = dim("—")
	}
	printKV([][2]string{
		{"  Status:", colorStatus(inst.Status)},
		{"  Detail:", inst.StatusDetail},
		{"  Host:", fmt.Sprintf("%s", inst.HostIP)},
		{"  Public IP:", inst.PublicIP},
		{"  Cluster:", colorID(inst.ClusterID)},
		{"  Location:", inst.ClusterLocation},
		{"  Worker:", colorID(inst.WorkerID)},
		{"  CPU:", inst.CPUPercent + "%"},
		{"  Memory:", inst.MemoryPercent + "%"},
		{"  Disk:", diskMB},
	})

	if len(svc.Environment) > 0 {
		fmt.Printf("\n%s\n", bold("Environment:"))
		for _, e := range svc.Environment {
			fmt.Printf("  %s\n", e)
		}
	}
}

func serviceStatusText(svc *oakestra.Service) string {
	if len(svc.InstanceList) == 0 {
		return "no instances"
	}
	running := 0
	for _, inst := range svc.InstanceList {
		if inst.Status == "RUNNING" || inst.Status == "running" {
			running++
		}
	}
	return fmt.Sprintf("%d/%d running", running, len(svc.InstanceList))
}

// ─── misc helpers ─────────────────────────────────────────────────────────────

func findInstance(svc *oakestra.Service, num int) (*oakestra.ServiceInstance, bool) {
	for i := range svc.InstanceList {
		if svc.InstanceList[i].InstanceNumber == num {
			return &svc.InstanceList[i], true
		}
	}
	return nil, false
}

func clearScreen() {
	fmt.Print("\033[2J\033[H")
}

// runFollow re-runs run every 5s until interrupted. It never returns to
// Execute(), so it applies api.Hint itself instead of relying on the
// central print point there.
func runFollow(run func() error) {
	for {
		clearScreen()
		if err := run(); err != nil {
			fmt.Fprintln(os.Stderr, api.Hint(err))
		}
		fmt.Println(dim("\nRefreshing every 5s — Ctrl+C to stop"))
		time.Sleep(5 * time.Second)
	}
}

// ─── progress bar ─────────────────────────────────────────────────────────────

func newBar(total int, description string) *progressbar.ProgressBar {
	return progressbar.NewOptions(total,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(30),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer: "=", SaucerHead: ">", SaucerPadding: " ",
			BarStart: "[", BarEnd: "]",
		}),
	)
}

// ─── resolve app by name ──────────────────────────────────────────────────────

func resolveAppIDByName(ctx context.Context, client *oakestra.Client, name string) (string, error) {
	app, _, err := client.Applications.ResolveByNameOrID(ctx, name)
	if err != nil {
		return "", err
	}
	return app.ApplicationID, nil
}
