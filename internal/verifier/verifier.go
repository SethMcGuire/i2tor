package verifier

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed keys/torbrowser.asc
var torBrowserKey string

//go:embed keys/i2p.asc
var i2pKey string

//go:embed keys/adoptium.asc
var adoptiumKey string

func VerifyDetachedSignature(ctx context.Context, keyName, artifactPath, signaturePath string) error {
	keyData, err := keyByName(keyName)
	if err != nil {
		return err
	}
	home, err := os.MkdirTemp("", "i2tor-gpg-*")
	if err != nil {
		return fmt.Errorf("create temporary gpg home: %w", err)
	}
	defer os.RemoveAll(home)

	importPath := filepath.Join(home, "signing-key.asc")
	if err := os.WriteFile(importPath, []byte(keyData), 0o600); err != nil {
		return fmt.Errorf("write temporary signing key: %w", err)
	}
	if output, err := runGPG(ctx, home, "--batch", "--yes", "--import", importPath); err != nil {
		return fmt.Errorf("import signing key %q: %w: %s", keyName, err, output)
	}
	if output, err := runGPG(ctx, home, "--batch", "--verify", signaturePath, artifactPath); err != nil {
		return fmt.Errorf("verify detached signature for %q with key %q: %w: %s", artifactPath, keyName, err, output)
	}
	return nil
}

func keyByName(name string) (string, error) {
	switch name {
	case "torbrowser":
		return torBrowserKey, nil
	case "i2p":
		return i2pKey, nil
	case "adoptium":
		return adoptiumKey, nil
	default:
		return "", fmt.Errorf("unknown signature key %q", name)
	}
}

func runGPG(ctx context.Context, home string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gpg", append([]string{"--homedir", home}, args...)...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
