// Package cmd wires up all Cobra sub-commands for oak-go-cli.
package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/oakestra/oak-go-cli/internal/api"
)

const rawBanner = `
                                         ░░░░░░░
                                   ░░░░░░░░░░░░░░░░░
                               ░░░░░░░░░░░░░░░░░░░░░░
                            ░░░░░░░░░░░░░░░░░░▒░░░░░░░
██╗    ██╗███████╗██╗  ░░░░██████╗░██████╗░███╗░░░███╗███████╗    ████████╗ ██████╗
██║    ██║██╔════╝██║░░░░░██╔════╝██╔═══██╗████╗░████║██╔════╝    ╚══██╔══╝██╔═══██╗
██║ █╗ ██║█████╗  ██║░░░░░██║░▒░░░██║░░░██║██╔████╔██║█████╗░░       ██║   ██║   ██║
██║███╗██║██╔══╝ ░██║░░░▒░██║░░▒░░██║░▒░██║██║╚██╔╝██║██╔══╝░░░░     ██║   ██║   ██║
╚███╔███╔╝███████╗███████╗╚██████╗╚██████╔╝██║░╚═╝░██║███████╗░░░    ██║   ╚██████╔╝
 ╚══╝╚══╝ ╚══════╝╚══════╝░╚═════╝░╚═════╝░╚═╝░▒░▒░╚═╝╚══════╝░░░    ╚═╝    ╚═════╝
                ░░░░░░░░░░▒▓▓░░░░▓▓▒▓▓▓▒▓▓▓▓▒▓▓▓░░░░░▒░░░▓▒░░░░░
                 ░░░░░░▒░░░░▒▓▓░░░▒▓▓▓▓▓▓▓▓░░▓▓▓▓░░▓▓▓▓▒░░░░░░
                   ░░▒▓░▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▒░░░
                       ░░░░░░░░░░░▒▓▒▓▓▓▓▓▓▓▒░░░░░░░░░░░
                                  ▒▓▒▓▓▒▓▓▓▓▒
         ██████╗  █████╗ ██╗  ██╗███████╗███████╗████████╗██████╗  █████╗
        ██╔═══██╗██╔══██╗██║ ██╔╝██╔════╝██╔════╝╚══██╔══╝██╔══██╗██╔══██╗
        ██║   ██║███████║█████╔╝ █████╗░░███████╗   ██║   ██████╔╝███████║
        ██║   ██║██╔══██║██╔═██╗ ██╔══╝░░╚════██║   ██║   ██╔══██╗██╔══██║
        ╚██████╔╝██║  ██║██║  ██╗███████╗███████║   ██║   ██║  ██║██║  ██║
         ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝╚══════╝   ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝
                               ▒▓▓▓▓▓▒▓▓▓▓▓▓▒▒
                             ▒▒▓▒▓ ▓▓  ▒▒▓▓▓▓▓▒▒▒▒ ▒
                        ▒▒ ▒▒▒▓▒▒▒▒▓▒▒   ▒▒▒▓▒▓ ▒▒▒▒ ▒
                       ▒    ▒ ▒▒▒▒   ▒▒ ▒▒  ▒ ▒▒▒▒ ▒▒▒▒
                               ▒  ▒▒             ▒ ▒   ▒

`

// colorBanner applies ANSI colors to the raw banner string.
func colorBanner() string {
	var sb strings.Builder
	for _, r := range rawBanner {
		switch r {
		case '║', '═', '╗', '╝', '╔', '╚':
			sb.WriteString(blue(string(r)))
		case '░':
			sb.WriteString(green(string(r)))
		case '█':
			sb.WriteString(ansi("97", string(r)))
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// rootCmd is the top-level command. All sub-commands are added in Execute().
var rootCmd = &cobra.Command{
	Use:   "oak",
	Short: "oak — Oakestra CLI (Go edition)",
	Long:  colorBanner() + "A fast, portable CLI for managing Oakestra deployments.",
	// Without this Cobra prints the raw (un-hinted) error itself before
	// Execute() below gets a chance to apply api.Hint. Setting it on the
	// root propagates to every subcommand, so Execute() stays the single
	// print point.
	SilenceErrors: true,
	// Show help when called with no arguments.
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// Execute is the single entry-point called from main.go. Every RunE that
// returns an error, directly or wrapped with %w, passes through here, so
// this is the one place that needs to apply api.Hint; individual commands
// just return the raw error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, api.Hint(err))
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(apiDocsCmd)
	rootCmd.AddCommand(dashboardCmd)
	rootCmd.AddCommand(applicationCmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(clusterCmd)
	rootCmd.AddCommand(configCmd)
	// install / uninstall / doctor require a Linux host — hide them on Windows.
	if runtime.GOOS != "windows" {
		rootCmd.AddCommand(installCmd)
		rootCmd.AddCommand(uninstallCmd)
		rootCmd.AddCommand(doctorCmd)
	}
	rootCmd.AddCommand(addonCmd)

	// Only expose `oak worker` when NodeEngine is installed on this machine.
	if runtime.GOOS != "windows" && nodeEngineInstalled() {
		rootCmd.AddCommand(workerCmd)
	}

	// Register the template helper that shows "name (alias1, alias2)" in the
	// "Available Commands" section.
	cobra.AddTemplateFuncs(template.FuncMap{
		"nameWithAliases": func(cmd *cobra.Command) string {
			if len(cmd.Aliases) == 0 {
				return cmd.Name()
			}
			return cmd.Name() + " " + dim("("+strings.Join(cmd.Aliases, ", ")+")")
		},
		// visibleLen returns the printable (non-ANSI) length of s, used for padding.
		"visibleLen": func(s string) int {
			// Strip ANSI escape sequences for length calculation.
			inEscape := false
			n := 0
			for _, r := range s {
				switch {
				case r == '\033':
					inEscape = true
				case inEscape && r == 'm':
					inEscape = false
				case !inEscape:
					n++
				}
			}
			return n
		},
	})

	// Override the help template to use nameWithAliases in the command listing
	// and to colorise section headers.
	rootCmd.SetHelpTemplate(helpTemplate())
	// Propagate to all sub-commands.
	cobra.AddTemplateFunc("nameWithAliases", func(cmd *cobra.Command) string {
		if len(cmd.Aliases) == 0 {
			return cmd.Name()
		}
		return cmd.Name() + " " + dim("("+strings.Join(cmd.Aliases, ", ")+")")
	})
}

// helpTemplate returns a Cobra help template that:
//   - shows "(alias, …)" next to every command name
//   - bolds section headers when colors are enabled
func helpTemplate() string {
	h := func(s string) string { return bold(s) }
	return `{{with .Long}}{{. | trimRightSpace}}

{{end}}` + h("Usage:") + `{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

` + h("Aliases:") + `
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

` + h("Examples:") + `
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

` + h("Available Commands:") + `{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{nameWithAliases . | printf "%-38s"}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

` + h("Flags:") + `
{{.LocalFlags.FlagUsages | trimRightSpace}}{{end}}{{if .HasAvailableInheritedFlags}}

` + h("Global Flags:") + `
{{.InheritedFlags.FlagUsages | trimRightSpace}}{{end}}{{if .HasHelpSubCommands}}

` + h("Additional help topics:") + `{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .Name .NamePadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.
{{end}}`
}
