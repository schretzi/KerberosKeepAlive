// Package cmd implements the kerberoskeepalive CLI.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/schretzi/kerberoskeepalive/internal/config"
)

var (
	configPath string
	profiles   []string
)

// Set via -ldflags by goreleaser (see .goreleaser.yaml); "dev" for local
// `go build`/`go run`.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

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
	rootCmd.Version = fmt.Sprintf("%s (commit %s, built %s)", version, commit, date)
}
