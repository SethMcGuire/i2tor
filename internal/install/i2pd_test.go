package install

import "testing"

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
