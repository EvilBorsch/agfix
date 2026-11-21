//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func serviceBinaryName() string { return "agfix.exe" }

func agfixDataDir() string {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		home, _ := os.UserHomeDir()
		local = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(local, "Programs", "agfix")
}

func lookPathAgy() (string, error) { return exec.LookPath("agy") }

// resignIfNeeded is a no-op on Windows (no mandatory signature enforcement for these binaries).
func resignIfNeeded(string) {}

func startupDir() string {
	appdata := os.Getenv("APPDATA")
	return filepath.Join(appdata, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
}

func launcherVBSPath() string {
	return filepath.Join(startupDir(), "agfix-watchdog.vbs")
}

// installService writes a hidden Startup launcher (runs `agfix watch` at logon with no console window)
// and starts the watchdog immediately in the background.
func installService(exe string) error {
	vbs := fmt.Sprintf("Set s = CreateObject(\"WScript.Shell\")\r\ns.Run \"\"\"%s\"\" watch\", 0, False\r\n", exe)
	path := launcherVBSPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(vbs), 0o644); err != nil {
		return err
	}
	// Launch now, hidden/detached, via the same launcher.
	cmd := exec.Command("wscript.exe", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	fmt.Println("  startup launcher:", path)
	return nil
}

// removeService deletes the Startup launcher and stops any running `agfix watch` process.
func removeService() error {
	// Stop running watchdog instances (match on command line so we don't kill ourselves).
	ps := `Get-CimInstance Win32_Process -Filter "Name='agfix.exe'" | Where-Object { $_.CommandLine -like '* watch*' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }`
	kill := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = kill.Run()
	err := os.Remove(launcherVBSPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
