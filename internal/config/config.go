package config

import (
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

// LogConfig configures the daemon's own rotating log file. This is separate
// from the launchd-captured stdout/stderr files, which only ever receive
// crash output (panics, and failures before logging is set up).
type LogConfig struct {
	Path       string `yaml:"path"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAgeDays int    `yaml:"max_age_days"`
	Compress   *bool  `yaml:"compress"`
}

// Profile is one managed Kerberos ticket: where its password comes from and
// where its credential cache should be written.
type Profile struct {
	Name             string      `yaml:"name"`
	Principal        string      `yaml:"principal"`
	Keychain         KeychainRef `yaml:"keychain"`
	CCachePath       string      `yaml:"ccache_path"`
	RefreshThreshold Duration    `yaml:"refresh_threshold"`
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

const (
	defaultPollInterval = 60 * time.Second

	defaultLogMaxSizeMB  = 5
	defaultLogMaxBackups = 3
	defaultLogMaxAgeDays = 28
)

// DefaultPath returns the conventional config path, written with a literal
// leading "~" so it stays portable in --help output and generated docs
// (ExpandPath resolves it against the real home directory at use time).
func DefaultPath() string {
	return "~/.config/kerberoskeepalive/config.yaml"
}

// DefaultLogPath returns the conventional daemon log path, written with a
// literal leading "~" for the same reason as DefaultPath.
func DefaultLogPath() string {
	return "~/Library/Logs/KerberosKeepAlive/daemon.log"
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
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config %s: %w", path, err)
	}
	return &cfg, nil
}

// applyDefaults fills in unset optional fields. Zero means "not configured"
// for every field here, so an explicit 0 in YAML is indistinguishable from
// omission — deliberate, since 0 is not a useful value for any of them.
// Compress is a *bool precisely because its default is true.
func (c *Config) applyDefaults() {
	if c.Daemon.PollInterval.Duration() == 0 {
		c.Daemon.PollInterval = Duration(defaultPollInterval)
	}
	if c.Daemon.Log.Path == "" {
		c.Daemon.Log.Path = DefaultLogPath()
	}
	if c.Daemon.Log.MaxSizeMB == 0 {
		c.Daemon.Log.MaxSizeMB = defaultLogMaxSizeMB
	}
	if c.Daemon.Log.MaxBackups == 0 {
		c.Daemon.Log.MaxBackups = defaultLogMaxBackups
	}
	if c.Daemon.Log.MaxAgeDays == 0 {
		c.Daemon.Log.MaxAgeDays = defaultLogMaxAgeDays
	}
	if c.Daemon.Log.Compress == nil {
		compress := true
		c.Daemon.Log.Compress = &compress
	}
}

// Validate checks the config for structural and semantic errors.
func (c *Config) Validate() error {
	if c.Daemon.Log.MaxSizeMB < 0 {
		return fmt.Errorf("daemon.log.max_size_mb must be >= 0")
	}
	if c.Daemon.Log.MaxBackups < 0 {
		return fmt.Errorf("daemon.log.max_backups must be >= 0")
	}
	if c.Daemon.Log.MaxAgeDays < 0 {
		return fmt.Errorf("daemon.log.max_age_days must be >= 0")
	}
	if len(c.Profiles) == 0 {
		return fmt.Errorf("no profiles configured")
	}
	seen := make(map[string]bool, len(c.Profiles))
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
		if !filepath.IsAbs(p.CCachePath) {
			return fmt.Errorf("profile %q: ccache_path must be an absolute path", p.Name)
		}
		if p.RefreshThreshold.Duration() <= 0 {
			return fmt.Errorf("profile %q: refresh_threshold must be > 0", p.Name)
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
