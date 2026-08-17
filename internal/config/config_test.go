package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const validYAML = `
krb5_conf_path: /etc/krb5.conf

daemon:
  poll_interval: 90s

profiles:
  - name: corp
    principal: jdoe@CORP.EXAMPLE.COM
    keychain:
      service: KerberosKeepAlive-corp
      account: jdoe
    ccache_path: /Users/jdoe/.krb5cc/corp
    refresh_threshold: 30m
`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return path
}

func TestLoadValid(t *testing.T) {
	path := writeTemp(t, validYAML)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got, want := cfg.Daemon.PollInterval.Duration(), 90*time.Second; got != want {
		t.Errorf("poll_interval = %v, want %v", got, want)
	}
	if len(cfg.Profiles) != 1 {
		t.Fatalf("len(Profiles) = %d, want 1", len(cfg.Profiles))
	}
	p := cfg.Profiles[0]
	if p.RefreshThreshold.Duration() != 30*time.Minute {
		t.Errorf("refresh_threshold = %v, want 30m", p.RefreshThreshold.Duration())
	}
	if p.Realm() != "CORP.EXAMPLE.COM" {
		t.Errorf("Realm() = %q, want CORP.EXAMPLE.COM", p.Realm())
	}
}

func TestLoadDefaultsPollInterval(t *testing.T) {
	yaml := `
profiles:
  - name: corp
    principal: jdoe@CORP.EXAMPLE.COM
    keychain:
      service: svc
      account: acc
    ccache_path: /tmp/ccache
    refresh_threshold: 10m
`
	cfg, err := Load(writeTemp(t, yaml))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got, want := cfg.Daemon.PollInterval.Duration(), defaultPollInterval; got != want {
		t.Errorf("poll_interval default = %v, want %v", got, want)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{"no profiles", `profiles: []`},
		{"missing name", `
profiles:
  - principal: jdoe@REALM
    keychain: {service: s, account: a}
    ccache_path: /tmp/x
    refresh_threshold: 5m
`},
		{"duplicate name", `
profiles:
  - name: corp
    principal: jdoe@REALM
    keychain: {service: s, account: a}
    ccache_path: /tmp/x
    refresh_threshold: 5m
  - name: corp
    principal: jdoe@REALM2
    keychain: {service: s2, account: a2}
    ccache_path: /tmp/y
    refresh_threshold: 5m
`},
		{"principal missing realm", `
profiles:
  - name: corp
    principal: jdoe
    keychain: {service: s, account: a}
    ccache_path: /tmp/x
    refresh_threshold: 5m
`},
		{"missing keychain account", `
profiles:
  - name: corp
    principal: jdoe@REALM
    keychain: {service: s}
    ccache_path: /tmp/x
    refresh_threshold: 5m
`},
		{"relative ccache_path", `
profiles:
  - name: corp
    principal: jdoe@REALM
    keychain: {service: s, account: a}
    ccache_path: relative/path
    refresh_threshold: 5m
`},
		{"zero refresh_threshold", `
profiles:
  - name: corp
    principal: jdoe@REALM
    keychain: {service: s, account: a}
    ccache_path: /tmp/x
    refresh_threshold: 0s
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load(writeTemp(t, tc.yaml)); err == nil {
				t.Fatalf("Load() succeeded, want error")
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir: %v", err)
	}

	cases := []struct {
		in, want string
	}{
		{"~/.config/kerberoskeepalive/config.yaml", filepath.Join(home, ".config/kerberoskeepalive/config.yaml")},
		{"~", home},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~user/not-supported", "~user/not-supported"}, // only bare "~" and "~/" are expanded
	}
	for _, tc := range cases {
		got, err := ExpandPath(tc.in)
		if err != nil {
			t.Errorf("ExpandPath(%q) returned error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ExpandPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSelectProfiles(t *testing.T) {
	cfg, err := Load(writeTemp(t, `
profiles:
  - name: corp
    principal: jdoe@CORP
    keychain: {service: s1, account: a1}
    ccache_path: /tmp/corp
    refresh_threshold: 5m
  - name: labs
    principal: jdoe@LABS
    keychain: {service: s2, account: a2}
    ccache_path: /tmp/labs
    refresh_threshold: 5m
`))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	all, err := cfg.SelectProfiles(nil)
	if err != nil || len(all) != 2 {
		t.Fatalf("SelectProfiles(nil) = %v, %v, want 2 profiles", all, err)
	}

	one, err := cfg.SelectProfiles([]string{"labs"})
	if err != nil || len(one) != 1 || one[0].Name != "labs" {
		t.Fatalf("SelectProfiles([labs]) = %v, %v", one, err)
	}

	if _, err := cfg.SelectProfiles([]string{"nope"}); err == nil {
		t.Fatalf("SelectProfiles([nope]) succeeded, want error")
	}
}
