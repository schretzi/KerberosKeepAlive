package krb

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestKinitArgs(t *testing.T) {
	cases := []struct {
		name string
		opts AcquireOptions
		want []string
	}{
		{
			name: "file ccache with lifetime",
			opts: AcquireOptions{
				Principal: "jdoe@CORP.EXAMPLE.COM",
				CCache:    "FILE:/Users/jdoe/.krb5cc/corp",
				Lifetime:  3 * time.Hour,
			},
			want: []string{
				passwordFileStdin, "--lifetime=10800s",
				"-c", "FILE:/Users/jdoe/.krb5cc/corp", "jdoe@CORP.EXAMPLE.COM",
			},
		},
		{
			// The API cache is reached by omitting -c entirely, so kinit
			// writes the default GSSCred cache that GSS consumers resolve.
			name: "api ccache omits -c",
			opts: AcquireOptions{
				Principal: "jdoe@CORP.EXAMPLE.COM",
				CCache:    "API",
				Lifetime:  3 * time.Hour,
			},
			want: []string{passwordFileStdin, "--lifetime=10800s", "jdoe@CORP.EXAMPLE.COM"},
		},
		{
			// Zero lifetime means "whatever the realm's default is".
			name: "no lifetime omits --lifetime",
			opts: AcquireOptions{Principal: "jdoe@CORP.EXAMPLE.COM", CCache: "API"},
			want: []string{passwordFileStdin, "jdoe@CORP.EXAMPLE.COM"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := kinitArgs(tc.opts); !slices.Equal(got, tc.want) {
				t.Errorf("kinitArgs() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Renewability is the property that actually bounds a stolen ticket's useful
// life, so its absence from argv is worth asserting rather than assuming.
func TestKinitArgsNeverRequestsRenewable(t *testing.T) {
	args := kinitArgs(AcquireOptions{
		Principal: "jdoe@CORP.EXAMPLE.COM",
		CCache:    "FILE:/tmp/cc",
		Lifetime:  time.Hour,
	})
	for _, arg := range args {
		if strings.Contains(arg, "renew") {
			t.Errorf("kinitArgs() produced %q, want no renewable request", arg)
		}
	}
}

func TestIsCredentialRejection(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
		want   bool
	}{
		{"preauth", "kinit: krb5_get_init_creds: Preauthentication failed", true},
		{"revoked", "kinit: Client's credentials have been revoked", true},
		{"expired password", "kinit: Password has expired", true},
		{"unknown principal", "kinit: Client unknown while getting initial credentials", true},
		{"case insensitive", "KINIT: PREAUTHENTICATION FAILED", true},
		// The whole point of the distinction: an unreachable KDC must stay
		// retryable, since that is the normal off-VPN state.
		{"kdc unreachable", "kinit: krb5_get_init_creds: unable to reach any KDC in realm CORP.EXAMPLE.COM", false},
		{"clock skew", "kinit: krb5_get_init_creds: Clock skew too great", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isCredentialRejection(tc.stderr); got != tc.want {
				t.Errorf("isCredentialRejection(%q) = %v, want %v", tc.stderr, got, tc.want)
			}
		})
	}
}

// A KRB5CCNAME inherited from launchd or a shell would silently redirect the
// ticket away from the configured cache, so it must never reach kinit.
func TestKinitEnvStripsKRB5CCNAME(t *testing.T) {
	t.Setenv("KRB5CCNAME", "FILE:/tmp/somewhere-else")
	for _, kv := range kinitEnv(AcquireOptions{}) {
		if strings.HasPrefix(kv, "KRB5CCNAME=") {
			t.Fatalf("kinitEnv() leaked %q into kinit's environment", kv)
		}
	}
}

func TestKinitEnvSetsKrb5Config(t *testing.T) {
	env := kinitEnv(AcquireOptions{Krb5ConfPath: "/etc/krb5.conf"})
	if !slices.Contains(env, "KRB5_CONFIG=/etc/krb5.conf") {
		t.Error("kinitEnv() did not set KRB5_CONFIG")
	}
	if slices.Contains(kinitEnv(AcquireOptions{}), "KRB5_CONFIG=") {
		t.Error("kinitEnv() set an empty KRB5_CONFIG")
	}
}
