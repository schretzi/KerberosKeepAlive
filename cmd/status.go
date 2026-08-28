package cmd

import (
	"errors"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/schretzi/kerberoskeepalive/internal/krb"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of configured tickets",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		selected, err := selectedProfiles(cfg)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tPRINCIPAL\tSTATUS\tEXPIRES\tREMAINING\tCCACHE")

		var anyBad bool
		for _, p := range selected {
			st, err := krb.ReadStatus(p.CCachePath)
			var state, expires, remaining string
			switch {
			case err != nil:
				state = "INVALID"
				anyBad = true
			case !st.Exists:
				state = "MISSING"
				anyBad = true
			case st.Expired:
				state = "EXPIRED"
				anyBad = true
				expires = st.EndTime.Local().Format(time.RFC3339)
			default:
				state = "VALID"
				expires = st.EndTime.Local().Format(time.RFC3339)
				remaining = st.Remaining.Round(time.Second).String()
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", p.Name, p.Principal, state, expires, remaining, p.CCachePath)
		}
		_ = w.Flush()

		if anyBad {
			return errors.New("one or more profiles are missing, invalid, or expired")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
