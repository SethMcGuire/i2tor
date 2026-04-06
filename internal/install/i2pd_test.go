package install

import (
	"strings"
	"testing"
)

func TestPickI2PAssetLinux(t *testing.T) {
	t.Parallel()

	release := i2pRelease{
		TagName: "i2p-2.11.0",
		Assets: []i2pReleaseAsset{
			{Name: "i2pinstall_2.11.0.jar", BrowserDownloadURL: "https://example.invalid/i2pinstall_2.11.0.jar", Digest: "sha256:abc"},
			{Name: "i2pinstall_2.11.0_windows.exe", BrowserDownloadURL: "https://example.invalid/i2pinstall_2.11.0_windows.exe", Digest: "sha256:def"},
		},
	}

	asset, err := pickI2PAsset(release, "linux")
	if err != nil {
		t.Fatalf("pickI2PAsset() error = %v", err)
	}
	if asset.Name != "i2pinstall_2.11.0.jar" {
		t.Fatalf("asset.Name = %q", asset.Name)
	}
}

func TestPickI2PAssetNoMatch(t *testing.T) {
	t.Parallel()

	_, err := pickI2PAsset(i2pRelease{TagName: "i2p-2.11.0"}, "linux")
	if err == nil {
		t.Fatalf("pickI2PAsset() error = nil, want no-match error")
	}
}

func TestRewriteInstallPathReference(t *testing.T) {
	t.Parallel()

	finalDir := "/home/seth/.local/share/i2tor/runtime/i2p"
	inputs := []string{
		`wrapper.java.classpath.1=/home/seth/.local/share/i2tor/runtime/i2p-install-872387859/lib/*.jar`,
		`wrapper.java.additional.2=-Di2p.dir.base="/home/seth/.local/share/i2tor/runtime/i2p-install-872387859"`,
		`I2P="/home/seth/.local/share/i2tor/runtime/i2p-install-872387859"`,
		`I2P_CONFIG_DIR="/home/seth/.local/share/i2tor/runtime/i2p-install-872387859"`,
	}

	for _, input := range inputs {
		got := rewriteInstallPathReference(input, finalDir)
		if strings.Contains(got, "i2p-install-872387859") {
			t.Fatalf("rewriteInstallPathReference() left stale temp path in %q", got)
		}
		if !strings.Contains(got, finalDir) {
			t.Fatalf("rewriteInstallPathReference() = %q, want final dir %q", got, finalDir)
		}
	}
}
