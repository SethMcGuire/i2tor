package apppaths

import (
	"context"
	"path/filepath"
	"testing"
)

func TestResolveConfiguredRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths, err := Resolve(context.Background(), root)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if paths.Root != root {
		t.Fatalf("Root = %q, want %q", paths.Root, root)
	}
	if paths.PACFile != filepath.Join(root, "runtime", "pac", "proxy.pac") {
		t.Fatalf("PACFile = %q", paths.PACFile)
	}
}
