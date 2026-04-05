package profile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"i2tor/internal/util"
)

func prefsContent(pacFileURI string) string {
	return fmt.Sprintf(
		"user_pref(\"network.proxy.type\", 2);\n"+
			"user_pref(\"network.proxy.autoconfig_url\", %q);\n"+
			"user_pref(\"extensions.torbutton.use_nontor_proxy\", true);\n"+
			"user_pref(\"extensions.torbutton.startup\", false);\n"+
			"user_pref(\"extensions.torbutton.test_enabled\", false);\n"+
			"user_pref(\"extensions.torlauncher.prompt_at_startup\", false);\n"+
			"user_pref(\"extensions.torlauncher.start_tor\", false);\n"+
			"user_pref(\"browser.fixup.domainsuffixwhitelist.i2p\", true);\n"+
			"user_pref(\"dom.security.https_only_mode\", false);\n"+
			"user_pref(\"dom.security.https_only_mode_pbm\", false);\n"+
			"user_pref(\"dom.security.https_first\", false);\n"+
			"user_pref(\"dom.security.https_first_pbm\", false);\n",
		pacFileURI,
	)
}

func WriteDedicatedProfilePrefs(ctx context.Context, profileDir, pacFilePath string) error {
	_ = ctx
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return fmt.Errorf("create dedicated profile directory: %w", err)
	}
	pacURI, err := util.FileURI(pacFilePath)
	if err != nil {
		return fmt.Errorf("convert PAC path to file URI: %w", err)
	}
	target := filepath.Join(profileDir, "user.js")
	tmp, err := os.CreateTemp(profileDir, "user-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp profile prefs: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(prefsContent(pacURI)); err != nil {
		tmp.Close()
		return fmt.Errorf("write dedicated profile prefs: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close dedicated profile prefs: %w", err)
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return fmt.Errorf("replace dedicated profile prefs: %w", err)
	}
	return nil
}
