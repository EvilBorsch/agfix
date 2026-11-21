//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const launchLabel = "com.agfix.watchdog"

func serviceBinaryName() string { return "agfix" }

func agfixDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "agfix")
}

func lookPathAgy() (string, error) {
	p, err := exec.LookPath("agy")
	if err != nil {
		return "", err
	}
	if rp, e := filepath.EvalSymlinks(p); e == nil {
		return rp, nil
	}
	return p, nil
}

// resignIfNeeded re-signs a Mach-O ad hoc after we edit it; on Apple Silicon an edited binary with a
// stale signature is killed at exec. Best-effort: failure is reported but not fatal.
func resignIfNeeded(path string) {
	cmd := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", "--preserve-metadata=identifier", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Retry without preserving identifier (unsigned originals have none).
		cmd2 := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", path)
		if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
			logf("codesign warning for %s: %s / %s", path, string(out), string(out2))
		}
	}
	// If this binary lives inside an .app bundle, drop the quarantine flag so Gatekeeper
	// does not call the modified bundle "damaged" on first launch.
	if app := bundleOf(path); app != "" {
		_ = exec.Command("/usr/bin/xattr", "-dr", "com.apple.quarantine", app).Run()
	}
}

// bundleOf returns the enclosing *.app path for a binary, or "".
func bundleOf(path string) string {
	for d := filepath.Dir(path); d != "/" && d != "."; d = filepath.Dir(d) {
		if filepath.Ext(d) == ".app" {
			return d
		}
	}
	return ""
}

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchLabel+".plist")
}

func installService(exe string) error {
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key>
  <array><string>%s</string><string>watch</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
</dict>
</plist>
`, launchLabel, exe)
	path := plistPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return err
	}
	uid := fmt.Sprintf("gui/%d", os.Getuid())
	_ = exec.Command("launchctl", "bootout", uid+"/"+launchLabel).Run()
	if out, err := exec.Command("launchctl", "bootstrap", uid, path).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %s", string(out))
	}
	return nil
}

func removeService() error {
	uid := fmt.Sprintf("gui/%d", os.Getuid())
	_ = exec.Command("launchctl", "bootout", uid+"/"+launchLabel).Run()
	return os.Remove(plistPath())
}
