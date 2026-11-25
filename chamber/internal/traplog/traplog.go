// Package traplog writes trap events as JSON lines. One line per event; the
// analysis scripts read these files to compute trigger rate, time to first
// trigger and progression depth.
package traplog

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Kind is the event type.
type Kind string

const (
	// Connect: a TCP connection reached a fake service (logged in refused mode).
	Connect Kind = "connect"
	// Request: an HTTP request reached a fake service.
	Request Kind = "request"
	// Exfiltrate: token material appeared in a request outside the
	// Authorization header (URL, body or another header).
	Exfiltrate Kind = "exfiltrate"
	// Use: the token access key was used to authenticate (SigV4 header).
	Use Kind = "use"
)

// Event is one trap log line.
type Event struct {
	Time       time.Time `json:"ts"`
	RunID      string    `json:"run_id"`
	Kind       Kind      `json:"kind"`
	Source     string    `json:"source"`
	Layer      string    `json:"layer,omitempty"`    // master path, e.g. sandbox/network/aws
	TokenID    string    `json:"token_id,omitempty"` // token id from the sandbox configuration
	Mode       string    `json:"mode,omitempty"`
	Remote     string    `json:"remote,omitempty"`
	Method     string    `json:"method,omitempty"`
	Host       string    `json:"host,omitempty"`
	Path       string    `json:"path,omitempty"`
	Status     int       `json:"status,omitempty"`
	TokenField string    `json:"token_field,omitempty"` // access_key_id | secret_access_key
	Where      string    `json:"where,omitempty"`       // authorization | url | body | header
}

// Logger is safe for concurrent use. It never stops on a write error: it
// counts the lost events and keeps the first error, see Lost.
type Logger struct {
	mu       sync.Mutex
	enc      *json.Encoder
	runID    string
	now      func() time.Time
	lost     int
	firstErr error
}

// New returns a logger that writes to w and stamps every event with runID.
func New(w io.Writer, runID string) *Logger {
	return &Logger{enc: json.NewEncoder(w), runID: runID, now: time.Now}
}

// Log writes one event. Time and RunID are filled in if empty.
func (l *Logger) Log(e Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e.Time.IsZero() {
		e.Time = l.now().UTC()
	}
	if e.RunID == "" {
		e.RunID = l.runID
	}
	if err := l.enc.Encode(e); err != nil {
		l.lost++
		if l.firstErr == nil {
			l.firstErr = err
		}
		return err
	}
	return nil
}

// Lost returns how many events could not be written, and the first error.
// A run with lost events has an incomplete trap log: its trigger counts are
// not valid.
func (l *Logger) Lost() (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lost, l.firstErr
}
