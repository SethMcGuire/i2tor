package runtime

import (
	"context"
	"testing"
	"time"

	"i2tor/internal/state"
)

func TestReconcileManagedProcessRecordClearsStalePID(t *testing.T) {
	t.Parallel()

	record := state.ManagedProcessRecord{PID: 999999, Owns: true}
	got := ReconcileManagedProcessRecord(record)
	if got.PID != 0 || got.Owns {
		t.Fatalf("record not cleared: %+v", got)
	}
}

func TestWaitForI2PReadyTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := WaitForI2PReady(ctx, "127.0.0.1:65530", 300*time.Millisecond)
	if err == nil {
		t.Fatalf("WaitForI2PReady() error = nil, want timeout")
	}
}
