package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	mu    sync.Mutex
	level string
	out   io.Writer
	file  *os.File
}

type entry struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Component string         `json:"component"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
}

func New(ctx context.Context, logFilePath, level string) (*Logger, error) {
	_ = ctx
	if err := os.MkdirAll(filepath.Dir(logFilePath), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	if err := rotate(logFilePath); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", logFilePath, err)
	}
	return &Logger{
		level: level,
		out:   io.MultiWriter(os.Stdout, f),
		file:  f,
	}, nil
}

func NewConsole(level string) *Logger {
	return &Logger{
		level: level,
		out:   os.Stdout,
	}
}

func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

func (l *Logger) Info(component, message string, fields map[string]any) {
	l.log("info", component, message, fields)
}

func (l *Logger) Warn(component, message string, fields map[string]any) {
	l.log("warn", component, message, fields)
}

func (l *Logger) Error(component, message string, fields map[string]any) {
	l.log("error", component, message, fields)
}

func (l *Logger) log(level, component, message string, fields map[string]any) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	e := entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level,
		Component: component,
		Message:   message,
		Fields:    fields,
	}
	encoded, err := json.Marshal(e)
	if err != nil {
		fmt.Fprintf(os.Stderr, `{"timestamp":"%s","level":"error","component":"logging","message":"failed to encode log entry","fields":{"error":"%s"}}`+"\n", time.Now().UTC().Format(time.RFC3339Nano), err)
		return
	}
	_, _ = l.out.Write(append(encoded, '\n'))
}

func rotate(current string) error {
	if _, err := os.Stat(current); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat current log %q: %w", current, err)
	}
	rotated := filepath.Join(filepath.Dir(current), fmt.Sprintf("i2tor-%s.log", time.Now().UTC().Format("20060102-150405")))
	if err := os.Rename(current, rotated); err != nil {
		return fmt.Errorf("rotate log %q: %w", current, err)
	}
	return nil
}
