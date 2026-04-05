package e2e

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBinaryRunWithManagedStubs(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only e2e test")
	}
	if os.Getenv("I2TOR_E2E") != "1" {
		t.Skip("set I2TOR_E2E=1 to run end-to-end launcher test")
	}

	root := t.TempDir()
	binPath := filepath.Join(root, "i2tor")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/i2tor")
	cmd.Dir = filepath.Clean(filepath.Join(".."))
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/i2tor-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build launcher: %v: %s", err, output)
	}

	dataDir := filepath.Join(root, "data")
	writeFile(t, filepath.Join(dataDir, "state", "config.json"), `{"reuse_existing_tor_browser":false,"reuse_existing_i2p":false,"auto_check_updates":false,"data_dir":"`+dataDir+`","log_level":"info"}`+"\n", 0o644)
	writeExecutable(t, filepath.Join(dataDir, "runtime", "java", "bin", "java"), "#!/bin/sh\ntrap 'exit 0' TERM INT\nwhile true; do sleep 1; done\n")
	writeExecutable(t, filepath.Join(dataDir, "runtime", "i2p", "i2prouter"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(dataDir, "runtime", "i2p", "lib", "dummy.jar"), "")
	writeFile(t, filepath.Join(dataDir, "runtime", "i2p", "clients.config"), "clientApp.3.startOnLoad=true\nclientApp.4.startOnLoad=true\n", 0o644)
	writeExecutable(t, filepath.Join(dataDir, "runtime", "tor-browser", "Browser", "firefox"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(dataDir, "runtime", "tor-browser", "Browser", "TorBrowser", "Tor", "tor"), "#!/bin/sh\ntrap 'exit 0' TERM INT\nwhile true; do sleep 1; done\n")
	writeFile(t, filepath.Join(dataDir, "runtime", "tor-browser", "Browser", "TorBrowser", "Data", "Tor", "torrc-defaults"), "", 0o644)
	writeFile(t, filepath.Join(dataDir, "runtime", "tor-browser", "Browser", "TorBrowser", "Data", "Tor", "geoip"), "", 0o644)
	writeFile(t, filepath.Join(dataDir, "runtime", "tor-browser", "Browser", "TorBrowser", "Data", "Tor", "geoip6"), "", 0o644)

	i2pLn, err := net.Listen("tcp", "127.0.0.1:4444")
	if err != nil {
		t.Skipf("port 4444 unavailable: %v", err)
	}
	defer i2pLn.Close()
	torLn, err := net.Listen("tcp", "127.0.0.1:9150")
	if err != nil {
		t.Skipf("port 9150 unavailable: %v", err)
	}
	defer torLn.Close()

	run := exec.Command(binPath, "run")
	run.Dir = filepath.Clean(filepath.Join(".."))
	run.Env = append(os.Environ(), "HOME="+root)
	output, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("launcher run: %v: %s", err, output)
	}

	pacData, err := os.ReadFile(filepath.Join(dataDir, "runtime", "pac", "proxy.pac"))
	if err != nil {
		t.Fatalf("read pac file: %v", err)
	}
	if !strings.Contains(string(pacData), "PROXY 127.0.0.1:4444") {
		t.Fatalf("pac file missing I2P rule")
	}
	userJS, err := os.ReadFile(filepath.Join(dataDir, "runtime", "profile", "user.js"))
	if err != nil {
		t.Fatalf("read user.js: %v", err)
	}
	for _, needle := range []string{"network.proxy.autoconfig_url", "extensions.torbutton.use_nontor_proxy", "extensions.torlauncher.start_tor"} {
		if !strings.Contains(string(userJS), needle) {
			t.Fatalf("user.js missing %q", needle)
		}
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	writeFile(t, path, content, 0o755)
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
