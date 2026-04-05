package runtime

import (
	"context"
	"fmt"
	"net"
	"time"
)

func WaitForI2PReady(ctx context.Context, addr string, timeout time.Duration) error {
	return waitForPortReady(ctx, addr, timeout, "I2P HTTP proxy")
}

func WaitForTorReady(ctx context.Context, addr string, timeout time.Duration) error {
	return waitForPortReady(ctx, addr, timeout, "Tor SOCKS proxy")
}

func waitForPortReady(ctx context.Context, addr string, timeout time.Duration, name string) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("%s did not become ready on %s within %s", name, addr, timeout)
		case <-ticker.C:
		}
	}
}
