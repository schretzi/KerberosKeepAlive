package krb

import (
	"fmt"
	"testing"
	"time"
)

// realWorldKlistJSON is verbatim output from `klist --json -c FILE:...` on
// macOS 15 (Heimdal). It pins the exact key names and timestamp format this
// package parses, so a change in either is caught here rather than in
// production. The ticket is long expired; only the shape matters.
const realWorldKlistJSON = `{ "version" : 1, "cache" : "FILE:/Users/jdoe/.krb5cc/corp", ` +
	`"principal" : "jdoe@CORP.EXAMPLE.COM", ` +
	`"tickets" : [{"Issued" : "20260828053139","Expires" : "20260828153137",` +
	`"Principal" : "krbtgt/CORP.EXAMPLE.COM@CORP.EXAMPLE.COM"}]}`

func TestParseKlistJSONRealWorldShape(t *testing.T) {
	st, err := parseKlistJSON([]byte(realWorldKlistJSON))
	if err != nil {
		t.Fatalf("parseKlistJSON returned error: %v", err)
	}
	if !st.Exists {
		t.Error("Exists = false, want true")
	}
	if got, want := st.Principal, "jdoe@CORP.EXAMPLE.COM"; got != want {
		t.Errorf("Principal = %q, want %q", got, want)
	}
	if got, want := st.Realm, "CORP.EXAMPLE.COM"; got != want {
		t.Errorf("Realm = %q, want %q", got, want)
	}
	// Heimdal emits local time with no zone suffix, so the parse must be
	// local too: interpreting it as UTC would skew every remaining-time
	// calculation by the machine's offset.
	want := time.Date(2026, 8, 28, 15, 31, 37, 0, time.Local)
	if !st.EndTime.Equal(want) {
		t.Errorf("EndTime = %v, want %v", st.EndTime, want)
	}
	if got, want := st.StartTime, time.Date(2026, 8, 28, 5, 31, 39, 0, time.Local); !got.Equal(want) {
		t.Errorf("StartTime = %v, want %v", got, want)
	}
	// No "Renew till" key: these tickets are acquired non-renewable.
	if !st.RenewTill.IsZero() {
		t.Errorf("RenewTill = %v, want zero for a non-renewable ticket", st.RenewTill)
	}
}

func TestParseKlistJSONExpiry(t *testing.T) {
	cases := []struct {
		name        string
		offset      time.Duration
		wantExpired bool
	}{
		{"valid", 2 * time.Hour, false},
		{"expired", -2 * time.Hour, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := klistJSONExpiring(time.Now().Add(tc.offset))
			st, err := parseKlistJSON([]byte(data))
			if err != nil {
				t.Fatalf("parseKlistJSON returned error: %v", err)
			}
			if st.Expired != tc.wantExpired {
				t.Errorf("Expired = %v, want %v (remaining %v)", st.Expired, tc.wantExpired, st.Remaining)
			}
		})
	}
}

// An empty cache is the normal state before the first kinit, not an error.
func TestParseKlistJSONNoTickets(t *testing.T) {
	st, err := parseKlistJSON([]byte(`{ "version" : 1, "tickets" : [] }`))
	if err != nil {
		t.Fatalf("parseKlistJSON returned error: %v", err)
	}
	if st.Exists {
		t.Error("Exists = true, want false for a cache with no tickets")
	}
}

// A cache holding only service tickets has no TGT to track expiry against.
func TestParseKlistJSONNoTGT(t *testing.T) {
	data := `{ "version" : 1, "principal" : "jdoe@CORP.EXAMPLE.COM", "tickets" : ` +
		`[{"Issued" : "20260828053139","Expires" : "20260828153137",` +
		`"Principal" : "HTTP/proxy.corp.example.com@CORP.EXAMPLE.COM"}]}`
	if _, err := parseKlistJSON([]byte(data)); err == nil {
		t.Error("parseKlistJSON accepted a cache with no krbtgt entry, want error")
	}
}

func TestParseKlistJSONMalformed(t *testing.T) {
	if _, err := parseKlistJSON([]byte(`not json`)); err == nil {
		t.Error("parseKlistJSON accepted malformed input, want error")
	}
}

func TestFilePath(t *testing.T) {
	cases := []struct{ ccache, want string }{
		{"FILE:/Users/jdoe/.krb5cc/corp", "/Users/jdoe/.krb5cc/corp"},
		{"API", ""},
	}
	for _, tc := range cases {
		if got := filePath(tc.ccache); got != tc.want {
			t.Errorf("filePath(%q) = %q, want %q", tc.ccache, got, tc.want)
		}
	}
}

func klistJSONExpiring(at time.Time) string {
	return fmt.Sprintf(`{ "version" : 1, "principal" : "jdoe@CORP.EXAMPLE.COM", "tickets" : `+
		`[{"Issued" : "%s","Expires" : "%s","Principal" : "krbtgt/CORP.EXAMPLE.COM@CORP.EXAMPLE.COM"}]}`,
		at.Add(-time.Hour).Format(klistTimeLayout), at.Format(klistTimeLayout))
}
