package pac

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const proxyPAC = `function FindProxyForURL(url, host) {
  if (shExpMatch(host, "*.i2p") || host === "i2p") {
    return "PROXY 127.0.0.1:4444";
  }
  return "SOCKS5 127.0.0.1:9150";
}
`

func Content() string {
	return proxyPAC
}

func ResolveProxyForHost(host string) string {
	if host == "i2p" || strings.HasSuffix(host, ".i2p") {
		return "PROXY 127.0.0.1:4444"
	}
	return "SOCKS5 127.0.0.1:9150"
}

func WritePACFile(ctx context.Context, pacPath string) error {
	_ = ctx
	if err := os.MkdirAll(filepath.Dir(pacPath), 0o755); err != nil {
		return fmt.Errorf("create PAC directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(pacPath), "proxy-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp PAC file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(proxyPAC); err != nil {
		tmp.Close()
		return fmt.Errorf("write PAC file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close PAC file: %w", err)
	}
	if err := os.Rename(tmp.Name(), pacPath); err != nil {
		return fmt.Errorf("replace PAC file: %w", err)
	}
	return nil
}
