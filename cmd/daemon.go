package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"kerberoskeepalive/internal/daemon"
	"kerberoskeepalive/internal/launchagent"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run in the foreground, keeping configured tickets refreshed (invoked by launchd)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return daemon.Run(cmd.Context(), configPath, profiles)
	},
}

var daemonInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Generate and load a LaunchAgent that runs the daemon at login",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := loadConfig(); err != nil {
			return fmt.Errorf("config invalid, not installing: %w", err)
		}
		if err := launchagent.Install(configPath); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "LaunchAgent installed and loaded.")
		return nil
	},
}

var daemonUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Unload and remove the LaunchAgent",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := launchagent.Uninstall(); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "LaunchAgent unloaded and removed.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonInstallCmd, daemonUninstallCmd)
}
