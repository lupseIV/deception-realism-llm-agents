package wear

import (
	"strings"
	"testing"
	"testing/fstest"
)

var testFS = fstest.MapFS{
	"low/shell/plain":         {Data: []byte("hello {{.Name}}\n")},
	"low/shell/greet.tmpl":    {Data: []byte("hi {{.Name}}\n")},
	"low/shell/list":          {Data: []byte("# comment\na\n\n b \n")},
	"low/project/files/x.py":  {Data: []byte("x\n")},
	"low/project/files/.hid":  {Data: []byte("h\n")},
	"medium/.keep":            {},
	"high/shell/greet.tmpl":   {Data: []byte("hello there {{.Name}}\n")},
	"high/project/files/y.py": {Data: []byte("y\n")},
}

func open(t *testing.T, level string) *Store {
	t.Helper()
	s, err := openFS(testFS, level)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPlainAndTemplate(t *testing.T) {
	s := open(t, "low")
	data := struct{ Name string }{"Andrei"}
	if got, _ := s.Text("shell/plain", data); got != "hello {{.Name}}\n" {
		t.Errorf("plain files are verbatim: %q", got)
	}
	if got, _ := s.Text("shell/greet", data); got != "hi Andrei\n" {
		t.Errorf("template: %q", got)
	}
	if _, err := s.Text("shell/greet", struct{ Other string }{}); err == nil {
		t.Error("a missing field must fail")
	}
	if _, err := s.Text("shell/none", data); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing file: %v", err)
	}
	if got, _ := s.Lines("shell/list", nil); strings.Join(got, "|") != "a|b" {
		t.Errorf("lines: %q", got)
	}
}

func TestLevelFallback(t *testing.T) {
	data := struct{ Name string }{"A"}
	if got, _ := open(t, "high").Text("shell/greet", data); got != "hello there A\n" {
		t.Errorf("high overrides low: %q", got)
	}
	if got, _ := open(t, "medium").Text("shell/greet", data); got != "hi A\n" {
		t.Errorf("medium falls back to low: %q", got)
	}
	if got, _ := open(t, "high").Text("shell/plain", data); !strings.HasPrefix(got, "hello") {
		t.Errorf("high falls back to low: %q", got)
	}
	low, _ := open(t, "low").Files("project/files")
	high, _ := open(t, "high").Files("project/files")
	if strings.Join(low, ",") != ".hid,x.py" || strings.Join(high, ",") != ".hid,x.py,y.py" {
		t.Errorf("files: low %v high %v", low, high)
	}
	if _, err := openFS(testFS, "max"); err == nil {
		t.Error("unknown level must fail")
	}
}

// The embedded store opens at every level.
func TestEmbedded(t *testing.T) {
	for _, l := range Levels {
		if _, err := Open(l); err != nil {
			t.Fatal(err)
		}
	}
}
