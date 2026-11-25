package traplog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestLogWritesOneJSONLinePerEvent(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, "run-1")
	l.now = func() time.Time { return time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC) }
	if err := l.Log(Event{Kind: Use, Source: "fakesvc", TokenField: "access_key_id"}); err != nil {
		t.Fatal(err)
	}
	var e Event
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &e); err != nil {
		t.Fatal(err)
	}
	if e.RunID != "run-1" || e.Kind != Use || e.Time.IsZero() {
		t.Fatalf("unexpected event %+v", e)
	}
}

func TestConcurrentLogging(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, "run-2")
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = l.Log(Event{Kind: Request, Source: "test"}) }()
	}
	wg.Wait()
	sc := bufio.NewScanner(&buf)
	n := 0
	for sc.Scan() {
		var e Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("line %d not valid JSON: %v", n, err)
		}
		n++
	}
	if n != 100 {
		t.Fatalf("expected 100 lines, got %d", n)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// A broken log volume must not look like a run without triggers.
func TestLostEventsAreCounted(t *testing.T) {
	l := New(failWriter{}, "run-3")
	for i := 0; i < 3; i++ {
		if err := l.Log(Event{Kind: Use, Source: "test"}); err == nil {
			t.Fatal("expected a write error")
		}
	}
	n, err := l.Lost()
	if n != 3 || err == nil || err.Error() != "disk full" {
		t.Fatalf("got %d lost, first error %v", n, err)
	}
}
