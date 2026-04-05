# Architecture

## Component diagram

```text
cmd/i2tor
  |
  +-- internal/apppaths     per-user directory resolution
  +-- internal/config       launcher config load/save
  +-- internal/state        manifest + single-instance lock
  +-- internal/logging      console + rotating file logs
  +-- internal/detect       optional read-only reuse of existing installs
  +-- internal/downloader   fetch, checksum, and safe extraction
  +-- internal/verifier     vendored release keys + detached signature verification
  +-- internal/install      managed install orchestration
  +-- internal/pac          PAC generation
  +-- internal/profile      dedicated profile prefs
  +-- internal/runtime      process start/stop, readiness, executable resolution
  +-- internal/gui          browser-based fallback ui shell over launcher actions
  +-- internal/ui           CLI status formatting
  +-- internal/util         file URI and platform helpers
  +-- packaging/linux       AppImage wrapper, desktop entry, icon integration
```

## Startup sequence

```text
Acquire lock
Initialize logging
Resolve app paths
Load config + manifest
Ensure Tor Browser
Ensure Java runtime
Ensure I2P
Start I2P
Wait for 127.0.0.1:4444
Start bundled Tor from Tor Browser
Wait for 127.0.0.1:9150
Write proxy.pac
Write profile/user.js
Launch browser binary directly with dedicated profile
Wait for browser exit
Stop owned Tor
Stop owned I2P
Persist manifest
Release lock
```

## Install flow

Dependency handling follows this order for Tor Browser and I2P:

1. reuse an existing managed install if valid
2. if allowed by config, detect and reuse an existing user install in read-only mode
3. otherwise download, verify, extract, and atomically place a managed install

Managed install safety properties:

- downloads land in `downloads/`
- signature verification and SHA-256 verification must pass before extraction or execution
- extraction rejects absolute paths and `..` traversal
- installs extract into a temp directory and move into `runtime/` atomically

## Process ownership rules

- I2P ownership is recorded in `state/manifest.json`
- Tor ownership is runtime-only and tied to the current launcher session
- stale PID records are reconciled on startup
- only launcher-owned I2P processes should be terminated on exit
- reused existing installs are read-only and do not imply process ownership

## Manifest and state

The manifest stores:

- app version and normalized OS/arch
- install source and version metadata
- artifact URL and local artifact path
- checksum and signature verification results
- install timestamps
- last successful launch timestamp
- launcher-owned I2P PID and arguments
- managed Java runtime install metadata
- PAC/profile output locations

The lock file in `state/launcher.lock` prevents multiple concurrent `run` instances from mutating the same runtime directory.

## Future extension points

- Evolve the current Linux-first native window into a more polished cross-platform desktop shell without changing runtime or install packages.
- Add richer process reconciliation to detect PID reuse and stronger platform-specific process identity validation.
- Add native desktop UI packaging later if the AppImage-plus-browser-shell approach becomes too limiting.
