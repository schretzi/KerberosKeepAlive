// Package manager orchestrates config, Keychain lookups, and Kerberos
// ticket acquisition — the shared logic used by both the CLI commands and
// the daemon poll loop.
package manager

import (
	"context"
	"errors"
	"fmt"

	"github.com/schretzi/kerberoskeepalive/internal/config"
	"github.com/schretzi/kerberoskeepalive/internal/keychain"
	"github.com/schretzi/kerberoskeepalive/internal/krb"
)

// AcquireProfile looks up p's password in the Keychain and runs kinit to
// (re)acquire its ticket, writing the credential cache named by p.CCache.
func AcquireProfile(ctx context.Context, cfg *config.Config, p config.Profile) error {
	password, err := keychain.LookupPassword(p.Keychain.Service, p.Keychain.Account)
	if err != nil {
		return fmt.Errorf("profile %s: %w", p.Name, err)
	}
	err = krb.Acquire(ctx, krb.AcquireOptions{
		Principal:    p.Principal,
		CCache:       p.CCache,
		Password:     password,
		Krb5ConfPath: cfg.Krb5ConfPath,
		Lifetime:     p.TicketLifetime.Duration(),
	})
	if err != nil {
		return fmt.Errorf("profile %s: %w", p.Name, err)
	}
	return nil
}

// IsPermanent reports whether err will keep failing identically until the
// user changes something — a rejected credential or a missing Keychain item,
// as opposed to an unreachable KDC. Callers that retry on a timer must stop
// on these: re-running kinit every poll against a stale password is how an
// account gets locked out.
func IsPermanent(err error) bool {
	return errors.Is(err, krb.ErrCredentialRejected) || errors.Is(err, keychain.ErrNotFound)
}

// NeedsRefresh reports whether p's ticket is missing, expired, or within its
// configured refresh threshold of expiring.
func NeedsRefresh(p config.Profile) (bool, krb.TicketStatus, error) {
	st, err := krb.ReadStatus(p.CCache)
	if err != nil {
		return true, st, err
	}
	if !st.Exists || st.Expired {
		return true, st, nil
	}
	return st.Remaining < p.RefreshThreshold.Duration(), st, nil
}
