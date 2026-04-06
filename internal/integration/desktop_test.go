package integration

import "testing"

func TestShellQuote(t *testing.T) {
	t.Parallel()

	got := shellQuote("/tmp/it's here/AppImage")
	want := `'/tmp/it'\''s here/AppImage'`
	if got != want {
		t.Fatalf("shellQuote() = %q, want %q", got, want)
	}
}
