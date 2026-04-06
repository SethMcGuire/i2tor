package profile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteDedicatedProfilePrefs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pacPath := filepath.Join(dir, "proxy.pac")
	if err := os.WriteFile(pacPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	startPage, err := WriteStartPage(context.Background(), filepath.Join(dir, "start-page"), Options{})
	if err != nil {
		t.Fatalf("WriteStartPage() error = %v", err)
	}
	if err := WriteDedicatedProfilePrefs(context.Background(), dir, pacPath, Options{HomePagePath: startPage}); err != nil {
		t.Fatalf("WriteDedicatedProfilePrefs() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "user.js"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `user_pref("network.proxy.type", 2);`) {
		t.Fatalf("missing proxy type pref: %s", content)
	}
	if !strings.Contains(content, `user_pref("network.proxy.autoconfig_url", "file://`) {
		t.Fatalf("missing PAC URI pref: %s", content)
	}
	if !strings.Contains(content, `user_pref("browser.startup.homepage", "file://`) {
		t.Fatalf("missing startup homepage pref: %s", content)
	}
	required := []string{
		`user_pref("extensions.torbutton.use_nontor_proxy", true);`,
		`user_pref("extensions.torbutton.startup", false);`,
		`user_pref("extensions.torbutton.test_enabled", false);`,
		`user_pref("extensions.torlauncher.prompt_at_startup", false);`,
		`user_pref("extensions.torlauncher.start_tor", false);`,
		`user_pref("i2tor.profile.display_name", "i2tor Dedicated Profile");`,
		`user_pref("browser.fixup.domainsuffixwhitelist.i2p", true);`,
		`user_pref("dom.security.https_only_mode", false);`,
		`user_pref("dom.security.https_first", false);`,
	}
	for _, needle := range required {
		if !strings.Contains(content, needle) {
			t.Fatalf("missing required Tor Browser PAC compatibility pref %q in %s", needle, content)
		}
	}
	for _, forbidden := range []string{
		`user_pref("network.proxy.allow_hijacking_localhost", true);`,
		`user_pref("network.proxy.no_proxies_on", "");`,
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("unexpected localhost opt-in pref %q in %s", forbidden, content)
		}
	}
}

func TestWriteDedicatedProfilePrefsWithLocalhostAccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pacPath := filepath.Join(dir, "proxy.pac")
	if err := os.WriteFile(pacPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	startPage, err := WriteStartPage(context.Background(), filepath.Join(dir, "start-page"), Options{ProfileDisplayName: "My i2tor Profile"})
	if err != nil {
		t.Fatalf("WriteStartPage() error = %v", err)
	}
	if err := WriteDedicatedProfilePrefs(context.Background(), dir, pacPath, Options{AllowLocalhostAccess: true, HomePagePath: startPage, ProfileDisplayName: "My i2tor Profile"}); err != nil {
		t.Fatalf("WriteDedicatedProfilePrefs() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "user.js"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	for _, needle := range []string{
		`user_pref("network.proxy.allow_hijacking_localhost", true);`,
		`user_pref("network.proxy.no_proxies_on", "");`,
		`user_pref("i2tor.profile.display_name", "My i2tor Profile");`,
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("missing localhost opt-in pref %q in %s", needle, content)
		}
	}
}

func TestWriteStartPage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path, err := WriteStartPage(context.Background(), dir, Options{ProfileDisplayName: "i2tor Session"})
	if err != nil {
		t.Fatalf("WriteStartPage() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	for _, needle := range []string{
		`Tor Browser, routed by i2tor`,
		`i2tor Session`,
		`127.0.0.1:4444`,
		`127.0.0.1:9150`,
		`src="logo.png"`,
		`http://tortaxi2dev6xjwbaydqzla77rrnth7yn2oqzjfmiuwn5h6vsk2a4syd.onion/`,
		`http://notbob.i2p`,
		`Getting Started`,
		`not affiliated with, endorsed by, or sponsored by the Tor Project or the I2P Project`,
		`Donations`,
		`Tor and I2P make this possible. If you donate, please consider supporting those projects first.`,
		`https://donate.torproject.org/`,
		`https://i2p.net/en/financial-support/`,
		`buy me a beer`,
		`YOUR_XMR_ADDRESS_HERE`,
		`YOUR_BTC_ADDRESS_HERE`,
		`YOUR_ETH_ADDRESS_HERE`,
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("missing start page content %q in %s", needle, content)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "logo.png")); err != nil {
		t.Fatalf("expected copied start page logo: %v", err)
	}
}
