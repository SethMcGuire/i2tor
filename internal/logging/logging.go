package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Logger struct {
	mu    sync.Mutex
	level int
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

var levelOrder = map[string]int{
	"debug": 0,
	"info":  1,
	"warn":  2,
	"error": 3,
}

const (
	rotatedLogsToKeep = 3
)

func New(ctx context.Context, logFilePath, level string) (*Logger, error) {
	_ = ctx
	logDir := filepath.Dir(logFilePath)
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	if err := rotate(logFilePath); err != nil {
		return nil, err
	}
	pruneRotated(logDir, rotatedLogsToKeep)
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", logFilePath, err)
	}
	return &Logger{
		level: resolveLevel(level),
		out:   io.MultiWriter(os.Stdout, f),
		file:  f,
	}, nil
}

func NewConsole(level string) *Logger {
	return &Logger{
		level: resolveLevel(level),
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
	if resolveLevel(level) < l.level {
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

func resolveLevel(level string) int {
	if order, ok := levelOrder[level]; ok {
		return order
	}
	return levelOrder["info"]
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

func pruneRotated(logDir string, keep int) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return
	}
	var rotated []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "i2tor-") && strings.HasSuffix(e.Name(), ".log") {
			rotated = append(rotated, filepath.Join(logDir, e.Name()))
		}
	}
	sort.Strings(rotated)
	if len(rotated) <= keep {
		return
	}
	for _, path := range rotated[:len(rotated)-keep] {
		_ = os.Remove(path)
	}
}
