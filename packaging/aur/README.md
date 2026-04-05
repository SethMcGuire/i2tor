# AUR Packaging

The recommended Arch package is `i2tor-bin`, built from the GitHub release AppImage rather than from source.

## Release artifact contract

The AUR package expects Linux release assets with this stable naming pattern:

- `i2tor-<version>-linux-x86_64.AppImage`
- `SHA256SUMS`
- `i2tor-<version>-linux-x86_64.AppImage.asc`
- `SHA256SUMS.asc`

Example:

- `i2tor-0.1.0-linux-x86_64.AppImage`

## Update process

For each new release:

1. Publish a GitHub release tag like `v0.1.0`
2. Confirm the release includes:
   - the AppImage
   - `SHA256SUMS`
   - the detached GPG signatures for both
3. Update [PKGBUILD](/home/seth/Documents/i2tor/packaging/aur/i2tor-bin/PKGBUILD):
   - `pkgver`
   - `sha256sums`
   - `url` / `source` owner path if needed
4. Generate `.SRCINFO`

```bash
cd packaging/aur/i2tor-bin
makepkg --printsrcinfo > .SRCINFO
```

5. Push the updated package metadata to the AUR `i2tor-bin` repo

## Release signing

The GitHub release workflow can sign release artifacts when these repository secrets are configured:

- `RELEASE_GPG_PRIVATE_KEY`
  ASCII-armored private key used for release signing
- `RELEASE_GPG_KEY_ID`
  key ID or fingerprint to pass to `gpg --local-user`
- `RELEASE_GPG_PASSPHRASE`
  passphrase for the signing key

When those secrets are present, the workflow publishes detached armored signatures alongside the AppImage and checksum files.

## Why `-bin`

`i2tor-bin` is the better first Arch package because:

- release artifacts already exist
- the native GUI path depends on desktop libraries that are easier to validate in the packaged AppImage flow
- users get the same tested artifact as GitHub release users

Source-based Arch packaging can come later if needed.
