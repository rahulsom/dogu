package downloader

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ulikunitz/xz"
)

var stderrMu sync.Mutex

func logProgress(format string, a ...any) {
	stderrMu.Lock()
	defer stderrMu.Unlock()
	fmt.Fprintf(os.Stderr, format, a...)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func isStderrTerminal() bool {
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

type downloadTask struct {
	id         int
	name       string
	total      int64
	current    int64
	completed  bool
	lastPct    int
	lastReport int64
}

type progressManager struct {
	mu            sync.Mutex
	tasks         []*downloadTask
	nextID        int
	isTTY         bool
	linesRendered int
	ticker        *time.Ticker
	doneChan      chan struct{}
	spinnerIdx    int
}

var globalPM = newProgressManager()

func newProgressManager() *progressManager {
	return &progressManager{}
}

func (pm *progressManager) startTask(downloadURL string, total int64) *downloadTask {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.isTTY = isStderrTerminal()

	pm.nextID++
	task := &downloadTask{
		id:      pm.nextID,
		name:    downloadURL,
		total:   total,
		current: 0,
	}

	if !pm.isTTY {
		if total > 0 {
			logProgress("Downloading %s (%s)...\n", downloadURL, formatBytes(total))
		} else {
			logProgress("Downloading %s...\n", downloadURL)
		}
	} else {
		pm.tasks = append(pm.tasks, task)
		if pm.ticker == nil {
			pm.ticker = time.NewTicker(80 * time.Millisecond)
			doneChan := make(chan struct{})
			pm.doneChan = doneChan
			go pm.renderLoop(pm.ticker, doneChan)
		}
	}

	return task
}

func (pm *progressManager) updateTask(task *downloadTask, n int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	task.current += int64(n)

	if !pm.isTTY {
		if task.total > 0 {
			pct := int((task.current * 100) / task.total)
			if pct >= task.lastPct+25 && pct < 100 {
				task.lastPct = (pct / 25) * 25
				logProgress("Downloading %s: %d%% (%s / %s)\n", task.name, task.lastPct, formatBytes(task.current), formatBytes(task.total))
			}
		} else {
			if task.current-task.lastReport >= 5*1024*1024 {
				task.lastReport = task.current
				logProgress("Downloading %s: %s downloaded\n", task.name, formatBytes(task.current))
			}
		}
	}
}

func (pm *progressManager) finishTask(task *downloadTask) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if task.completed {
		return
	}
	task.completed = true

	if !pm.isTTY {
		if task.total > 0 {
			logProgress("Downloaded %s (100%% - %s)\n", task.name, formatBytes(task.total))
		} else {
			logProgress("Downloaded %s (%s)\n", task.name, formatBytes(task.current))
		}
		return
	}

	pm.clearLines()

	if task.total > 0 {
		fmt.Fprintf(os.Stderr, "Downloaded %s (100%% - %s)\n", task.name, formatBytes(task.total))
	} else {
		fmt.Fprintf(os.Stderr, "Downloaded %s (%s)\n", task.name, formatBytes(task.current))
	}

	newTasks := make([]*downloadTask, 0, len(pm.tasks))
	for _, t := range pm.tasks {
		if t.id != task.id {
			newTasks = append(newTasks, t)
		}
	}
	pm.tasks = newTasks

	if len(pm.tasks) == 0 {
		if pm.ticker != nil {
			pm.ticker.Stop()
			pm.ticker = nil
			if pm.doneChan != nil {
				close(pm.doneChan)
				pm.doneChan = nil
			}
		}
	} else {
		pm.renderLocked()
	}
}

func (pm *progressManager) renderLoop(ticker *time.Ticker, doneChan chan struct{}) {
	for {
		select {
		case <-ticker.C:
			pm.mu.Lock()
			pm.renderLocked()
			pm.mu.Unlock()
		case <-doneChan:
			return
		}
	}
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (pm *progressManager) clearLines() {
	if pm.linesRendered > 0 {
		fmt.Fprintf(os.Stderr, "\033[%dA\033[J", pm.linesRendered)
		pm.linesRendered = 0
	}
}

func (pm *progressManager) renderLocked() {
	if !pm.isTTY || len(pm.tasks) == 0 {
		return
	}

	pm.clearLines()

	pm.spinnerIdx = (pm.spinnerIdx + 1) % len(spinnerFrames)
	spinner := spinnerFrames[pm.spinnerIdx]

	for _, task := range pm.tasks {
		displayName := truncateURL(task.name, 50)
		var line string
		if task.total > 0 {
			pct := int((task.current * 100) / task.total)
			if pct > 100 {
				pct = 100
			}
			filled := (pct * 20) / 100
			bar := strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)
			line = fmt.Sprintf("%s Downloading %s %d%% [%s] (%s / %s)",
				spinner, displayName, pct, bar, formatBytes(task.current), formatBytes(task.total))
		} else {
			line = fmt.Sprintf("%s Downloading %s (%s downloaded)",
				spinner, displayName, formatBytes(task.current))
		}
		fmt.Fprintf(os.Stderr, "%s\033[K\n", line)
	}

	pm.linesRendered = len(pm.tasks)
}

func truncateURL(u string, maxLen int) string {
	if len(u) <= maxLen {
		return u
	}
	if maxLen < 10 {
		return u[:maxLen]
	}
	half := (maxLen - 3) / 2
	return u[:half] + "..." + u[len(u)-half:]
}

type progressReader struct {
	reader io.Reader
	task   *downloadTask
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		globalPM.updateTask(pr.task, n)
	}
	return n, err
}

func downloadFile(ctx context.Context, downloadURL string, cacheFile string) error {
	tmpCache := cacheFile + ".tmp"
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", downloadURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %s from %s", resp.Status, downloadURL)
	}

	out, err := os.Create(tmpCache)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	total := resp.ContentLength
	task := globalPM.startTask(downloadURL, total)

	pr := &progressReader{
		reader: resp.Body,
		task:   task,
	}

	_, err = io.Copy(out, pr)
	out.Close()
	if err != nil {
		os.Remove(tmpCache)
		globalPM.finishTask(task)
		return fmt.Errorf("failed to write download stream: %w", err)
	}

	if err := os.Rename(tmpCache, cacheFile); err != nil {
		globalPM.finishTask(task)
		return fmt.Errorf("failed to save cache file: %w", err)
	}

	globalPM.finishTask(task)
	return nil
}

// DownloadAndExtract downloads archive from downloadURL (or checks cache) and extracts it into targetDir.
func DownloadAndExtract(ctx context.Context, downloadURL string, cacheFile string, targetDir string) error {
	// If targetDir already exists and is non-empty, assume already installed
	if entries, err := os.ReadDir(targetDir); err == nil && len(entries) > 0 {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		return fmt.Errorf("failed to create cache dir: %w", err)
	}

	// Download to cacheFile if not present
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		if err := downloadFile(ctx, downloadURL, cacheFile); err != nil {
			return err
		}
	}

	// Extract cacheFile to targetDir
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target dir: %w", err)
	}

	lowerURL := strings.ToLower(downloadURL)
	lowerCache := strings.ToLower(cacheFile)

	if strings.HasSuffix(lowerCache, ".zip") || strings.HasSuffix(lowerURL, ".zip") {
		return ExtractZip(cacheFile, targetDir)
	} else if strings.HasSuffix(lowerCache, ".tar.gz") || strings.HasSuffix(lowerCache, ".tgz") || strings.HasSuffix(lowerURL, ".tar.gz") || strings.HasSuffix(lowerURL, ".tgz") {
		return ExtractTarGz(cacheFile, targetDir)
	} else if strings.HasSuffix(lowerCache, ".tar.xz") || strings.HasSuffix(lowerURL, ".tar.xz") {
		return ExtractTarXz(cacheFile, targetDir)
	} else {
		return ExtractTarGz(cacheFile, targetDir)
	}
}

// SaveSingleBinary saves a single binary file to targetDir/binName
func SaveSingleBinary(ctx context.Context, downloadURL string, cacheFile string, targetDir string, binName string) error {
	destPath := filepath.Join(targetDir, binName)
	if _, err := os.Stat(destPath); err == nil {
		return nil
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		return err
	}

	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		if err := downloadFile(ctx, downloadURL, cacheFile); err != nil {
			return err
		}
	}

	in, err := os.Open(cacheFile)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func sanitizeExtractPath(dest string, filePath string) (string, error) {
	target := filepath.Join(dest, filePath)
	cleanDest := filepath.Clean(dest)
	cleanTarget := filepath.Clean(target)

	if !strings.HasPrefix(cleanTarget, cleanDest+string(os.PathSeparator)) && cleanTarget != cleanDest {
		return "", fmt.Errorf("illegal file path in archive: %s", filePath)
	}
	return cleanTarget, nil
}

func ExtractTarGz(archive string, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	return extractTar(gzr, dest)
}

func ExtractTarXz(archive string, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()

	xzr, err := xz.NewReader(f)
	if err != nil {
		return err
	}

	return extractTar(xzr, dest)
}

func extractTar(r io.Reader, dest string) error {
	tr := tar.NewReader(r)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target, err := sanitizeExtractPath(dest, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				// Ignore symlink errors
			}
		}
	}
	return nil
}

func ExtractZip(archive string, dest string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		target, err := sanitizeExtractPath(dest, f.Name)
		if err != nil {
			return err
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func FindSubDirWithDir(candidateDir string, targetSub string) string {
	if fi, err := os.Stat(filepath.Join(candidateDir, targetSub)); err == nil && fi.IsDir() {
		return candidateDir
	}

	entries, err := os.ReadDir(candidateDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				subPath := filepath.Join(candidateDir, entry.Name())
				if fi, err := os.Stat(filepath.Join(subPath, targetSub)); err == nil && fi.IsDir() {
					return subPath
				}
			}
		}
	}

	return candidateDir
}
