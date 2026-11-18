package persona

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/layers/environment/wear"
)

// The persona's project under ~/work: a small Python service whose source,
// commits and branches the shell history and editor state refer to. Its
// text lives in the wear store under project/: meta.json (sources, feature
// branches, colleagues, commits) and files/ (the repo tree). Files are
// templates: a line starting with "?flag " is kept only once the commit
// that sets flag is in the history, "?!flag " only before it. That gives
// every commit a real diff.

// feature is one piece of the persona's own work: a branch and its commit.
type feature struct {
	Branch  string `json:"branch"`
	Message string `json:"message"`
}

// projectCommit is one commit of the project history, oldest first.
type projectCommit struct {
	Author  int      `json:"author"`            // 0 = the persona, n = colleagues[n-1]
	Message string   `json:"message,omitempty"` // first line; persona commits take their feature's
	Feature string   `json:"feature,omitempty"` // branch of a persona commit
	Flag    string   `json:"flag,omitempty"`    // flag the commit turns on, "" for none
	Files   []string `json:"files"`
}

type colleague struct {
	Name string `json:"name"`
	User string `json:"user"`
}

// projectMeta is project/meta.json plus the file templates.
type projectMeta struct {
	ClonedAfter int             `json:"clonedAfter"` // commits that existed at the persona's clone
	Sources     []string        `json:"sources"`     // files edited in the history, most edited first
	Colleagues  []colleague     `json:"colleagues"`
	Features    []feature       `json:"features"`
	Commits     []projectCommit `json:"commits"`

	templates map[string]string // repo path -> template
}

// loadProject reads and checks the project from the store.
func loadProject(w *wear.Store) (*projectMeta, error) {
	raw, err := w.Text("project/meta.json", nil)
	if err != nil {
		return nil, err
	}
	var m projectMeta
	dec := json.NewDecoder(strings.NewReader(raw))
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("project/meta.json: %w", err)
	}
	names, err := w.Files("project/files")
	if err != nil {
		return nil, err
	}
	m.templates = map[string]string{}
	for _, n := range names {
		if m.templates[n], err = w.Text("project/files/"+n, nil); err != nil {
			return nil, err
		}
	}
	var errs []error
	branches := map[string]string{}
	for _, f := range m.Features {
		branches[f.Branch] = f.Message
	}
	committed := map[string]bool{}
	for i := range m.Commits {
		c := &m.Commits[i]
		if c.Author < 0 || c.Author > len(m.Colleagues) {
			errs = append(errs, fmt.Errorf("commit %d: author %d unknown", i, c.Author))
		}
		if c.Feature != "" {
			msg, ok := branches[c.Feature]
			if !ok {
				errs = append(errs, fmt.Errorf("commit %d: feature %q unknown", i, c.Feature))
			}
			c.Message = msg
		}
		if c.Message == "" {
			errs = append(errs, fmt.Errorf("commit %d: no message", i))
		}
		for _, f := range c.Files {
			committed[f] = true
			if _, ok := m.templates[f]; !ok {
				errs = append(errs, fmt.Errorf("commit %d: no file %s in project/files", i, f))
			}
		}
	}
	for f := range m.templates {
		if !committed[f] {
			errs = append(errs, fmt.Errorf("project/files/%s is never committed", f))
		}
	}
	if m.ClonedAfter < 1 || m.ClonedAfter >= len(m.Commits) {
		errs = append(errs, fmt.Errorf("clonedAfter %d out of range", m.ClonedAfter))
	}
	if len(m.Sources) < 4 || len(m.Features) == 0 {
		errs = append(errs, errors.New("need at least 4 sources and one feature"))
	}
	for _, s := range m.Sources {
		if !committed[s] {
			errs = append(errs, fmt.Errorf("source %s is not in the project", s))
		}
	}
	return &m, errors.Join(errs...)
}

// render returns the file at the state where the flags in done are set, and
// whether it exists in that state.
func (m *projectMeta) render(name string, done map[string]bool) (string, bool) {
	tpl, ok := m.templates[name]
	if !ok {
		return "", false
	}
	var b strings.Builder
	for _, line := range strings.SplitAfter(tpl, "\n") {
		keep := true
		for keep && strings.HasPrefix(line, "?") { // conditions can be chained
			cond, rest, ok := strings.Cut(line[1:], " ")
			if !ok { // a conditional empty line: "?flag\n"
				cond, rest = strings.TrimSuffix(line[1:], "\n"), "\n"
			}
			neg := strings.HasPrefix(cond, "!")
			keep = done[strings.TrimPrefix(cond, "!")] != neg
			line = rest
		}
		if keep {
			b.WriteString(line)
		}
	}
	return b.String(), true
}
