package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type rotatingWriter struct {
	mu         sync.Mutex
	file       *os.File
	path       string
	maxSize    int64
	maxBackups int
	maxAge     time.Duration
	size       int64
}

func newRotatingWriter(path string, maxSizeMB, maxBackups, maxAgeDays int) (*rotatingWriter, error) {
	if path == "" {
		return nil, errors.New("path is required")
	}
	if maxSizeMB <= 0 {
		maxSizeMB = 100
	}
	if maxBackups <= 0 {
		maxBackups = 7
	}
	if maxAgeDays <= 0 {
		maxAgeDays = 30
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create audit log directory: %w", err)
	}
	writer := &rotatingWriter{
		path:       path,
		maxSize:    int64(maxSizeMB) * 1024 * 1024,
		maxBackups: maxBackups,
		maxAge:     time.Duration(maxAgeDays) * 24 * time.Hour,
	}
	return writer, nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.ensureFile(); err != nil {
		return 0, err
	}
	if w.needsRotate(len(p)) {
		if err := w.rotate(); err != nil {
			return 0, err
		}
		if err := w.ensureFile(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	w.size = 0
	return err
}

func (w *rotatingWriter) ensureFile() error {
	if w.file != nil {
		return nil
	}
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("stat audit log: %w", err)
	}
	w.file = file
	w.size = info.Size()
	return nil
}

func (w *rotatingWriter) needsRotate(incoming int) bool {
	if w.maxSize <= 0 {
		return false
	}
	return w.size+int64(incoming) > w.maxSize
}

func (w *rotatingWriter) rotate() error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	w.size = 0

	if w.maxBackups > 0 {
		for i := w.maxBackups - 1; i >= 1; i-- {
			src := fmt.Sprintf("%s.%d", w.path, i)
			dst := fmt.Sprintf("%s.%d", w.path, i+1)
			if _, err := os.Stat(src); err == nil {
				_ = os.Rename(src, dst)
			}
		}
		if _, err := os.Stat(w.path); err == nil {
			_ = os.Rename(w.path, fmt.Sprintf("%s.1", w.path))
		}
	} else {
		_ = os.Remove(w.path)
	}

	w.cleanupByAge()
	return nil
}

func (w *rotatingWriter) cleanupByAge() {
	if w.maxAge <= 0 {
		return
	}
	cutoff := time.Now().Add(-w.maxAge)
	for i := 1; i <= w.maxBackups; i++ {
		path := fmt.Sprintf("%s.%d", w.path, i)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
		}
	}
}
