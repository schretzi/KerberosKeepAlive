package krb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jcmturner/gokrb5/v8/credentials"
	"github.com/jcmturner/gokrb5/v8/iana/nametype"
	"github.com/jcmturner/gokrb5/v8/types"
)

// klistPath is hardcoded for the same reason as kinitPath: launchd's default
// PATH for LaunchAgents is minimal.
const klistPath = "/usr/bin/klist"

// statusTimeout bounds a klist invocation. Reading a local cache is a Mach
// round-trip to GSSCred, so this only needs to cover a wedged daemon.
const statusTimeout = 10 * time.Second

// klistTimeLayout is Heimdal klist's --json timestamp format, e.g.
// "20260828153137". It carries no zone, and comparison against the human
// output ("End time: Aug 28 15:31:37 2026") confirms it is local time.
const klistTimeLayout = "20060102150405"

// TicketStatus summarizes the state of a single profile's credential cache.
type TicketStatus struct {
	Exists    bool
	Principal string
	Realm     string
	StartTime time.Time
	EndTime   time.Time
	RenewTill time.Time
	Remaining time.Duration
	Expired   bool
}

// filePath returns the filesystem path behind a FILE: ccache spec, or "" when
// ccache names the default API cache.
func filePath(ccache string) string {
	path, _ := strings.CutPrefix(ccache, "FILE:")
	if path == ccache {
		return ""
	}
	return path
}

// ReadStatus loads the credential cache named by ccache (a normalized spec:
// "FILE:/path" or config.CCacheAPI) and reports its krbtgt ticket's validity.
// A missing cache is not an error: it yields TicketStatus{Exists: false}.
//
// FILE caches are parsed in-process with gokrb5. The API cache cannot be —
// it lives in GSSCred's memory, not in a file gokrb5 could open — so it is
// read by shelling out to klist, consistent with using kinit to write it.
func ReadStatus(ccache string) (TicketStatus, error) {
	if path := filePath(ccache); path != "" {
		return readFileStatus(path)
	}
	return readAPIStatus()
}

// readFileStatus parses a FILE ccache with gokrb5.
func readFileStatus(ccachePath string) (TicketStatus, error) {
	if _, err := os.Stat(ccachePath); errors.Is(err, os.ErrNotExist) {
		return TicketStatus{Exists: false}, nil
	}

	cc, err := credentials.LoadCCache(ccachePath)
	if err != nil {
		return TicketStatus{}, fmt.Errorf("loading ccache %s: %w", ccachePath, err)
	}

	realm := cc.GetClientRealm()
	entry, ok := cc.GetEntry(types.PrincipalName{
		NameType:   nametype.KRB_NT_SRV_INST,
		NameString: []string{"krbtgt", realm},
	})
	if !ok {
		return TicketStatus{Exists: true, Realm: realm}, fmt.Errorf("no krbtgt entry found in ccache %s", ccachePath)
	}

	remaining := time.Until(entry.EndTime)
	return TicketStatus{
		Exists:    true,
		Principal: cc.GetClientPrincipalName().PrincipalNameString() + "@" + realm,
		Realm:     realm,
		StartTime: entry.StartTime,
		EndTime:   entry.EndTime,
		RenewTill: entry.RenewTill,
		Remaining: remaining,
		Expired:   remaining <= 0,
	}, nil
}

// klistOutput is the shape of `klist --json`, verified against Heimdal on
// macOS 15:
//
//	{ "version": 1, "cache": "FILE:/…", "principal": "user@REALM",
//	  "tickets": [{"Issued": "20260828053139", "Expires": "20260828153137",
//	               "Principal": "krbtgt/REALM@REALM"}] }
//
// "Renew till" is absent for non-renewable tickets, which is what this tool
// acquires.
type klistOutput struct {
	Principal string        `json:"principal"`
	Tickets   []klistTicket `json:"tickets"`
}

type klistTicket struct {
	Issued    string `json:"Issued"`
	Expires   string `json:"Expires"`
	Principal string `json:"Principal"`
	RenewTill string `json:"Renew till"`
}

// readAPIStatus reads the default GSSCred cache via klist.
func readAPIStatus() (TicketStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()

	// No -c: klist resolves the default cache, matching how Acquire writes it
	// and how GSS-API consumers read it.
	cmd := exec.CommandContext(ctx, klistPath, "--json")
	cmd.Env = kinitEnv(AcquireOptions{})
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Run(); err != nil {
		// klist exits non-zero when no cache exists yet, which is the normal
		// pre-first-kinit state rather than a failure.
		if strings.Contains(strings.ToLower(stderr.String()), "cache not found") {
			return TicketStatus{Exists: false}, nil
		}
		return TicketStatus{}, fmt.Errorf("klist failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return parseKlistJSON(stdout.Bytes())
}

// parseKlistJSON turns klist --json output into a TicketStatus. Split out
// from readAPIStatus so it is testable without a live GSSCred cache.
func parseKlistJSON(data []byte) (TicketStatus, error) {
	var out klistOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return TicketStatus{}, fmt.Errorf("parsing klist output: %w", err)
	}
	if len(out.Tickets) == 0 {
		return TicketStatus{Exists: false}, nil
	}

	realm := ""
	if idx := strings.LastIndex(out.Principal, "@"); idx >= 0 {
		realm = out.Principal[idx+1:]
	}

	tgt, ok := findTGT(out.Tickets, realm)
	if !ok {
		return TicketStatus{Exists: true, Principal: out.Principal, Realm: realm},
			fmt.Errorf("no krbtgt entry found in default API cache for %s", out.Principal)
	}

	endTime, err := parseKlistTime(tgt.Expires)
	if err != nil {
		return TicketStatus{}, fmt.Errorf("parsing ticket expiry %q: %w", tgt.Expires, err)
	}
	// Issued and Renew till are best-effort: a ticket is usable without them,
	// so a parse failure degrades the display rather than failing the check.
	startTime, _ := parseKlistTime(tgt.Issued)
	renewTill, _ := parseKlistTime(tgt.RenewTill)

	remaining := time.Until(endTime)
	return TicketStatus{
		Exists:    true,
		Principal: out.Principal,
		Realm:     realm,
		StartTime: startTime,
		EndTime:   endTime,
		RenewTill: renewTill,
		Remaining: remaining,
		Expired:   remaining <= 0,
	}, nil
}

// findTGT picks the krbtgt/REALM@REALM entry out of a cache's tickets,
// falling back to any krbtgt service ticket when the realm is unknown.
func findTGT(tickets []klistTicket, realm string) (klistTicket, bool) {
	if realm != "" {
		want := "krbtgt/" + realm + "@" + realm
		for _, t := range tickets {
			if t.Principal == want {
				return t, true
			}
		}
	}
	for _, t := range tickets {
		if strings.HasPrefix(t.Principal, "krbtgt/") {
			return t, true
		}
	}
	return klistTicket{}, false
}

func parseKlistTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("empty timestamp")
	}
	return time.ParseInLocation(klistTimeLayout, s, time.Local)
}
