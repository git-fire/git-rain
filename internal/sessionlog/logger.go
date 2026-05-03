package sessionlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/git-rain/git-rain/internal/safety"
)

// LogEntry matches git-fire's structured session event shape.
type LogEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Level       string    `json:"level"`
	Repo        string    `json:"repo,omitempty"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
	Error       string    `json:"error,omitempty"`
	Duration    string    `json:"duration,omitempty"`
}

// EventSubscriber receives structured entries as they are written.
type EventSubscriber func(LogEntry)

// Logger writes JSONL session logs.
type Logger struct {
	logPath     string
	file        *os.File
	writes      int
	subscribers []EventSubscriber
}

// NewLogger creates a new rain session logger.
func NewLogger(logDir string) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	logFilename := fmt.Sprintf("git-rain-%s.log", time.Now().Format("20060102-150405"))
	logPath := filepath.Join(logDir, logFilename)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}
	logger := &Logger{logPath: logPath, file: file}
	logger.Info("", "git-rain-start", "Git-rain session started")
	return logger, nil
}

// Subscribe registers a process-local subscriber.
func (l *Logger) Subscribe(fn EventSubscriber) {
	if fn == nil {
		return
	}
	l.subscribers = append(l.subscribers, fn)
}

// Log writes one JSONL entry.
func (l *Logger) Log(entry LogEntry) error {
	if l.file == nil {
		return fmt.Errorf("logger not initialized")
	}
	entry.Timestamp = time.Now()
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal log entry: %w", err)
	}
	if _, err := l.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write log entry: %w", err)
	}
	for _, fn := range l.subscribers {
		fn(entry)
	}
	l.writes++
	if l.writes%20 == 0 {
		return l.file.Sync()
	}
	return nil
}

// Info writes a level=info entry.
func (l *Logger) Info(repo, action, description string) {
	_ = l.Log(LogEntry{Level: "info", Repo: repo, Action: action, Description: description})
}

// Error writes a level=error entry with redaction.
func (l *Logger) Error(repo, action, description string, err error) {
	e := LogEntry{Level: "error", Repo: repo, Action: action, Description: description}
	if err != nil {
		e.Error = safety.SanitizeText(err.Error())
	}
	_ = l.Log(e)
}

// Success writes a level=success entry.
func (l *Logger) Success(repo, action, description string, duration time.Duration) {
	_ = l.Log(LogEntry{
		Level:       "success",
		Repo:        repo,
		Action:      action,
		Description: description,
		Duration:    duration.String(),
	})
}

// Close writes end marker and closes file.
func (l *Logger) Close() error {
	if l.file == nil {
		return nil
	}
	l.Info("", "git-rain-end", "Git-rain session ended")
	_ = l.file.Sync()
	return l.file.Close()
}

// LogPath returns current file path.
func (l *Logger) LogPath() string { return l.logPath }

// DefaultLogDir resolves standard cache path for rain logs.
func DefaultLogDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "git-rain", "logs")
	}
	return filepath.Join(base, "git-rain", "logs")
}
