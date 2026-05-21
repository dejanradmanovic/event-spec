package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServeCmd_MissingDB(t *testing.T) {
	cmd := newServeCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --db is not provided")
	}
	if !strings.Contains(err.Error(), "--db") {
		t.Errorf("error should mention '--db', got: %v", err)
	}
}

func TestServeCmd_StartsAndShutdown(t *testing.T) {
	// Find a free port.
	lc := &net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	// Use in-memory SQLite to avoid file locking on Windows during test cleanup.
	dsn := ":memory:"

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "event-spec.yaml"), []byte(`version: 1
workspace: test-workspace
registry:
  mode: server
`), 0o644); err != nil {
		t.Fatal(err)
	}
	withWorkDir(t, root)

	go func() {
		cmd := newServeCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{
			"--port", fmt.Sprintf("%d", port),
			"--db", dsn,
		})
		_ = cmd.Execute()
	}()

	// Wait for the server to be ready.
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	dialer := &net.Dialer{}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := dialer.DialContext(context.Background(), "tcp", addr); err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Unauthenticated GET /v1/events must return 401 — this proves the server
	// is running and the auth middleware is enforced correctly.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s/v1/events", addr), http.NoBody)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/events: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated request, got %d", resp.StatusCode)
	}
}
