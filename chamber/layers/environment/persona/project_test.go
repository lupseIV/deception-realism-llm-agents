package persona

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/layers/environment/wear"
)

// Every commit changes exactly the files it lists.
func testMeta(t *testing.T) *projectMeta {
	t.Helper()
	w, err := wear.Open("low")
	if err != nil {
		t.Fatal(err)
	}
	m, err := loadProject(w)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestEveryCommitHasARealDiff(t *testing.T) {
	m := testMeta(t)
	done := map[string]bool{}
	exists := map[string]bool{}
	state := map[string]string{}
	for _, pc := range m.Commits {
		if pc.Flag != "" {
			done[pc.Flag] = true
		}
		for _, f := range pc.Files {
			exists[f] = true
		}
		listed := map[string]bool{}
		for _, f := range pc.Files {
			listed[f] = true
		}
		for f := range exists {
			content, _ := m.render(f, done)
			if old, ok := state[f]; (!ok || old != content) != listed[f] {
				t.Errorf("%q: %s changed=%v but listed=%v", pc.Message, f, !ok || old != content, listed[f])
			}
			state[f] = content
		}
	}
	for f := range m.templates {
		if !exists[f] {
			t.Errorf("template %s is never committed", f)
		}
	}
	for _, f := range m.Sources {
		if !exists[f] {
			t.Errorf("history edits %s, which is not in the project", f)
		}
	}
	if strings.Contains(state["app/pricing.py"], "?") {
		t.Error("unrendered condition left in pricing.py")
	}
}

// loadProject resolves feature messages and rejects a broken project.
func TestLoadProjectChecks(t *testing.T) {
	m := testMeta(t)
	if m.Commits[5].Feature == "" || m.Commits[5].Message != "Store created_at in UTC" {
		t.Errorf("persona commits take their feature's message: %+v", m.Commits[5])
	}
	broken := fstest.MapFS{
		"low/project/meta.json": {Data: []byte(`{"clonedAfter": 1, "sources": ["a.py"],
			"colleagues": [], "features": [{"branch": "b", "message": "m"}],
			"commits": [{"author": 2, "files": ["a.py", "missing.py"]}, {"author": 0, "feature": "nope", "files": ["a.py"]}]}`)},
		"low/project/files/a.py":      {Data: []byte("a\n")},
		"low/project/files/orphan.py": {Data: []byte("o\n")},
	}
	w, err := wear.OpenFS(broken, "low")
	if err != nil {
		t.Fatal(err)
	}
	_, err = loadProject(w)
	for _, want := range []string{"author 2 unknown", "no message", "no file missing.py", "feature \"nope\" unknown", "orphan.py is never committed", "at least 4 sources"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want %q in %v", want, err)
		}
	}
}
