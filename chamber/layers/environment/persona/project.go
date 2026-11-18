package persona

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

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

// projectFiles builds the worktree and .git of the project: a clone made a
// few days after the account was set up, followed by the persona's and
// colleagues' later commits.
func projectFiles(c homeCtx, g *homeGen, w *wear.Store, m *projectMeta, data wearData) ([]File, error) {
	if c.repo == "" {
		return nil, nil
	}
	remote := data.Remote
	clone := g.between(g.created.AddDate(0, 0, 1), g.created.AddDate(0, 0, 3))
	times := commitTimes(g, clone, m)

	store := newObjectStore()
	done := map[string]bool{}
	exists := map[string]bool{}
	state := map[string]string{}
	changed := map[string]time.Time{}
	var ids []objID
	var head *objID
	for i, pc := range m.Commits {
		if pc.Flag != "" {
			done[pc.Flag] = true
		}
		for _, f := range pc.Files {
			exists[f] = true
		}
		for f := range exists {
			content, _ := m.render(f, done)
			if old, ok := state[f]; !ok || old != content {
				changed[f] = times[i]
			}
			state[f] = content
		}
		name, email := c.fullName, c.user+"@"+c.domain
		if pc.Author > 0 {
			col := m.Colleagues[pc.Author-1]
			name, email = col.Name, col.User+"@"+c.domain
		}
		id := store.writeCommit(store.writeTree(state), head, ident(name, email, times[i]), pc.Message)
		ids = append(ids, id)
		head = &ids[len(ids)-1]
	}

	gitDir := path.Join(c.repoPath, ".git")
	var fs []File
	add := func(rel string, content []byte, mt time.Time) {
		p := path.Join(gitDir, rel)
		if strings.HasSuffix(rel, "/") {
			p += "/" // an empty directory
		}
		fs = append(fs, File{Path: p, Content: string(content), ModTime: mt})
	}
	me := ident(c.fullName, c.user+"@"+c.domain, clone)
	zero := strings.Repeat("0", 40)
	last := times[len(times)-1]

	// Worktree: unchanged files keep the clone time.
	var index []indexEntry
	for f, content := range state {
		mt := changed[f]
		if mt.Before(clone) {
			mt = clone
		}
		fs = append(fs, File{Path: path.Join(c.repoPath, f), Content: content, ModTime: mt})
		index = append(index, indexEntry{path: f, id: gitObject{"blob", []byte(content)}.id(), size: len(content), mtime: mt})
	}

	// What clone wrote.
	cloned := ids[m.ClonedAfter-1]
	name, packData, idxData := store.pack(store.reachable(cloned))
	add("objects/pack/"+name+".pack", packData, clone)
	add("objects/pack/"+name+".idx", idxData, clone)
	add("objects/info/", nil, clone)
	add("refs/tags/", nil, clone)
	var errs []error
	for rel, name := range map[string]string{"description": "git/description", "info/exclude": "git/info_exclude", "config": "git/repo_config"} {
		text, err := w.Text(name, data)
		errs = append(errs, err)
		add(rel, []byte(text), clone)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	add("packed-refs", []byte(fmt.Sprintf("# pack-refs with: peeled fully-peeled sorted \n%s refs/remotes/origin/main\n", cloned)), clone)
	add("refs/remotes/origin/HEAD", []byte("ref: refs/remotes/origin/main\n"), clone)
	add("logs/refs/remotes/origin/HEAD", []byte(fmt.Sprintf("%s %s %s\tclone: from %s\n", zero, cloned, me, remote)), clone)

	// Later commits: loose objects and one reflog line each.
	reflog := fmt.Sprintf("%s %s %s\tclone: from %s\n", zero, cloned, me, remote)
	packed := map[objID]bool{}
	for _, id := range store.reachable(cloned) {
		packed[id] = true
	}
	for i := m.ClonedAfter; i < len(ids); i++ {
		for _, id := range store.reachable(ids[i]) {
			if !packed[id] {
				packed[id] = true
				p, z := store.loose(id)
				add(p, z, times[i])
			}
		}
		msg := "pull --rebase (finish): returning to refs/heads/main"
		if m.Commits[i].Author == 0 {
			msg = "commit: " + m.Commits[i].Message
		}
		reflog += fmt.Sprintf("%s %s %s\t%s\n", ids[i-1], ids[i], ident(c.fullName, c.user+"@"+c.domain, times[i]), msg)
	}
	add("logs/HEAD", []byte(reflog), last)
	add("logs/refs/heads/main", []byte(reflog), last)
	add("HEAD", []byte("ref: refs/heads/main\n"), clone)
	add("refs/heads/main", []byte(ids[len(ids)-1].String()+"\n"), last)
	add("refs/remotes/origin/main", []byte(ids[len(ids)-1].String()+"\n"), last)
	add("ORIG_HEAD", []byte(ids[len(ids)-2].String()+"\n"), last)
	add("FETCH_HEAD", []byte(fmt.Sprintf("%s\t\tbranch 'main' of %s\n", ids[len(ids)-1], strings.TrimSuffix(remote, ".git"))), last)
	add("index", gitIndex(index, 1000), last)
	return fs, nil
}

// commitTimes spreads the commits before the clone over the two months
// before it, and the later ones from a week after the clone to a few days
// ago, in order.
func commitTimes(g *homeGen, clone time.Time, m *projectMeta) []time.Time {
	var ts []time.Time
	pre := func(a, b time.Time, n int) {
		var xs []time.Time
		for range n {
			xs = append(xs, g.between(a, b))
		}
		sort.Slice(xs, func(i, j int) bool { return xs[i].Before(xs[j]) })
		ts = append(ts, xs...)
	}
	pre(clone.AddDate(0, -3, 0), clone.AddDate(0, 0, -2), m.ClonedAfter)
	pre(clone.AddDate(0, 0, 7), g.now.AddDate(0, 0, -4), len(m.Commits)-m.ClonedAfter)
	for i := 1; i < len(ts); i++ { // strictly increasing
		if !ts[i].After(ts[i-1]) {
			ts[i] = ts[i-1].Add(37 * time.Minute)
		}
	}
	return ts
}
