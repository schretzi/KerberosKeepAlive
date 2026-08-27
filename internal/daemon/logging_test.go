package daemon

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kerberoskeepalive/internal/config"
)

func logCfg(t *testing.T, path string, maxSizeMB, maxBackups int) *config.Config {
	t.Helper()
	compress := false
	return &config.Config{
		Daemon: config.DaemonConfig{
			Log: config.LogConfig{
				Path:       path,
				MaxSizeMB:  maxSizeMB,
				MaxBackups: maxBackups,
				Compress:   &compress,
			},
		},
	}
}

// setupLogging replaces the standard logger's output, so every test here has
// to put it back or it leaks into the rest of the run.
func restoreDefaultLogger(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
}

func TestSetupLoggingWritesToConfiguredPath(t *testing.T) {
	restoreDefaultLogger(t)
	path := filepath.Join(t.TempDir(), "daemon.log")

	closer, err := setupLogging(logCfg(t, path, 5, 3))
	if err != nil {
		t.Fatalf("setupLogging: %v", err)
	}
	log.Print("hello from the daemon")
	if err := closer.Close(); err != nil {
		t.Fatalf("closing log: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	if !strings.Contains(string(data), "hello from the daemon") {
		t.Errorf("log file does not contain the logged line, got:\n%s", data)
	}
}

// The whole point of the feature: a KDC that stays unreachable logs forever,
// so the file must roll over instead of growing without bound.
func TestSetupLoggingRotatesAndPrunes(t *testing.T) {
	restoreDefaultLogger(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.log")

	const maxBackups = 2
	closer, err := setupLogging(logCfg(t, path, 1, maxBackups))
	if err != nil {
		t.Fatalf("setupLogging: %v", err)
	}

	// ~4 MB against a 1 MB cap, so it must roll several times and then start
	// discarding the oldest backups.
	line := strings.Repeat("x", 1024)
	for range 4 * 1024 {
		log.Print(line)
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("closing log: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading log dir: %v", err)
	}
	var active, backups int
	for _, e := range entries {
		switch {
		case e.Name() == "daemon.log":
			active++
		case strings.HasPrefix(e.Name(), "daemon-"):
			backups++
		default:
			t.Errorf("unexpected file in log dir: %s", e.Name())
		}
	}
	if active != 1 {
		t.Errorf("active log files = %d, want 1", active)
	}
	if backups == 0 {
		t.Errorf("no rotated backups produced; the log never rolled over")
	}
	if backups > maxBackups {
		t.Errorf("rotated backups = %d, want at most max_backups (%d)", backups, maxBackups)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat active log: %v", err)
	}
	if info.Size() > 2*1024*1024 {
		t.Errorf("active log is %d bytes, want it capped near 1 MB", info.Size())
	}
}

func TestSetupLoggingCreatesMissingLogDirectory(t *testing.T) {
	restoreDefaultLogger(t)
	path := filepath.Join(t.TempDir(), "nested", "subdir", "daemon.log")

	closer, err := setupLogging(logCfg(t, path, 5, 3))
	if err != nil {
		t.Fatalf("setupLogging: %v", err)
	}
	log.Print("x")
	if err := closer.Close(); err != nil {
		t.Fatalf("closing log: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("log file not created under a missing directory: %v", err)
	}
}
