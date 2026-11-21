# agfix — unlock Google Antigravity from a blocked region

`agfix` is a small cross-platform tool (Windows / Linux / macOS) that removes the **client-side region
check** in Google Antigravity — the **Desktop app**, the standalone **IDE**, and the **`agy` CLI** — and
then keeps it removed across auto-updates via a lightweight background watchdog.

> **You still need a VPN.** `agfix` only disables the check *inside the client*. The Antigravity / Gemini
> API itself still refuses requests that arrive from a blocked region, so your traffic must exit through a
> VPN/proxy whose endpoint is in a region where Antigravity is available. Configuring that VPN is **out of
> scope for this tool** — bring your own. Without it, the patch alone is not enough.

- [What it does and why it's safe](#what-it-does-and-why-its-safe)
- [Install (per OS)](#install)
- [For an automated agent](#for-an-automated-agent)
- [Verify / uninstall](#verify)
- [Build from source](#build-from-source)

---

## What it does and why it's safe

- The Antigravity language server / CLI carries the literal field name **`ineligible`**, which it matches
  in the server's reply to detect a region refusal and stop. `agfix` renames that literal to
  **`inexigible`** — **exactly 10 bytes for 10 bytes**. The file size and every other byte are unchanged,
  and the edit round-trips **byte-for-byte** (`agfix unpatch` restores the original exactly).
- Only Antigravity's own binaries are touched. **No DNS, no certificates, no system proxy, no network
  settings.** Nothing is sent anywhere.
- Writes are atomic (temp file + rename), safe even while Antigravity is running.
- On macOS the binary is re-signed ad-hoc (`codesign --sign -`) and the bundle's quarantine flag is
  cleared — otherwise Apple Silicon refuses to launch a modified binary.
- A background **watchdog** re-applies the rename a few seconds after Antigravity auto-updates (an update
  ships a fresh, unpatched binary). It writes a file **only** when it again contains the stock
  `ineligible` literal and is fully written; an unknown new build (no signature) is left untouched.

It removes **only the client-side location gate**. A permitted-region network exit (your VPN) is still
required for the API itself.

---

## Install

Download the binary for your OS from [`dist/`](dist) (verify against [`dist/SHA256SUMS.txt`](dist/SHA256SUMS.txt)),
or [build from source](#build-from-source). Then run `install`, which patches everything found and sets up
the auto-repatch watchdog so the fix survives updates and reboots.

All commands are the same on every OS:

| Command | Action |
| --- | --- |
| `agfix status` | List Antigravity binaries found and whether each is patched |
| `agfix patch` | Patch every binary found (reversible) |
| `agfix unpatch` | Restore every binary to its original bytes |
| `agfix watch` | Run the re-patch watchdog in the foreground |
| `agfix install` | Patch now **and** install the background watchdog (survives updates + reboot) |
| `agfix uninstall` | Remove the watchdog, then revert all patches |
| `agfix version` | Print version |

### Windows

```powershell
.\agfix-windows-amd64.exe install
```

No admin needed (user-space paths only). The watchdog is a hidden launcher in the Startup folder
(`…\Startup\agfix-watchdog.vbs`); it starts at logon with no visible window.

### Linux

```sh
chmod +x agfix-linux-amd64
./agfix-linux-amd64 install
```

If Antigravity is installed under `/opt` (a `.deb`/`.rpm` package), run with `sudo` so the binary can be
written. The watchdog is installed as a `systemctl --user` unit (`agfix-watchdog.service`).
AppImage is not supported (the file is temporary, so it cannot be patched) — install the `.deb`/`.rpm` or
unpack the `tar.gz`.

### macOS (Apple Silicon or Intel)

```sh
chmod +x agfix-macos-arm64        # or agfix-macos-x64 on Intel
sudo ./agfix-macos-arm64 install
```

Launch Antigravity once manually and quit it first (so the quarantine flag is cleared on the real install,
not a temporary App-Translocation copy). The watchdog is a LaunchAgent (`com.agfix.watchdog`). If macOS
says the app is "damaged": System Settings → Privacy & Security → App Management → allow your terminal,
then rerun.

---

## For an automated agent

To set up Antigravity on a fresh machine non-interactively:

1. Ensure Antigravity (and/or the `agy` CLI) is installed: https://antigravity.google/download
2. Ensure a VPN/proxy with an exit in a permitted region is active (bring your own; not provided here).
3. Fetch the matching binary from `dist/` (check `dist/SHA256SUMS.txt`) **or** `go build ./cmd/agfix`.
4. Run `agfix install`. It is idempotent — safe to run repeatedly.
5. Confirm with `agfix status` (every row should read `patched`), then run a prompt through `agy` to
   confirm the network path works end to end.

Exit codes: `0` on success; non-zero if nothing could be patched (e.g. Antigravity not found).

---

## Verify

```
agfix status
```

Then run `agy` (or the IDE) and send a prompt. If the model answers, the patch and the network are both
good. If it times out or refuses by region, the problem is the network side (your VPN / exit region), not
the patch.

## Uninstall

```
agfix uninstall
```

Removes the watchdog and restores every binary to its original bytes. (After an Antigravity update the
files are stock anyway.)

---

## Build from source

Requires Go 1.21+. No external dependencies.

```sh
go build -trimpath -ldflags "-s -w" -o agfix ./cmd/agfix
# cross-compile, e.g.:
GOOS=linux  GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o agfix-linux-amd64 ./cmd/agfix
GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o agfix-macos-arm64 ./cmd/agfix
```

---

## License / disclaimer

MIT (see [LICENSE](LICENSE)). A personal-use tool for reaching a **publicly free** developer tool from a
region where it is geo-blocked. Use at your own risk; this is against Google's Terms of Service. It is
**not** a bypass of any paid subscription — it only re-enables free access that Google geo-restricted. The
bypass mechanism (renaming the literal) was found by
[confeden/Antigravity](https://github.com/confeden/Antigravity); this implementation was written
independently.
