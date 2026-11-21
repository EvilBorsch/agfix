package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// cmdInstall patches everything, installs the agfix binary to a stable location, and registers the
// background watchdog so the patch survives Antigravity auto-updates.
func cmdInstall() error {
	if err := cmdPatch(); err != nil {
		return err
	}
	exe, err := installedBinaryPath()
	if err != nil {
		return err
	}
	fmt.Printf("\nInstalling watchdog service (binary: %s)\n", exe)
	if err := installService(exe); err != nil {
		return fmt.Errorf("install watchdog: %w", err)
	}
	fmt.Println("Watchdog installed and started. It will re-apply the patch after each Antigravity update.")
	fmt.Println("Run `agfix status` any time to check, or `agfix uninstall` to remove everything.")
	return nil
}

// cmdUninstall removes the watchdog, then reverts every patch.
func cmdUninstall() error {
	if err := removeService(); err != nil {
		fmt.Fprintln(os.Stderr, "warning: remove watchdog:", err)
	} else {
		fmt.Println("Watchdog service removed.")
	}
	return cmdUnpatch()
}

// installedBinaryPath copies the running agfix executable into a per-user location the service can run
// from, and returns that path. Running the service straight from a temp/Downloads path is fragile, so
// we keep a stable copy.
func installedBinaryPath() (string, error) {
	src, err := os.Executable()
	if err != nil {
		return "", err
	}
	src, _ = filepath.Abs(src)
	dstDir := agfixDataDir()
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dstDir, serviceBinaryName())
	// If we are already running from the install location, keep it.
	if sameFile(src, dst) {
		return dst, nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	if err := writeAtomic(dst, data); err != nil {
		return "", err
	}
	if err := os.Chmod(dst, 0o755); err != nil {
		return "", err
	}
	return dst, nil
}

func sameFile(a, b string) bool {
	fa, err1 := os.Stat(a)
	fb, err2 := os.Stat(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return os.SameFile(fa, fb)
}
