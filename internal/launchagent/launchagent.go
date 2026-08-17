// Package launchagent generates and (un)loads the macOS LaunchAgent plist
// that runs `kerberoskeepalive daemon` in the user's login session.
package launchagent

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"text/template"

	"kerberoskeepalive/internal/config"
)

const plistTemplateSrc = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label | xmlesc}}</string>

    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryPath | xmlesc}}</string>
        <string>daemon</string>
        <string>--config</string>
        <string>{{.ConfigPath | xmlesc}}</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>

    <key>WorkingDirectory</key>
    <string>{{.HomeDir | xmlesc}}</string>

    <key>StandardOutPath</key>
    <string>{{.LogDir | xmlesc}}/daemon.out.log</string>

    <key>StandardErrorPath</key>
    <string>{{.LogDir | xmlesc}}/daemon.err.log</string>

    <key>ProcessType</key>
    <string>Background</string>

    <key>ThrottleInterval</key>
    <integer>10</integer>
</dict>
</plist>
`

var plistTemplate = template.Must(template.New("launchagent-plist").Funcs(template.FuncMap{
	"xmlesc": func(s string) string {
		var buf bytes.Buffer
		_ = xml.EscapeText(&buf, []byte(s))
		return buf.String()
	},
}).Parse(plistTemplateSrc))

type plistData struct {
	Label      string
	BinaryPath string
	ConfigPath string
	HomeDir    string
	LogDir     string
}

// Label returns this LaunchAgent's reverse-DNS label, e.g.
// "com.jdoe.kerberoskeepalive".
func Label() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("determining current user: %w", err)
	}
	return fmt.Sprintf("com.%s.kerberoskeepalive", u.Username), nil
}

// PlistPath returns where the LaunchAgent plist is (to be) written:
// ~/Library/LaunchAgents/<label>.plist.
func PlistPath() (string, error) {
	label, err := Label()
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
}

func logDir(home string) string {
	return filepath.Join(home, "Library", "Logs", "KerberosKeepAlive")
}

func guiDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

func launchctl(args ...string) (string, error) {
	// #nosec G204 -- fixed binary name resolved via PATH; args are our own
	// constructed strings (domain/label/plist path), never a shell string,
	// so there's no injection surface.
	cmd := exec.Command("launchctl", args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return strings.TrimSpace(buf.String()), err
}

// Install renders the LaunchAgent plist (pointing at the currently running
// binary and configPath), writes it to PlistPath, and loads it via
// `launchctl bootstrap`. It is idempotent: if the agent is already loaded,
// it is unloaded first.
func Install(configPath string) error {
	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving own executable path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(binPath); err == nil {
		binPath = resolved
	}
	expandedConfigPath, err := config.ExpandPath(configPath)
	if err != nil {
		return fmt.Errorf("resolving config path %s: %w", configPath, err)
	}
	absConfigPath, err := filepath.Abs(expandedConfigPath)
	if err != nil {
		return fmt.Errorf("resolving config path %s: %w", configPath, err)
	}

	label, err := Label()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("determining home directory: %w", err)
	}

	data := plistData{
		Label:      label,
		BinaryPath: binPath,
		ConfigPath: absConfigPath,
		HomeDir:    home,
		LogDir:     logDir(home),
	}

	if err := os.MkdirAll(data.LogDir, 0o750); err != nil {
		return fmt.Errorf("creating log directory %s: %w", data.LogDir, err)
	}

	var buf bytes.Buffer
	if err := plistTemplate.Execute(&buf, data); err != nil {
		return fmt.Errorf("rendering plist: %w", err)
	}

	plistPath, err := PlistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o750); err != nil {
		return fmt.Errorf("creating LaunchAgents directory: %w", err)
	}
	if err := os.WriteFile(plistPath, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("writing plist %s: %w", plistPath, err)
	}

	// Idempotent reinstall: unload first if already loaded. A failure here
	// just means it wasn't loaded yet, which is fine.
	_, _ = launchctl("bootout", guiDomain()+"/"+label)

	if out, err := launchctl("bootstrap", guiDomain(), plistPath); err != nil {
		return fmt.Errorf("launchctl bootstrap failed: %w: %s", err, out)
	}
	return nil
}

// Uninstall unloads the LaunchAgent (if loaded) and removes its plist file.
func Uninstall() error {
	label, err := Label()
	if err != nil {
		return err
	}
	plistPath, err := PlistPath()
	if err != nil {
		return err
	}

	if out, err := launchctl("bootout", guiDomain()+"/"+label); err != nil {
		// Not currently loaded isn't a hard failure; still remove the plist.
		fmt.Fprintf(os.Stderr, "launchctl bootout: %v: %s\n", err, out)
	}

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing plist %s: %w", plistPath, err)
	}
	return nil
}
