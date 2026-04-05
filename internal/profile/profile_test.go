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
	if err := WriteDedicatedProfilePrefs(context.Background(), dir, pacPath); err != nil {
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
	required := []string{
		`user_pref("extensions.torbutton.use_nontor_proxy", true);`,
		`user_pref("extensions.torbutton.startup", false);`,
		`user_pref("extensions.torbutton.test_enabled", false);`,
		`user_pref("extensions.torlauncher.prompt_at_startup", false);`,
		`user_pref("extensions.torlauncher.start_tor", false);`,
		`user_pref("browser.fixup.domainsuffixwhitelist.i2p", true);`,
		`user_pref("dom.security.https_only_mode", false);`,
		`user_pref("dom.security.https_first", false);`,
	}
	for _, needle := range required {
		if !strings.Contains(content, needle) {
			t.Fatalf("missing required Tor Browser PAC compatibility pref %q in %s", needle, content)
		}
	}
}
