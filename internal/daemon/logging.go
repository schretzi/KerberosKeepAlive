package daemon

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/schretzi/kerberoskeepalive/internal/config"
)

// setupLogging points the standard logger at a size-rotating file so an
// unreachable KDC — which produces two lines per poll indefinitely — can't
// grow the log without bound.
//
// launchd's StandardOutPath/StandardErrorPath still capture this process's
// raw stdout/stderr, but once this returns, those files only receive output
// that bypasses the logger entirely: panics, and anything written before
// setup. The rotating file is the log to read.
//
// The returned closer flushes and releases the file.
func setupLogging(cfg *config.Config) (io.Closer, error) {
	path, err := config.ExpandPath(cfg.Daemon.Log.Path)
	if err != nil {
		return nil, fmt.Errorf("resolving daemon.log.path %s: %w", cfg.Daemon.Log.Path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("creating log directory %s: %w", filepath.Dir(path), err)
	}

	rotator := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    cfg.Daemon.Log.MaxSizeMB,
		MaxBackups: cfg.Daemon.Log.MaxBackups,
		MaxAge:     cfg.Daemon.Log.MaxAgeDays,
		Compress:   *cfg.Daemon.Log.Compress,
		LocalTime:  true,
	}
	log.SetOutput(rotator)
	return rotator, nil
}
