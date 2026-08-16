// Package manager orchestrates config, Keychain lookups, and Kerberos
// ticket acquisition — the shared logic used by both the CLI commands and
// the daemon poll loop.
package manager

import (
	"context"
	"fmt"

	"kerberoskeepalive/internal/config"
	"kerberoskeepalive/internal/keychain"
	"kerberoskeepalive/internal/krb"
)

// AcquireProfile looks up p's password in the Keychain and runs kinit to
// (re)acquire its ticket, writing the ccache to p.CCachePath.
func AcquireProfile(ctx context.Context, cfg *config.Config, p config.Profile) error {
	password, err := keychain.LookupPassword(p.Keychain.Service, p.Keychain.Account)
	if err != nil {
		return fmt.Errorf("profile %s: %w", p.Name, err)
	}
	if err := krb.Acquire(ctx, p.Principal, p.CCachePath, password, cfg.Krb5ConfPath); err != nil {
		return fmt.Errorf("profile %s: %w", p.Name, err)
	}
	return nil
}

// NeedsRefresh reports whether p's ticket is missing, expired, or within its
// configured refresh threshold of expiring.
func NeedsRefresh(p config.Profile) (bool, krb.TicketStatus, error) {
	st, err := krb.ReadStatus(p.CCachePath)
	if err != nil {
		return true, st, err
	}
	if !st.Exists || st.Expired {
		return true, st, nil
	}
	return st.Remaining < p.RefreshThreshold.Duration(), st, nil
}
