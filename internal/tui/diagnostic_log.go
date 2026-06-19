package tui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultDiagnosticLogMaxBytes = 2 * 1024 * 1024
	diagnosticLogBufferSize      = 256
)

type diagnosticSink interface {
	RecordDiagnosticEvent(diagnosticEvent)
}

type PersistentDiagnosticLog struct {
	path string
	file *os.File
	ch   chan diagnosticEvent
	wg   sync.WaitGroup
	once sync.Once
}

func DefaultDiagnosticLogPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "jira-tui", "diagnostics.jsonl"), nil
}

func OpenPersistentDiagnosticLog(path string) (*PersistentDiagnosticLog, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("diagnostic log path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := rotateDiagnosticLog(path, defaultDiagnosticLogMaxBytes); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	log := &PersistentDiagnosticLog{
		path: path,
		file: file,
		ch:   make(chan diagnosticEvent, diagnosticLogBufferSize),
	}
	log.wg.Add(1)
	go log.writeLoop()
	return log, nil
}

func rotateDiagnosticLog(path string, maxBytes int64) error {
	if maxBytes <= 0 {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() <= maxBytes {
		return nil
	}
	_ = os.Remove(path + ".1")
	return os.Rename(path, path+".1")
}

func (l *PersistentDiagnosticLog) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

func (l *PersistentDiagnosticLog) RecordDiagnosticEvent(event diagnosticEvent) {
	if l == nil {
		return
	}
	select {
	case l.ch <- event:
	default:
	}
}

func (l *PersistentDiagnosticLog) Close() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		close(l.ch)
		l.wg.Wait()
	})
	return l.file.Close()
}

func (l *PersistentDiagnosticLog) writeLoop() {
	defer l.wg.Done()
	encoder := json.NewEncoder(l.file)
	for event := range l.ch {
		_ = encoder.Encode(diagnosticLogRecord{
			Time:   event.At.Format(time.RFC3339Nano),
			Kind:   string(event.Kind),
			Label:  event.Label,
			Status: event.Status,
			Detail: redactDiagnosticText(event.Detail),
		})
	}
}

type diagnosticLogRecord struct {
	Time   string `json:"time"`
	Kind   string `json:"kind"`
	Label  string `json:"label,omitempty"`
	Status string `json:"status,omitempty"`
	Detail string `json:"detail,omitempty"`
}
