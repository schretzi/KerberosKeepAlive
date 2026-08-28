package cmd

import "github.com/spf13/cobra"

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Force re-acquire a ticket now, regardless of current expiry",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runAcquireAll(cmd)
	},
}

func init() {
	rootCmd.AddCommand(refreshCmd)
}
