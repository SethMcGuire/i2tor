package main

import "testing"

func TestBlankDefault(t *testing.T) {
	t.Parallel()

	if got := blankDefault("", "fallback"); got != "fallback" {
		t.Fatalf("blankDefault(empty) = %q, want fallback", got)
	}
	if got := blankDefault("value", "fallback"); got != "value" {
		t.Fatalf("blankDefault(value) = %q, want value", got)
	}
}
