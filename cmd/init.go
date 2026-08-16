package cmd

import "github.com/spf13/cobra"

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Acquire a fresh ticket now for the configured profile(s) (first-time setup)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAcquireAll(cmd)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
