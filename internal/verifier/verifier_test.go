package verifier

import (
	"strings"
	"testing"
)

func TestKeyByName(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"torbrowser", "i2p", "adoptium"} {
		key, err := keyByName(name)
		if err != nil {
			t.Fatalf("keyByName(%q) error = %v", name, err)
		}
		if !strings.Contains(key, "BEGIN PGP PUBLIC KEY BLOCK") {
			t.Fatalf("keyByName(%q) did not return an armored key", name)
		}
	}
}

func TestKeyByNameUnknown(t *testing.T) {
	t.Parallel()

	if _, err := keyByName("missing"); err == nil {
		t.Fatalf("keyByName() error = nil, want unknown key error")
	}
}
