// Package daemon implements the poll loop behind the `daemon` subcommand.
package daemon

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/schretzi/kerberoskeepalive/internal/config"
	"github.com/schretzi/kerberoskeepalive/internal/manager"
)

// Run loads configPath and polls every selected profile's ticket (all
// profiles if profileNames is empty), reacquiring any that are missing,
// expired, or within their refresh threshold, until it receives
// SIGTERM/SIGINT or ctx is canceled. The config file (profiles and
// thresholds) is re-read at the start of every poll cycle so edits take
// effect without a restart; the poll interval and log settings are fixed at
// startup — changing daemon.poll_interval or daemon.log requires a restart.
func Run(ctx context.Context, configPath string, profileNames []string) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	selected, err := cfg.SelectProfiles(profileNames)
	if err != nil {
		return err
	}

	closeLog, err := setupLogging(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = closeLog.Close() }()

	log.Printf("daemon starting: %d profile(s), poll_interval=%s, log=%s",
		len(selected), cfg.Daemon.PollInterval.Duration(), cfg.Daemon.Log.Path)

	pollAll(ctx, cfg, selected)

	ticker := time.NewTicker(cfg.Daemon.PollInterval.Duration())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cfg, err := config.Load(configPath)
			if err != nil {
				log.Printf("reloading config: %v", err)
				continue
			}
			selected, err := cfg.SelectProfiles(profileNames)
			if err != nil {
				log.Printf("reloading config: %v", err)
				continue
			}
			pollAll(ctx, cfg, selected)
		case <-ctx.Done():
			log.Println("received shutdown signal, exiting")
			return nil
		}
	}
}

func pollAll(ctx context.Context, cfg *config.Config, selected []config.Profile) {
	for _, p := range selected {
		refresh, st, err := manager.NeedsRefresh(p)
		if err != nil {
			log.Printf("profile %s: checking status: %v", p.Name, err)
		}
		if !refresh {
			continue
		}
		log.Printf("profile %s: refreshing (exists=%v expired=%v remaining=%s)", p.Name, st.Exists, st.Expired, st.Remaining.Round(time.Second))
		if err := manager.AcquireProfile(ctx, cfg, p); err != nil {
			// AcquireProfile already prefixes "profile <name>: ", so adding
			// it here too would double it.
			log.Print(err)
			continue
		}
		log.Printf("profile %s: ticket acquired", p.Name)
	}
}
