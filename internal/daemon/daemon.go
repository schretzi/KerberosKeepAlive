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

	blocked := make(blocklist, len(selected))
	pollAll(ctx, cfg, selected, blocked)

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
			pollAll(ctx, cfg, selected, blocked)
		case <-ctx.Done():
			log.Println("received shutdown signal, exiting")
			return nil
		}
	}
}

// blocklist records profiles whose last acquisition failed for a reason that
// retrying cannot fix — a rejected password, a missing Keychain item. It maps
// profile name to the credential identity that failed, so editing the config
// to point at a different principal or Keychain item clears the block without
// a restart. Fixing the *password* behind an unchanged Keychain item is not
// observable here, so that case needs a daemon restart or a manual `refresh`.
//
// This exists because the poll loop has no backoff: without it, a stale
// Keychain password means a failed pre-auth every poll interval, which walks
// straight into the AD account lockout threshold in a few minutes.
type blocklist map[string]string

// credentialKey identifies the inputs that determine whether the KDC will
// reject an acquisition. The ccache is deliberately excluded: where the
// ticket is written has no bearing on whether the credential is accepted.
func credentialKey(p config.Profile) string {
	return p.Principal + "\x00" + p.Keychain.Service + "\x00" + p.Keychain.Account
}

func (b blocklist) blocks(p config.Profile) bool {
	key, ok := b[p.Name]
	return ok && key == credentialKey(p)
}

func (b blocklist) block(p config.Profile) { b[p.Name] = credentialKey(p) }

func pollAll(ctx context.Context, cfg *config.Config, selected []config.Profile, blocked blocklist) {
	for _, p := range selected {
		// Skip silently: the reason was logged once when the block was set,
		// and repeating it every poll is the noise this is meant to avoid.
		if blocked.blocks(p) {
			continue
		}
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
			if manager.IsPermanent(err) {
				blocked.block(p)
				log.Printf("profile %s: not retrying until its principal or keychain reference changes, or the daemon is restarted — repeated attempts with a rejected credential risk locking the account", p.Name)
			}
			continue
		}
		log.Printf("profile %s: ticket acquired", p.Name)
	}
}
