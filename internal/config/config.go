package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level YAML configuration.
type Config struct {
	Krb5ConfPath string       `yaml:"krb5_conf_path"`
	Daemon       DaemonConfig `yaml:"daemon"`
	Profiles     []Profile    `yaml:"profiles"`
}

// DaemonConfig configures the `daemon` subcommand's poll loop.
type DaemonConfig struct {
	PollInterval Duration  `yaml:"poll_interval"`
	Log          LogConfig `yaml:"log"`
}

// LogConfig configures the daemon's own log file. This is separate from the
// launchd-captured stderr file, which only ever receives crash output (panics,
// and failures before logging is set up).
//
// There are no rotation knobs here: rotation belongs to newsyslog, configured
// in MacbookSetup under etc/newsyslog.d/kerberoskeepalive.conf. The daemon's
// only obligation is to notice when newsyslog has rotated the file out from
// under it, which internal/logfile handles.
type LogConfig struct {
	Path string `yaml:"path"`
}

// Profile is one managed Kerberos ticket: where its password comes from and
// where its credential cache should be written.
type Profile struct {
	Name      string      `yaml:"name"`
	Principal string      `yaml:"principal"`
	Keychain  KeychainRef `yaml:"keychain"`

	// CCache names the credential cache to write, either CCacheAPI (the
	// default in-memory GSSCred cache) or "FILE:/absolute/path". A bare
	// absolute path is accepted and normalized to a FILE: spec.
	CCache string `yaml:"ccache"`

	// CCachePath is the pre-API spelling of CCache, kept so configs written
	// before API caches were supported keep loading. When CCache is unset
	// this is normalized into CCache as a FILE: spec.
	//
	// Deprecated: set CCache instead.
	CCachePath string `yaml:"ccache_path"`

	// TicketLifetime is passed to kinit as --lifetime. Optional: when zero,
	// kinit is left to request the KDC's default (commonly 10h). Shortening
	// it caps how long a stolen ccache stays usable, since these tickets are
	// non-renewable — the daemon re-acquires from the Keychain password
	// rather than renewing, so there is no renew_till ceiling to work around.
	TicketLifetime Duration `yaml:"ticket_lifetime"`

	// RefreshThreshold is how much remaining validity triggers a re-acquire.
	// It must stay well below TicketLifetime: the daemon re-acquires every
	// (lifetime - threshold), so a threshold near the lifetime turns every
	// poll into a fresh AS-REQ.
	RefreshThreshold Duration `yaml:"refresh_threshold"`
}

// CCacheAPI is the ccache spec selecting the user's default GSSCred cache:
// the in-memory, per-session cache macOS uses by default, held by the
// GSSCred daemon rather than written to disk.
//
// It is spelled without a UUID deliberately. macOS addresses individual API
// caches by UUID (klist rejects "API:corp" with "failed to parse uuid"), so
// there is no stable human-chosen name to target. Writing the *default*
// cache is also what makes this useful: GSS-API consumers such as Alpaca ask
// for the default credential, so they pick the ticket up with no KRB5CCNAME
// plumbing. Only one profile may claim it — see Config.Validate.
const CCacheAPI = "API"

// UsesAPICache reports whether p targets the default GSSCred cache rather
// than a file.
func (p Profile) UsesAPICache() bool { return p.CCache == CCacheAPI }

// CCacheFilePath returns the filesystem path behind p's FILE: ccache, or ""
// when p uses the API cache.
func (p Profile) CCacheFilePath() string {
	if p.UsesAPICache() {
		return ""
	}
	return strings.TrimPrefix(p.CCache, "FILE:")
}

// normalizeCCache canonicalizes a configured ccache spec into either
// CCacheAPI or "FILE:/absolute/path".
func normalizeCCache(spec string) (string, error) {
	switch {
	case spec == CCacheAPI, spec == "API:":
		return CCacheAPI, nil
	case strings.HasPrefix(spec, "API:"):
		return "", fmt.Errorf("ccache %q: macOS addresses API caches by UUID, so a named one cannot be targeted; use %q for the default GSSCred cache", spec, CCacheAPI)
	case strings.HasPrefix(spec, "FILE:"):
		path := strings.TrimPrefix(spec, "FILE:")
		if !filepath.IsAbs(path) {
			return "", fmt.Errorf("ccache %q: FILE: path must be absolute", spec)
		}
		return "FILE:" + path, nil
	case filepath.IsAbs(spec):
		return "FILE:" + spec, nil
	}
	return "", fmt.Errorf("ccache %q: unsupported form, use %q, FILE:/absolute/path, or /absolute/path", spec, CCacheAPI)
}

// KeychainRef identifies an existing macOS Keychain generic-password item to
// read the profile's password from. This tool never writes to the Keychain.
type KeychainRef struct {
	Service string `yaml:"service"`
	Account string `yaml:"account"`
}

// Realm returns the realm portion of the profile's principal (after the last "@").
func (p Profile) Realm() string {
	idx := strings.LastIndex(p.Principal, "@")
	if idx < 0 {
		return ""
	}
	return p.Principal[idx+1:]
}

// Duration wraps time.Duration so YAML string values like "30m" parse via
// time.ParseDuration instead of yaml.v3's default integer-nanoseconds handling.
type Duration time.Duration

func (d Duration) Duration() time.Duration { return time.Duration(d) }

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

const defaultPollInterval = 60 * time.Second

// DefaultPath returns the conventional config path, written with a literal
// leading "~" so it stays portable in --help output and generated docs
// (ExpandPath resolves it against the real home directory at use time).
func DefaultPath() string {
	return "~/.config/kerberoskeepalive/config.yaml"
}

// DefaultLogPath returns the conventional daemon log path, written with a
// literal leading "~" for the same reason as DefaultPath.
//
// Flat in ~/Library/Logs and named after the binary, per
// MacbookSetup/CONVENTIONS.md — not a per-project subdirectory.
func DefaultLogPath() string {
	return "~/Library/Logs/kerberoskeepalive.log"
}

// ExpandPath resolves a leading "~" or "~/..." in path against the current
// user's home directory. Paths without a leading "~" are returned unchanged.
func ExpandPath(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

// Load reads, parses and validates the config file at path (which may start
// with "~/").
func Load(path string) (*Config, error) {
	path, err := ExpandPath(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is the user's own --config flag, reading it is the point
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	cfg.applyDefaults()
	if err := cfg.normalize(); err != nil {
		return nil, fmt.Errorf("validating config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config %s: %w", path, err)
	}
	return &cfg, nil
}

// normalize canonicalizes each profile's ccache spec, folding the deprecated
// ccache_path spelling into CCache. It runs before Validate so the rest of
// the config package (and every consumer) only ever sees a normalized spec.
func (c *Config) normalize() error {
	for i := range c.Profiles {
		p := &c.Profiles[i]
		spec := p.CCache
		switch {
		case spec != "" && p.CCachePath != "":
			return fmt.Errorf("profile %q: set either ccache or ccache_path, not both", p.Name)
		case spec == "":
			spec = p.CCachePath
		}
		if spec == "" {
			return fmt.Errorf("profile %q: ccache is required", p.Name)
		}
		normalized, err := normalizeCCache(spec)
		if err != nil {
			return fmt.Errorf("profile %q: %w", p.Name, err)
		}
		p.CCache, p.CCachePath = normalized, ""
	}
	return nil
}

// applyDefaults fills in unset optional fields. Zero means "not configured"
// for every field here, so an explicit 0 in YAML is indistinguishable from
// omission — deliberate, since 0 is not a useful value for any of them.
func (c *Config) applyDefaults() {
	if c.Daemon.PollInterval.Duration() == 0 {
		c.Daemon.PollInterval = Duration(defaultPollInterval)
	}
	if c.Daemon.Log.Path == "" {
		c.Daemon.Log.Path = DefaultLogPath()
	}
}

// Validate checks the config for structural and semantic errors.
func (c *Config) Validate() error {
	if len(c.Profiles) == 0 {
		return errors.New("no profiles configured")
	}
	seen := make(map[string]bool, len(c.Profiles))
	var apiProfile string
	for i, p := range c.Profiles {
		if p.Name == "" {
			return fmt.Errorf("profile %d: name is required", i)
		}
		if seen[p.Name] {
			return fmt.Errorf("profile %q: duplicate name", p.Name)
		}
		seen[p.Name] = true
		if !strings.Contains(p.Principal, "@") {
			return fmt.Errorf("profile %q: principal %q must be of the form user@REALM", p.Name, p.Principal)
		}
		if p.Keychain.Service == "" || p.Keychain.Account == "" {
			return fmt.Errorf("profile %q: keychain.service and keychain.account are required", p.Name)
		}
		// Two profiles cannot both own the default GSSCred cache, and there
		// is no second one to give the loser: macOS keys API caches by UUID,
		// and GSS consumers resolve the default. Rejecting this is honest
		// about a limit of the platform rather than of this tool.
		if p.UsesAPICache() {
			if apiProfile != "" {
				return fmt.Errorf("profiles %q and %q both use ccache %q, but only one profile can own the default GSSCred cache; give one of them a FILE: ccache", apiProfile, p.Name, CCacheAPI)
			}
			apiProfile = p.Name
		}
		if p.RefreshThreshold.Duration() <= 0 {
			return fmt.Errorf("profile %q: refresh_threshold must be > 0", p.Name)
		}
		if p.TicketLifetime.Duration() < 0 {
			return fmt.Errorf("profile %q: ticket_lifetime must be > 0", p.Name)
		}
		// The daemon re-acquires every (lifetime - threshold). At threshold
		// >= lifetime that interval is zero or negative, so a freshly issued
		// ticket already reads as due for refresh and every poll fires a new
		// AS-REQ — a self-inflicted password spray against the KDC.
		if lifetime := p.TicketLifetime.Duration(); lifetime > 0 && p.RefreshThreshold.Duration() >= lifetime {
			return fmt.Errorf("profile %q: refresh_threshold (%s) must be less than ticket_lifetime (%s), otherwise every poll re-acquires", p.Name, p.RefreshThreshold.Duration(), lifetime)
		}
	}
	return nil
}

// SelectProfiles returns the profiles matching names, in the order given, or
// every configured profile if names is empty.
func (c *Config) SelectProfiles(names []string) ([]Profile, error) {
	if len(names) == 0 {
		return c.Profiles, nil
	}
	byName := make(map[string]Profile, len(c.Profiles))
	for _, p := range c.Profiles {
		byName[p.Name] = p
	}
	selected := make([]Profile, 0, len(names))
	for _, n := range names {
		p, ok := byName[n]
		if !ok {
			return nil, fmt.Errorf("unknown profile %q", n)
		}
		selected = append(selected, p)
	}
	return selected, nil
}
