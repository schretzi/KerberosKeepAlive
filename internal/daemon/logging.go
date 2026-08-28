package daemon

import (
	"fmt"
	"io"
	"log"

	"github.com/schretzi/kerberoskeepalive/internal/config"
	"github.com/schretzi/kerberoskeepalive/internal/logfile"
)

// setupLogging points the standard logger at the daemon's own log file.
//
// Rotation is newsyslog's job (etc/newsyslog.d/kerberoskeepalive.conf in
// MacbookSetup), not this process's: an unreachable KDC produces two lines per
// poll indefinitely, and newsyslog caps that by size. Because newsyslog
// rotates by renaming, the writer re-stats the path and reopens when the file
// moves — otherwise the daemon would keep writing into the archived inode and
// the live log would stay empty forever.
//
// launchd's StandardErrorPath still captures this process's raw stderr, but
// once this returns those files only receive output that bypasses the logger
// entirely: panics, and anything written before setup. This is the log to
// read.
//
// The returned closer flushes and releases the file.
func setupLogging(cfg *config.Config) (io.Closer, error) {
	path, err := config.ExpandPath(cfg.Daemon.Log.Path)
	if err != nil {
		return nil, fmt.Errorf("resolving daemon.log.path %s: %w", cfg.Daemon.Log.Path, err)
	}
	w, err := logfile.Open(path)
	if err != nil {
		return nil, err
	}
	log.SetOutput(w)
	return w, nil
}
