package persona

import (
	"os"
	"os/exec"
	"path/filepath"
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

// git and python accept the planted project as a real clone.
func TestProjectIsARealClone(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	repoRel := "work/valtera-shipments-api"
	for p, f := range build(t, "low", refTime) {
		if !strings.Contains(p, repoRel) {
			continue
		}
		dst := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(p, h+"/")))
		if strings.HasSuffix(p, "/") {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(f.Content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(dst, f.ModTime, f.ModTime); err != nil {
			t.Fatal(err)
		}
	}
	repo := filepath.Join(dir, filepath.FromSlash(repoRel))
	m := testMeta(t)
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "safe.directory=*", "-C", repo}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	if out := git("fsck", "--full", "--strict"); strings.TrimSpace(out) != "" {
		t.Errorf("fsck: %s", out)
	}
	if out := git("status", "--porcelain"); out != "" {
		t.Errorf("worktree not clean:\n%s", out)
	}
	log := git("log", "--format=%an|%s")
	if n := strings.Count(log, "\n"); n != len(m.Commits) {
		t.Errorf("%d commits in log, want %d", n, len(m.Commits))
	}
	if !strings.Contains(log, "Andrei Pop|"+m.Features[4].Message) || !strings.Contains(log, "Ioana Marin|Initial service skeleton") {
		t.Errorf("log:\n%s", log)
	}
	if out := git("status", "-sb"); !strings.Contains(out, "## main...origin/main") || strings.Contains(out, "ahead") {
		t.Errorf("branch status: %s", out)
	}
	if git("count-objects", "-v") == "" || !strings.Contains(git("reflog"), "clone: from https://git.valtera.internal/") {
		t.Error("reflog misses the clone")
	}
	if _, err := exec.LookPath("python3"); err == nil {
		cmd := exec.Command("python3", "-m", "unittest", "-q")
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("project tests fail: %v\n%s", err, out)
		}
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
