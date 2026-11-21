package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// ---- status -----------------------------------------------------------------

func cmdStatus() {
	targets := discoverTargets()
	if len(targets) == 0 {
		fmt.Println("No Antigravity binaries found.")
		fmt.Println("Install Antigravity (and/or the agy CLI) first: https://antigravity.google/download")
		return
	}
	fmt.Printf("Found %d Antigravity binary(ies):\n", len(targets))
	for _, t := range targets {
		fmt.Printf("  [%-26s] %s\n", inspect(t), t)
	}
}

// ---- patch / unpatch --------------------------------------------------------

func cmdPatch() error {
	targets := discoverTargets()
	if len(targets) == 0 {
		return fmt.Errorf("no Antigravity binaries found (install it first: https://antigravity.google/download)")
	}
	var patched, already, skipped int
	for _, t := range targets {
		switch inspect(t) {
		case statePatched:
			fmt.Printf("  already patched: %s\n", t)
			already++
		case stateStock, statePartial:
			if err := rewrite(t, stockLiteral, patchedLiteral); err != nil {
				fmt.Printf("  FAILED: %s: %v\n", t, err)
				skipped++
				continue
			}
			resignIfNeeded(t)
			fmt.Printf("  patched: %s\n", t)
			patched++
		case stateUnknown:
			fmt.Printf("  skipped (unknown build, left untouched): %s\n", t)
			skipped++
		default:
			fmt.Printf("  skipped (%s): %s\n", inspect(t), t)
			skipped++
		}
	}
	fmt.Printf("\nDone: %d patched, %d already patched, %d skipped.\n", patched, already, skipped)
	if patched+already == 0 {
		return fmt.Errorf("nothing was patched")
	}
	return nil
}

func cmdUnpatch() error {
	targets := discoverTargets()
	var reverted int
	for _, t := range targets {
		if inspect(t) == statePatched || inspect(t) == statePartial {
			if err := rewrite(t, patchedLiteral, stockLiteral); err != nil {
				fmt.Printf("  FAILED: %s: %v\n", t, err)
				continue
			}
			resignIfNeeded(t)
			fmt.Printf("  reverted: %s\n", t)
			reverted++
		}
	}
	fmt.Printf("Done: %d reverted.\n", reverted)
	return nil
}

// rewrite replaces every occurrence of from with to (same length) and writes the file atomically,
// preserving the original mode. Safe to call on a running binary (temp + rename).
func rewrite(path, from, to string) error {
	if len(from) != len(to) {
		return fmt.Errorf("internal: length mismatch")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	n := bytes.Count(data, []byte(from))
	if n == 0 {
		return fmt.Errorf("literal %q not present", from)
	}
	data = bytes.ReplaceAll(data, []byte(from), []byte(to))
	return writeAtomic(path, data)
}

// writeAtomic writes data to a sibling temp file then renames it over path.
func writeAtomic(path string, data []byte) error {
	mode := os.FileMode(0o755)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	dir := filepath.Dir(path)
	tmp := filepath.Join(dir, fmt.Sprintf(".%s.%d.agfixtmp", filepath.Base(path), os.Getpid()))
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// ---- watchdog ---------------------------------------------------------------

// cmdWatch runs forever, re-applying the patch shortly after an update replaces a binary.
func cmdWatch() error {
	logf("watchdog started (%s/%s)", runtime.GOOS, runtime.GOARCH)
	const interval = 7 * time.Second
	// Remember each file's last size+mtime so we only act on a settled (fully written) file.
	type snap struct {
		size int64
		mod  time.Time
		seen int // consecutive stable observations
	}
	prev := map[string]snap{}
	for {
		for _, t := range discoverTargets() {
			fi, err := os.Stat(t)
			if err != nil {
				continue
			}
			cur := snap{size: fi.Size(), mod: fi.ModTime()}
			p := prev[t]
			if p.size == cur.size && p.mod.Equal(cur.mod) {
				cur.seen = p.seen + 1
			} else {
				cur.seen = 0
			}
			prev[t] = cur
			// Only touch a file that has been stable for two ticks (not mid-update).
			if cur.seen < 1 {
				continue
			}
			if inspect(t) == stateStock {
				if err := rewrite(t, stockLiteral, patchedLiteral); err != nil {
					logf("re-patch FAILED %s: %v", t, err)
					continue
				}
				resignIfNeeded(t)
				logf("re-patched after update: %s", t)
				// refresh snapshot after our own write
				if fi2, e := os.Stat(t); e == nil {
					prev[t] = snap{size: fi2.Size(), mod: fi2.ModTime(), seen: 0}
				}
			}
		}
		time.Sleep(interval)
	}
}

func logf(format string, a ...any) {
	fmt.Printf("%s agfix: %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, a...))
}
