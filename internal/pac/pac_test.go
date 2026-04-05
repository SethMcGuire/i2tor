package pac

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWritePACFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "proxy.pac")
	if err := WritePACFile(context.Background(), path); err != nil {
		t.Fatalf("WritePACFile() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != Content() {
		t.Fatalf("PAC content mismatch:\n%s", data)
	}
}

func TestResolveProxyForHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		host string
		want string
	}{
		{host: "i2p", want: "PROXY 127.0.0.1:4444"},
		{host: "example.i2p", want: "PROXY 127.0.0.1:4444"},
		{host: "example.com", want: "SOCKS5 127.0.0.1:9150"},
		{host: "localhost", want: "SOCKS5 127.0.0.1:9150"},
	}

	for _, tt := range tests {
		if got := ResolveProxyForHost(tt.host); got != tt.want {
			t.Fatalf("ResolveProxyForHost(%q) = %q, want %q", tt.host, got, tt.want)
		}
	}
}
