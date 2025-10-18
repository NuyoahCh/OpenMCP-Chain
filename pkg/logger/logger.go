package logger

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Config describes how the application logger should behave.
type Config struct {
	Level       string
	Format      string
	OutputPaths []string
	Audit       AuditConfig
}

// AuditConfig controls audit log output behaviour.
type AuditConfig struct {
	Enabled    bool
	Path       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
}

var (
	defaultLogger *slog.Logger
	auditLogger   *slog.Logger
	once          sync.Once
	closers       []io.Closer
	initErr       error
)

// Init configures the global logger instances.
func Init(cfg Config) error {
	once.Do(func() {
		level := parseLevel(cfg.Level)
		handlerOpts := &slog.HandlerOptions{Level: level, AddSource: true}

		handler, err := buildHandler(cfg.Format, cfg.OutputPaths, handlerOpts)
		if err != nil {
			initErr = err
			return
		}
		defaultLogger = slog.New(handler)

		auditLogger = defaultLogger
		if cfg.Audit.Enabled {
			audit, err := buildAuditLogger(cfg.Audit)
			if err != nil {
				initErr = err
				return
			}
			auditLogger = audit
		}
	})
	if initErr != nil {
		return initErr
	}
	if defaultLogger == nil {
		return errors.New("logger already initialised")
	}
	return nil
}

func buildHandler(format string, outputs []string, opts *slog.HandlerOptions) (slog.Handler, error) {
	writers := make([]io.Writer, 0, len(outputs))
	if len(outputs) == 0 {
		writers = append(writers, os.Stdout)
	} else {
		for _, out := range outputs {
			writer, closer, err := openWriter(out)
			if err != nil {
				return nil, err
			}
			if closer != nil {
				closers = append(closers, closer)
			}
			writers = append(writers, writer)
		}
	}

	var writer io.Writer
	if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = io.MultiWriter(writers...)
	}

	if strings.EqualFold(format, "text") {
		return slog.NewTextHandler(writer, opts), nil
	}
	return slog.NewJSONHandler(writer, opts), nil
}

func buildAuditLogger(cfg AuditConfig) (*slog.Logger, error) {
	if cfg.Path == "" {
		return nil, errors.New("audit log path cannot be empty when enabled")
	}
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = 100
	}
	if cfg.MaxBackups <= 0 {
		cfg.MaxBackups = 7
	}
	if cfg.MaxAgeDays <= 0 {
		cfg.MaxAgeDays = 30
	}

	writer, err := newRotatingWriter(cfg.Path, cfg.MaxSizeMB, cfg.MaxBackups, cfg.MaxAgeDays)
	if err != nil {
		return nil, err
	}
	closers = append(closers, writer)
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(handler), nil
}

func openWriter(path string) (io.Writer, io.Closer, error) {
	switch strings.ToLower(path) {
	case "stdout":
		return os.Stdout, nil, nil
	case "stderr":
		return os.Stderr, nil, nil
	default:
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, nil, fmt.Errorf("create log directory: %w", err)
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file %s: %w", path, err)
		}
		return file, file, nil
	}
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// L returns the structured logger instance.
func L() *slog.Logger {
	if defaultLogger == nil {
		_ = Init(Config{})
	}
	return defaultLogger
}

// Audit returns the audit logger.
func Audit() *slog.Logger {
	if auditLogger == nil {
		return L()
	}
	return auditLogger
}

// Sync flushes buffered log entries to their outputs.
func Sync() error {
	var err error
	for _, closer := range closers {
		err = errors.Join(err, closer.Close())
	}
	closers = nil
	return err
}

// Named returns a child logger with the provided component name.
func Named(name string) *slog.Logger {
	return L().WithGroup(name)
}
