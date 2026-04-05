# i2tor

`i2tor` is a Go desktop launcher that composes Tor Browser and Java I2P without forking Tor Browser, patching its source, or touching system-wide proxy settings. The launcher owns a dedicated Tor Browser profile, generates a PAC file that sends `.i2p` traffic to the local I2P HTTP proxy on `127.0.0.1:4444`, and keeps all other browser traffic on Tor Browser's SOCKS proxy at `127.0.0.1:9150`.

## Why a launcher instead of a browser fork

The launcher approach keeps browser maintenance and security updates aligned with upstream Tor Browser. `i2tor` only manages process orchestration, profile isolation, PAC generation, managed installs, and fail-closed startup behavior for `.i2p` routing.

## Linux-first note

Linux is the primary implementation target in this repository. Windows and macOS code paths are isolated behind the same interfaces, but managed install behavior for those platforms is intentionally conservative and may require follow-up validation before production use.

## Build

```bash
go build ./cmd/i2tor
```

If your environment restricts the Go build cache, use:

```bash
env GOCACHE=/tmp/i2tor-gocache go build ./cmd/i2tor
```

## Run

```bash
./i2tor run
./i2tor install
./i2tor update
./i2tor status
./i2tor paths
./i2tor doctor
./i2tor logs
./i2tor gui
./i2tor desktop
./i2tor webui
./i2tor uninstall
```

`run` performs the full launch sequence:

1. acquire the single-instance lock
2. resolve paths and state
3. reuse or install Tor Browser
4. reuse or install Java 17+ if needed
5. reuse or install I2P
6. start I2P
7. wait for `127.0.0.1:4444`
8. start the bundled Tor daemon from the Tor Browser bundle
9. wait for `127.0.0.1:9150`
10. write `proxy.pac`
11. write the dedicated `user.js`
12. launch the browser binary directly with the dedicated profile
13. shut down only the Tor and I2P processes owned by the launcher

If I2P readiness fails, the browser is not launched.

## Managed directory layout

Linux defaults to `~/.local/share/i2tor/`.

```text
downloads/
runtime/
  tor-browser/
  i2p/
  java/
  profile/
  pac/
logs/
state/
```

Important files:

- `state/manifest.json`
- `state/config.json`
- `runtime/pac/proxy.pac`
- `runtime/profile/user.js`
- `logs/current.log`

## Uninstall

`i2tor uninstall` removes only the launcher-managed data directory and leaves reused external installs untouched.

```bash
./i2tor uninstall
```

On Linux this removes `~/.local/share/i2tor/` by default. The command refuses to proceed if it still sees a launcher-owned I2P process recorded as active.

## Configuration

Default config shape:

```json
{
  "reuse_existing_tor_browser": true,
  "reuse_existing_i2p": true,
  "auto_check_updates": true,
  "data_dir": "",
  "log_level": "info"
}
```

Sample files are in [testdata/config.sample.json](/home/seth/Documents/i2tor/testdata/config.sample.json) and [testdata/manifest.sample.json](/home/seth/Documents/i2tor/testdata/manifest.sample.json).

## Updates and GUI

`i2tor update` refreshes only launcher-managed dependencies. Reused external Tor Browser or I2P installs are reported but not modified.

`i2tor gui` is the primary Linux desktop entrypoint. It opens a native app window that checks install state, prompts for managed updates when available, and starts automatically when everything is already ready.

`i2tor desktop` is an alias for the same native desktop app path.

`i2tor webui` keeps the older browser-based control surface available as a fallback.

## AppImage

For the Linux-first product path, this repository now targets AppImage as the primary desktop package. The AppImage bundles:

- the `i2tor` launcher binary
- your `i2tor.png` icon
- an XDG desktop entry
- an `AppRun` wrapper that launches the native desktop GUI

Build it locally with:

```bash
chmod +x packaging/linux/build-appimage.sh
VERSION=dev packaging/linux/build-appimage.sh
```

This produces:

```text
dist/i2tor-dev-linux-x86_64.AppImage
```

When a user runs that AppImage, the launcher still keeps Tor Browser, I2P, Java, logs, PAC, profile, and state under `~/.local/share/i2tor/`.

For release builds, the workflow can also publish:

- `SHA256SUMS`
- `i2tor-<version>-linux-x86_64.AppImage.asc`
- `SHA256SUMS.asc`

when release signing secrets are configured in GitHub Actions.

## Troubleshooting

- `failed to verify Tor Browser signed checksum`: confirm the Tor Browser checksum signature and checksum files were fetched successfully and that `gpg` is available.
- `failed to verify I2P signature`: confirm the detached `.sig` asset was fetched successfully and that `gpg` is available.
- `failed to verify Java runtime signature`: confirm the Adoptium signature asset was fetched successfully and that `gpg` is available.
- `failed to extract archive safely`: inspect the archive structure; extraction rejects absolute paths and `..` traversal.
- `I2P HTTP proxy did not become ready on 127.0.0.1:4444 within timeout`: confirm the router started correctly and check the managed I2P logs under the runtime directory.
- `failed to write PAC file`: verify `runtime/pac/` is writable.
- `failed to write dedicated profile prefs`: verify `runtime/profile/` is writable.
- `failed to launch Tor Browser executable`: run `i2tor status` and `i2tor doctor`, then inspect `logs/current.log`.

## Known limitations

- Tor Browser artifact resolution is version-pinned in code and should be reviewed before production rollout.
- Windows and macOS managed install paths are isolated, but Linux is the only path exercised by the included integration tests.
- Managed I2P install currently depends on upstream Java I2P installer behavior and a managed or existing Java 17+ runtime.
- Signature verification depends on a working `gpg` binary at runtime.
- Reuse of existing installs depends on heuristic path detection and explicit opt-in via config.

## Security notes

- `.i2p` never falls back to Tor or direct.
- The launcher writes to its own dedicated Tor Browser profile only.
- The launcher tracks I2P process ownership in the manifest and does not intentionally kill unrelated I2P instances.
- A managed Java runtime is installed automatically if Java 17+ is not already available.
- Managed downloads are verified by checksum and signature before execution.

## Packaging notes

Linux:
- AppImage is the primary packaging target.
- The AppImage desktop entry launches the native `i2tor` GUI.
- The packaged launcher still installs and manages runtime dependencies in the per-user data directory.
- Release assets use a stable Linux AppImage name: `i2tor-<version>-linux-x86_64.AppImage`.
- Release signing can publish detached armored signatures for the AppImage and `SHA256SUMS`.
- A future `i2tor-bin` AUR package should consume that AppImage release artifact.

Windows:
- Package as a per-user install under `%LocalAppData%\\i2tor\\`.
- Avoid registry changes except those required by the installer framework itself.

macOS:
- Package as a signed app bundle that wraps the CLI launcher.
- Keep app data under `~/Library/Application Support/i2tor/` and avoid global proxy settings.
