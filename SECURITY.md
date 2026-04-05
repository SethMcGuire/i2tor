# Security Policy

## Threat model

`i2tor` is a local launcher that downloads or reuses Tor Browser, Java I2P, and a Java runtime when needed, prepares a dedicated browser profile, and starts local child processes. Its primary threats are:

- tampered downloads
- unsafe archive extraction
- accidental mutation of a user's normal browser profiles
- accidental routing fallback for `.i2p`
- accidental termination of unrelated user processes

## Trust assumptions

- The operating system user account running `i2tor` is trusted to read and write its own per-user data directory.
- Official artifact distribution channels for Tor Browser, Java I2P, and the managed Java runtime are assumed to be the source of truth for managed installs.
- Existing-install reuse is treated as read-only and lower trust than managed installs because the launcher did not provision those artifacts itself.

## Checksum verification limitations

Managed downloads are SHA-256 verified before extraction or execution. This is still necessary even with signatures:

- a compromised checksum distribution channel can still subvert verification
- checksum verification complements signature validation by binding the exact artifact bytes

## Signature verification status and limitations

`i2tor` now verifies:

- Tor Browser signed checksum files with the Tor Browser Developers signing key
- Java I2P detached signatures with the upstream I2P release signing key
- Adoptium detached signatures with the Adoptium signing key

Current limitations:

- the launcher relies on `gpg` being present on the host
- vendored public keys in the repository must be rotated deliberately if upstream changes release keys
- this does not yet provide transparency logging, key pin rotation policy, or TUF-style delegated metadata

## No-fallback rule for `.i2p`

The generated PAC file routes only `.i2p` hostnames to `127.0.0.1:4444` and routes everything else to Tor Browser SOCKS on `127.0.0.1:9150`. If I2P readiness fails, the browser is not launched. There is no intentional fallback from `.i2p` to Tor or direct.

## Profile isolation rule

`i2tor` writes PAC and proxy preferences only into its dedicated profile under `runtime/profile/`. It does not modify default Firefox profiles, default Tor Browser profiles, or system-wide proxy settings.

## Process ownership rule

The launcher records the PID and launch metadata for the I2P process it starts. On shutdown, it only attempts to terminate the I2P process that it owns. If an existing or already-running I2P instance was not started by the launcher, it should not be terminated by `i2tor`.
