// Package cmd implements the kerberoskeepalive CLI.
package cmd

import (
	"fmt"
	"os"

	"github.com/schretzi/kerberoskeepalive/internal/config"
	"github.com/schretzi/kerberoskeepalive/internal/version"

	"github.com/spf13/cobra"
)

var (
	configPath string
	profiles   []string
)

// appName is the binary name: it drives the launchd label, the log file
// names and the `version` output.
const appName = "kerberoskeepalive"

// licenseNotice is printed by `kerberoskeepalive version`.
const licenseNotice = `Copyright (C) 2026 Schretzi
License: MIT <https://opensource.org/licenses/MIT>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.`

var rootCmd = &cobra.Command{
	Use:           "kerberoskeepalive",
	Short:         "Manage and keep macOS Kerberos tickets alive",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI and exits the process on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// Root returns the root command, for tooling that needs the command tree
// without running it (e.g. the docs generator in tools/gendocs).
func Root() *cobra.Command {
	return rootCmd
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", config.DefaultPath(), "path to config file")
	rootCmd.PersistentFlags().StringSliceVar(&profiles, "profile", nil, "profile name(s) to operate on (default: all configured profiles)")
	// Avoid a generation-timestamp footer that would otherwise churn every
	// time docs/ is regenerated with no real content change.
	rootCmd.DisableAutoGenTag = true

	// `--version` and `version` report the same thing, from the same place.
	rootCmd.Version = version.String(appName)
	rootCmd.AddCommand(version.NewCommand(appName, licenseNotice))
}
