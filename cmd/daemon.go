package cmd

import (
	"github.com/schretzi/kerberoskeepalive/internal/daemon"

	"github.com/spf13/cobra"
)

// daemonCmd is the foreground process. Installing and controlling the launchd
// job that runs it lives under `service` — see cmd/service.go.
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run in the foreground, keeping configured tickets refreshed (invoked by launchd)",
	Long: `Poll the configured profiles and reacquire any ticket close to expiry, until
interrupted.

This is the process the LaunchAgent runs; it is not how you install or control
that agent. Use ` + "`kerberoskeepalive service`" + ` for the launchd job.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return daemon.Run(cmd.Context(), configPath, profiles)
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}
