//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const unitName = "agfix-watchdog.service"

func serviceBinaryName() string { return "agfix" }

func agfixDataDir() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "agfix")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "agfix")
}

// lookPathAgy resolves `agy` on PATH (symlinks followed).
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

// resignIfNeeded is a no-op on Linux (ELF binaries have no mandatory code signature).
func resignIfNeeded(string) {}

func userUnitPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", unitName)
}

func installService(exe string) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemd not found; run `%s watch` manually or add it to your autostart", exe)
	}
	unit := fmt.Sprintf(`[Unit]
Description=agfix Antigravity re-patch watchdog
After=default.target

[Service]
Type=simple
ExecStart=%s watch
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
`, exe)
	path := userUnitPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(unit), 0o644); err != nil {
		return err
	}
	run := func(args ...string) { _ = exec.Command("systemctl", append([]string{"--user"}, args...)...).Run() }
	run("daemon-reload")
	run("enable", "--now", unitName)
	out, err := exec.Command("systemctl", "--user", "is-active", unitName).CombinedOutput()
	fmt.Printf("  systemd --user %s: %s", unitName, string(out))
	_ = err
	return nil
}

func removeService() error {
	if _, err := exec.LookPath("systemctl"); err == nil {
		_ = exec.Command("systemctl", "--user", "disable", "--now", unitName).Run()
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	}
	return os.Remove(userUnitPath())
}
