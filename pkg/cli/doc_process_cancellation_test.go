//go:build !windows

package cli_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestExampleCancellationKillsDescendants(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "survived")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", `(sleep 1; echo survived > "$1") & echo ready; wait`, "test", marker)
	configureExampleProcess(cmd)
	// Cancel only after the child has started, without depending on startup speed.
	ready, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var buf [6]byte
	if _, err := ready.Read(buf[:]); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	cancel()
	if err := cmd.Wait(); err == nil {
		t.Fatal("cancelled command succeeded")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("cancellation waited for descendant pipes")
	}
	time.Sleep(1100 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("descendant survived cancellation: %v", err)
	}
}
