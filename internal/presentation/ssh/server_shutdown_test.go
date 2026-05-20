package ssh

import (
	"context"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestStartShutdownClosesHalfOpenConnections(t *testing.T) {
	t.Parallel()

	addr := freeTCPAddr(t)
	hostKeyPath := filepath.Join(t.TempDir(), "ssh_host_ed25519_key")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := New(addr, hostKeyPath, logger, nil, nil, nil, nil, nil, "", nil, nil, nil)
	done := make(chan error, 1)
	go func() {
		done <- server.Start(ctx)
	}()

	var conn net.Conn
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn = c
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if conn == nil {
		t.Fatal("failed to connect test client to ssh server")
	}
	defer conn.Close()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("server.Start returned error on shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server shutdown timed out with half-open connection")
	}
}

func freeTCPAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve tcp port: %v", err)
	}
	defer ln.Close()

	return ln.Addr().String()
}
