package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"kerberoskeepalive/internal/config"
	"kerberoskeepalive/internal/manager"
)

func loadConfig() (*config.Config, error) {
	return config.Load(configPath)
}

func selectedProfiles(cfg *config.Config) ([]config.Profile, error) {
	return cfg.SelectProfiles(profiles)
}

// runAcquireAll acquires fresh tickets for the selected profiles, printing
// per-profile progress, and returns a summary error if any profile failed.
func runAcquireAll(cmd *cobra.Command) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	selected, err := selectedProfiles(cfg)
	if err != nil {
		return err
	}

	var failed int
	for _, p := range selected {
		if err := manager.AcquireProfile(cmd.Context(), cfg, p); err != nil {
			failed++
			fmt.Fprintln(cmd.ErrOrStderr(), err)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "profile %s: ticket acquired\n", p.Name)
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d profile(s) failed", failed, len(selected))
	}
	return nil
}
