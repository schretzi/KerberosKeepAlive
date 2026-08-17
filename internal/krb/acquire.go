// Package krb acquires and inspects Kerberos ticket-granting tickets.
//
// Acquisition shells out to macOS's built-in /usr/bin/kinit (Heimdal) rather
// than doing the AS-REQ in pure Go: kinit already writes a standard
// credential-cache file in the exact format the rest of the system expects,
// which avoids hand-rolling a ccache binary serializer. Reading ccache files
// back (see status.go) uses gokrb5's credentials package instead, since that
// only requires the well-tested read side of that library.
package krb

import (
	"bytes"
	"context"
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

// Acquire runs kinit to obtain a fresh ticket for principal, writing the
// resulting credential cache to ccachePath (created 0600 by kinit itself).
// The password is piped via stdin using kinit's --password-file=STDIN, so it
// never appears in argv or touches disk. If krb5ConfPath is non-empty it is
// forwarded to kinit via the KRB5_CONFIG environment variable.
func Acquire(ctx context.Context, principal, ccachePath, password, krb5ConfPath string) error {
	if err := os.MkdirAll(filepath.Dir(ccachePath), 0o700); err != nil {
		return fmt.Errorf("creating ccache directory for %s: %w", ccachePath, err)
	}

	ctx, cancel := context.WithTimeout(ctx, acquireTimeout)
	defer cancel()

	// #nosec G204 -- kinitPath is a hardcoded constant; ccachePath/principal
	// come from the user's own local config, passed as argv (no shell), so
	// there's no injection surface.
	cmd := exec.CommandContext(ctx, kinitPath, "--password-file=STDIN", "-c", ccachePath, principal)
	if krb5ConfPath != "" {
		cmd.Env = append(os.Environ(), "KRB5_CONFIG="+krb5ConfPath)
	}
	cmd.Stdin = strings.NewReader(password)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kinit failed for %s: %w: %s", principal, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
