// agfix — cross-platform client-side patcher for Google Antigravity (Desktop, IDE and the `agy` CLI).
//
// What it does, and why it is safe/reversible:
//
//   - The Antigravity language server / CLI asks Google whether the account may use the product and,
//     when the answer is a region refusal, the binary matches the literal field name "ineligible" in
//     the reply and stops. We rename that literal to "inexigible" (both are exactly 10 bytes), so the
//     client no longer recognises the refusal field and proceeds. The file size and every other byte
//     are unchanged, and the edit round-trips byte-for-byte (`agfix unpatch`).
//
//   - This only removes a *client-side* geo gate. The actual API calls still have to leave the machine
//     from a permitted region — that is the VPN/WARP egress job, handled on the server side. agfix does
//     not touch the network, DNS, certificates or system proxy.
//
//   - A tiny background watchdog re-applies the rename after Antigravity auto-updates (an update ships a
//     fresh, unpatched binary). It only ever writes a file that still contains the stock "ineligible"
//     literal and is stable (fully written), and it leaves unknown builds alone.
//
// Subcommands: status | patch | unpatch | watch | install | uninstall | version
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const (
	version = "1.0.0"

	stockLiteral   = "ineligible"
	patchedLiteral = "inexigible"
)

func init() {
	if len(stockLiteral) != len(patchedLiteral) {
		panic("patch literals must be the same length")
	}
}

func main() {
	cmd := "status"
	if len(os.Args) > 1 {
		cmd = strings.TrimLeft(os.Args[1], "-")
	}
	var err error
	switch cmd {
	case "status", "st":
		cmdStatus()
	case "patch", "p":
		err = cmdPatch()
	case "unpatch", "revert", "u":
		err = cmdUnpatch()
	case "watch":
		err = cmdWatch()
	case "install", "i":
		err = cmdInstall()
	case "uninstall", "remove":
		err = cmdUninstall()
	case "version", "v":
		fmt.Printf("agfix %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	case "help", "h", "":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`agfix — Antigravity client-side region-gate patcher

Usage:
  agfix status      Show installs found and whether each binary is patched
  agfix patch       Patch every Antigravity binary found (reversible)
  agfix unpatch     Revert every binary to its original bytes
  agfix watch       Run the re-patch watchdog in the foreground
  agfix install     Patch now, then install the background watchdog service
  agfix uninstall   Stop/remove the watchdog service, then revert all patches
  agfix version     Print version

The patch renames the 10-byte literal "ineligible" to "inexigible" in the
language server / CLI. Same length, every other byte untouched, fully reversible.
It removes only the client-side location check; a permitted-region network exit
(your VPN/WARP) is still required for the API itself.
`)
}

// ---- target discovery -------------------------------------------------------

// fileState classifies one candidate binary.
type fileState int

const (
	stateMissing    fileState = iota // path does not exist
	statePatched                     // contains patchedLiteral, no stockLiteral
	stateStock                       // contains stockLiteral (needs patch)
	statePartial                     // contains both (interrupted write / partial)
	stateUnknown                     // readable but neither literal present (new build?)
	stateUnreadable                  // exists but could not be read
)

func (s fileState) String() string {
	switch s {
	case statePatched:
		return "patched"
	case stateStock:
		return "NOT patched"
	case statePartial:
		return "partial"
	case stateUnknown:
		return "unknown build (no signature)"
	case stateUnreadable:
		return "unreadable"
	default:
		return "missing"
	}
}

// inspect reads a file and reports whether it carries our literals.
func inspect(path string) fileState {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return stateMissing
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return stateUnreadable
	}
	hasStock := bytes.Contains(data, []byte(stockLiteral))
	hasPatched := bytes.Contains(data, []byte(patchedLiteral))
	switch {
	case hasStock && hasPatched:
		return statePartial
	case hasStock:
		return stateStock
	case hasPatched:
		return statePatched
	default:
		return stateUnknown
	}
}

// candidateRoots returns install roots to probe, platform-specific.
func candidateRoots() []string {
	home, _ := os.UserHomeDir()
	var roots []string
	add := func(p ...string) {
		if j := filepath.Join(p...); j != "" {
			roots = append(roots, j)
		}
	}
	switch runtime.GOOS {
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		pf := os.Getenv("PROGRAMFILES")
		pf86 := os.Getenv("PROGRAMFILES(X86)")
		add(local, "Programs", "Antigravity")
		add(local, "Programs", "Antigravity IDE")
		add(pf, "Antigravity")
		add(pf, "Antigravity IDE")
		add(pf86, "Antigravity")
		add(pf86, "Antigravity IDE")
		add(local, "Antigravity")
		add(local, "Antigravity IDE")
		add(local, "agy")
		add(local, "agy", "bin")
		if home != "" {
			add(home, ".gemini", "bin")
		}
	case "darwin":
		add("/Applications/Antigravity.app/Contents")
		add("/Applications/Antigravity IDE.app/Contents")
		if home != "" {
			add(home, "Applications/Antigravity.app/Contents")
			add(home, "Applications/Antigravity IDE.app/Contents")
			add(home, ".gemini", "bin")
			add(home, ".local", "bin")
			add(home, "bin")
		}
		add("/usr/local/bin")
	default: // linux and friends
		for _, base := range []string{"/opt", "/usr/share", "/usr/lib", "/usr/local/share"} {
			for _, name := range []string{"Antigravity", "Antigravity IDE", "antigravity", "antigravity-ide"} {
				add(base, name)
			}
		}
		if home != "" {
			add(home, ".local/share/Antigravity")
			add(home, ".local/share/Antigravity IDE")
			add(home, ".local/bin")
			add(home, ".gemini", "bin")
			add(home, "bin")
		}
	}
	return roots
}

// binaryNames returns the binary basenames to look for inside/near a root.
func binaryNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"agy.exe", "language_server.exe", "language_server_windows_x64.exe"}
	}
	return []string{"agy", "language_server", "language_server_linux_x64", "language_server_darwin_arm64", "language_server_darwin_x64"}
}

// discoverTargets returns the de-duplicated set of existing Antigravity binaries.
func discoverTargets() []string {
	seen := map[string]bool{}
	var out []string
	consider := func(p string) {
		if p == "" {
			return
		}
		if rp, err := filepath.Abs(p); err == nil {
			p = rp
		}
		// Canonicalise for the seen-set: follow symlinks, and fold case on case-insensitive filesystems.
		key := p
		if rp, err := filepath.EvalSymlinks(p); err == nil {
			key = rp
		}
		if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
			key = strings.ToLower(key)
		}
		if seen[key] {
			return
		}
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			seen[key] = true
			out = append(out, p)
		}
	}
	// Known subdirs where a language server lives under an install root.
	subdirs := [][]string{
		{},
		{"resources", "bin"},
		{"Resources", "bin"},
		{"resources", "app", "extensions", "antigravity", "bin"},
		{"Resources", "app", "extensions", "antigravity", "bin"},
	}
	for _, root := range candidateRoots() {
		for _, name := range binaryNames() {
			consider(filepath.Join(root, name))
			for _, sd := range subdirs {
				parts := append([]string{root}, sd...)
				parts = append(parts, name)
				consider(filepath.Join(parts...))
			}
		}
		// Shallow scan for language_server* and agy under the root (catches drifting suffixes).
		walkRoot(root, consider)
	}
	// Also follow `agy` on PATH.
	if p, err := lookPathAgy(); err == nil {
		consider(p)
	}
	sort.Strings(out)
	return out
}

// walkRoot does a bounded scan under root, feeding every agy/language_server* file to consider.
func walkRoot(root string, consider func(string)) {
	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil {
			return nil //nolint:nilerr // best-effort scan
		}
		if rel, rerr := filepath.Rel(root, p); rerr == nil && strings.Count(rel, string(os.PathSeparator)) > 6 {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		base := fi.Name()
		if strings.HasSuffix(base, ".agfixtmp") {
			return nil
		}
		if base == "agy" || base == "agy.exe" || strings.HasPrefix(base, "language_server") {
			consider(p)
		}
		return nil
	})
}
