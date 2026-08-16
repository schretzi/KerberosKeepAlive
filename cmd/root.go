// Package cmd implements the kerberoskeepalive CLI.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"kerberoskeepalive/internal/config"
)

var (
	configPath string
	profiles   []string
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

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", config.DefaultPath(), "path to config file")
	rootCmd.PersistentFlags().StringSliceVar(&profiles, "profile", nil, "profile name(s) to operate on (default: all configured profiles)")
}
