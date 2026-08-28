package daemon

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/schretzi/kerberoskeepalive/internal/config"
)

func logCfg(path string) *config.Config {
	return &config.Config{
		Daemon: config.DaemonConfig{
			Log: config.LogConfig{Path: path},
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
	path := filepath.Join(t.TempDir(), "kerberoskeepalive.log")

	closer, err := setupLogging(logCfg(path))
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

func TestSetupLoggingCreatesMissingLogDirectory(t *testing.T) {
	restoreDefaultLogger(t)
	path := filepath.Join(t.TempDir(), "nested", "subdir", "kerberoskeepalive.log")

	closer, err := setupLogging(logCfg(path))
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

// setupLogging must hand the standard logger a rotation-aware writer, not a
// plain file: newsyslog rotates by renaming, and a plain *os.File would go on
// filling the archive while the live log stayed empty.
func TestSetupLoggingSurvivesRotation(t *testing.T) {
	restoreDefaultLogger(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "kerberoskeepalive.log")

	closer, err := setupLogging(logCfg(path))
	if err != nil {
		t.Fatalf("setupLogging: %v", err)
	}
	defer closer.Close()

	log.Print("before rotation")
	if err := os.Rename(path, filepath.Join(dir, "kerberoskeepalive.log.0")); err != nil {
		t.Fatalf("simulating newsyslog rename: %v", err)
	}

	// logfile throttles its rotation check to once a second; wait it out
	// rather than reaching into the writer's internals from another package.
	time.Sleep(1100 * time.Millisecond)
	log.Print("after rotation")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("live log was not recreated after rotation: %v", err)
	}
	if !strings.Contains(string(data), "after rotation") {
		t.Errorf("post-rotation line did not land in the live log, got:\n%s", data)
	}
	if strings.Contains(string(data), "before rotation") {
		t.Errorf("live log unexpectedly contains pre-rotation output:\n%s", data)
	}
}
