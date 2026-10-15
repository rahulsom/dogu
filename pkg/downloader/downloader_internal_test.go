package downloader

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestTruncateURL(t *testing.T) {
	short := "https://example.com/file.tar.gz"
	if got := truncateURL(short, 50); got != short {
		t.Errorf("expected %q, got %q", short, got)
	}

	long := "https://api.foojay.io/disco/v3.0/ids/5685a7dc402ed37f569c0f043e99c506/redirect"
	truncated := truncateURL(long, 40)
	if len(truncated) > 40 {
		t.Errorf("expected length <= 40, got length %d (%q)", len(truncated), truncated)
	}
	if !strings.Contains(truncated, "...") {
		t.Errorf("expected truncated string to contain '...', got %q", truncated)
	}
}

func TestProgressManager_TTYRendering(t *testing.T) {
	pm := &progressManager{
		isTTY: true,
	}

	task1 := &downloadTask{
		id:      1,
		name:    "https://example.com/jdk17.tar.gz",
		total:   1000,
		current: 500,
	}
	task2 := &downloadTask{
		id:      2,
		name:    "https://example.com/jdk21.tar.gz",
		total:   2000,
		current: 500,
	}

	pm.tasks = append(pm.tasks, task1, task2)

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	os.Stderr = w

	pm.mu.Lock()
	pm.renderLocked()
	pm.mu.Unlock()

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "50%") {
		t.Errorf("expected 50%% progress in output, got: %q", out)
	}
	if !strings.Contains(out, "jdk17") || !strings.Contains(out, "jdk21") {
		t.Errorf("expected task names in output, got: %q", out)
	}
	if !strings.Contains(out, "█") {
		t.Errorf("expected progress bar blocks in output, got: %q", out)
	}
}

func TestProgressManager_RedirectTTYRendering(t *testing.T) {
	pm := &progressManager{
		isTTY: true,
	}

	task := &downloadTask{
		id:          1,
		name:        "https://api.foojay.io/disco/v3.0/ids/27f8a3bf93692e/redirect",
		redirectURL: "https://cdn.foojay.io/downloads/jdk17.tar.gz",
		total:       2000,
		current:     820,
	}

	pm.tasks = append(pm.tasks, task)

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	os.Stderr = w

	pm.mu.Lock()
	pm.renderLocked()
	pm.mu.Unlock()

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines of output for redirected download, got %d lines: %q", len(lines), out)
	}

	if !strings.Contains(lines[0], "27f8a3bf93692e/redirect") {
		t.Errorf("expected line 1 to contain original URL, got: %q", lines[0])
	}

	if !strings.Contains(lines[1], "jdk17.tar.gz") {
		t.Errorf("expected line 2 to contain redirected URL, got: %q", lines[1])
	}

	if !strings.Contains(lines[1], "41%") {
		t.Errorf("expected line 2 to contain progress percentage 41%%, got: %q", lines[1])
	}
}

func TestProgressManager_FinishTask(t *testing.T) {
	pm := &progressManager{
		isTTY: true,
	}

	task := &downloadTask{
		id:      1,
		name:    "https://example.com/test.zip",
		total:   100,
		current: 100,
	}
	pm.tasks = []*downloadTask{task}

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	os.Stderr = w

	pm.finishTask(task)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "Downloaded https://example.com/test.zip") {
		t.Errorf("expected finished message in output, got: %q", out)
	}
	if len(pm.tasks) != 0 {
		t.Errorf("expected tasks to be empty after finish, got %d tasks", len(pm.tasks))
	}
}
