package install

import (
	"os"
	"path/filepath"
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

func TestNormalizeManagedI2PPortableConfigRewritesAllConfigDirAssignments(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	javaBinDir := filepath.Join(tmpDir, "java", "bin")
	if err := os.MkdirAll(javaBinDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	javaPath := filepath.Join(javaBinDir, "java")
	if err := os.WriteFile(javaPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(java) error = %v", err)
	}

	installDir := filepath.Join(tmpDir, "runtime", "i2p")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(installDir) error = %v", err)
	}
	for _, name := range []string{"wrapper.config", "runplain.sh", "eepget"} {
		content := "I2P=\"/tmp/stage\"\n"
		if name == "wrapper.config" {
			content = "wrapper.java.classpath.1=/tmp/stage/lib/*.jar\nwrapper.java.library.path.1=/tmp/stage\nwrapper.java.library.path.2=/tmp/stage/lib\n"
		}
		if err := os.WriteFile(filepath.Join(installDir, name), []byte(content), 0o755); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}
	i2prouter := `I2P="/tmp/stage"
if [ "` + "`uname -s`" + `" = "Darwin" ]; then
I2P_CONFIG_DIR="/tmp/stage"
else
    I2P_CONFIG_DIR="/home/seth/.i2p"
fi
PIDDIR="/tmp/stage"
LOGDIR="/tmp/stage"
I2PTEMP="/tmp/stage"
`
	if err := os.WriteFile(filepath.Join(installDir, "i2prouter"), []byte(i2prouter), 0o755); err != nil {
		t.Fatalf("WriteFile(i2prouter) error = %v", err)
	}

	java := InstalledApp{ExecutablePath: javaPath}
	if err := NormalizeManagedI2PPortableConfig(installDir, java); err != nil {
		t.Fatalf("NormalizeManagedI2PPortableConfig() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(installDir, "i2prouter"))
	if err != nil {
		t.Fatalf("ReadFile(i2prouter) error = %v", err)
	}
	content := string(data)
	if strings.Contains(content, "/home/seth/.i2p") {
		t.Fatalf("i2prouter still contains host config dir fallback: %q", content)
	}
	if got := strings.Count(content, `I2P_CONFIG_DIR="`+installDir+`"`); got < 2 {
		t.Fatalf("I2P_CONFIG_DIR replacements = %d, want at least 2 in %q", got, content)
	}

	wrapperData, err := os.ReadFile(filepath.Join(installDir, "wrapper.config"))
	if err != nil {
		t.Fatalf("ReadFile(wrapper.config) error = %v", err)
	}
	wrapperContent := string(wrapperData)
	for _, want := range []string{
		"wrapper.java.classpath.1=" + filepath.Join(installDir, "lib", "*.jar"),
		"wrapper.java.library.path.1=" + installDir,
		"wrapper.java.library.path.2=" + filepath.Join(installDir, "lib"),
	} {
		if !strings.Contains(wrapperContent, want) {
			t.Fatalf("wrapper.config missing %q in %q", want, wrapperContent)
		}
	}
}
