// Package krb acquires and inspects Kerberos ticket-granting tickets.
//
// Acquisition shells out to macOS's built-in /usr/bin/kinit (Heimdal) rather
// than doing the AS-REQ in pure Go: kinit already writes a standard
// credential cache in the exact format the rest of the system expects, and
// for API caches it is the only practical way to reach Apple's GSSCred
// daemon without cgo. Reading caches back (see status.go) uses gokrb5 for
// FILE caches and /usr/bin/klist for API ones.
package krb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// kinitPath is hardcoded rather than resolved via PATH because launchd's
// default PATH for LaunchAgents is minimal and may not include /usr/bin.
const kinitPath = "/usr/bin/kinit"

// acquireTimeout bounds a single kinit attempt so a slow/unreachable KDC
// can't stall an entire daemon poll cycle. Observed live: kinit against a
// resolvable-but-KDC-less domain can block ~15s on DNS/network timeouts
// before even a killed context unblocks it, so this leaves headroom above
// that while still capping the worst case.
const acquireTimeout = 30 * time.Second

// passwordFileStdin makes kinit read the password from stdin, so it never
// appears in argv or touches disk.
const passwordFileStdin = "--password-file=STDIN"

// ErrCredentialRejected reports that the KDC understood the request and
// refused it — a wrong or expired password, a revoked or unknown principal.
// Retrying cannot fix any of those, and retrying every poll interval against
// a stale password is how an account gets locked out, so callers should stop
// until the credential or the config changes.
var ErrCredentialRejected = errors.New("KDC rejected the credential")

// rejectionMarkers are substrings of Heimdal kinit's stderr that mean the
// credential itself was refused, as opposed to the KDC being unreachable or
// some other transient failure. Matched case-insensitively.
var rejectionMarkers = []string{
	"preauthentication failed",
	"password incorrect",
	"bad password",
	"password has expired",
	"credentials have been revoked",
	"client unknown",
	"principal unknown",
	"not found in kerberos database",
}

// AcquireOptions describes a single kinit invocation.
type AcquireOptions struct {
	// Principal is the client principal, user@REALM.
	Principal string

	// CCache is a normalized cache spec: config.CCacheAPI or "FILE:/path".
	CCache string

	// Password is piped to kinit over stdin.
	Password string

	// Krb5ConfPath, when non-empty, is forwarded as KRB5_CONFIG.
	Krb5ConfPath string

	// Lifetime, when non-zero, is requested via kinit --lifetime. The KDC
	// issues min(requested, account policy), so asking for less than the
	// realm allows always works and asking for more silently does not.
	Lifetime time.Duration
}

// Acquire runs kinit to obtain a fresh ticket for opts.Principal. Tickets are
// deliberately not requested as renewable: the daemon re-acquires from the
// Keychain password, so renewal buys nothing, and a non-renewable ticket that
// leaks dies at its expiry instead of being renewable for days by whoever
// took it.
//
// The password is piped via kinit's --password-file=STDIN, so it never
// appears in argv or touches disk. Failures caused by the KDC rejecting the
// credential wrap ErrCredentialRejected.
func Acquire(ctx context.Context, opts AcquireOptions) error {
	if path := filePath(opts.CCache); path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("creating ccache directory for %s: %w", path, err)
		}
	}
	args := kinitArgs(opts)

	ctx, cancel := context.WithTimeout(ctx, acquireTimeout)
	defer cancel()

	// #nosec G204 -- kinitPath is a hardcoded constant; the principal and
	// ccache come from the user's own local config and are passed as argv
	// (no shell), so there is no injection surface.
	cmd := exec.CommandContext(ctx, kinitPath, args...)
	cmd.Env = kinitEnv(opts)
	cmd.Stdin = strings.NewReader(opts.Password)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if isCredentialRejection(msg) {
			return fmt.Errorf("kinit failed for %s: %w: %s", opts.Principal, ErrCredentialRejected, msg)
		}
		return fmt.Errorf("kinit failed for %s: %w: %s", opts.Principal, err, msg)
	}
	return nil
}

// kinitArgs builds kinit's argv. Note what is absent: neither --renewable
// nor --renewable-life is ever passed, so the KDC issues a non-renewable
// ticket that dies at its expiry rather than one an attacker could renew for
// days without the password.
func kinitArgs(opts AcquireOptions) []string {
	args := []string{passwordFileStdin}
	if opts.Lifetime > 0 {
		args = append(args, fmt.Sprintf("--lifetime=%ds", int64(opts.Lifetime.Seconds())))
	}
	// For the API cache no -c is passed at all: kinit then writes the user's
	// default GSSCred cache and switches the default to it, which is exactly
	// what GSS-API consumers resolve. See config.CCacheAPI.
	if filePath(opts.CCache) != "" {
		args = append(args, "-c", opts.CCache)
	}
	return append(args, opts.Principal)
}

// kinitEnv builds kinit's environment. KRB5CCNAME is cleared unconditionally
// so the cache actually written is the one the config asked for: a stray
// KRB5CCNAME inherited from the launchd or shell environment would otherwise
// silently redirect the ticket somewhere neither the daemon nor Alpaca looks.
func kinitEnv(opts AcquireOptions) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "KRB5CCNAME=") {
			continue
		}
		env = append(env, kv)
	}
	if opts.Krb5ConfPath != "" {
		env = append(env, "KRB5_CONFIG="+opts.Krb5ConfPath)
	}
	return env
}

// isCredentialRejection reports whether kinit's stderr indicates the KDC
// refused the credential rather than failing transiently.
func isCredentialRejection(stderr string) bool {
	lower := strings.ToLower(stderr)
	for _, marker := range rejectionMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
