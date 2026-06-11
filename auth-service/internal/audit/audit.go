package audit

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type Event struct {
	Timestamp string `json:"timestamp"`
	User      string `json:"user"`
	Role      string `json:"role"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
	Result    string `json:"result"`
}

type Logger struct {
	mu   sync.Mutex
	file *os.File
}

func NewLogger(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &Logger{file: f}, nil
}

func (l *Logger) Log(event Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	event.Timestamp = time.Now().UTC().Format(time.RFC3339)

	b, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = l.file.Write(append(b, '\n'))
	return err
}

func (l *Logger) Close() error {
	return l.file.Close()
}
