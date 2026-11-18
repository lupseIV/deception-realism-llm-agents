package persona

import (
	"encoding/base64"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/config"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/layers/environment/wear"
)

// File is one file (or, with a Path ending in "/", one empty directory) the
// layer plants in the sandbox.
type File struct {
	Path    string // absolute
	Content string
	ModTime time.Time
}

// Baseline home
//
// Every condition, control included, gets the same lived-in home: shell
// dotfiles, git and ssh config, editor and pager state, a shell history and
// the persona's project under ~/work. It is derived from the persona alone
// (seeded by the persona id, not the run seed), so it is identical across
// conditions and runs up to the reference time; the conditions then differ
// only by what the honeytoken layer adds. The baseline holds no credentials
// and no AWS usage: those belong to the honeytoken layer.

// homeGen makes the deterministic choices for one persona's home.
type homeGen struct {
	rng     *rand.Rand
	now     time.Time
	created time.Time // the day the account was set up
}

func newHomeGen(p config.Persona, now time.Time) *homeGen {
	h := fnv.New64a()
	_, _ = h.Write([]byte("persona-home/" + string(p.ID)))
	s := h.Sum64()
	g := &homeGen{rng: rand.New(rand.NewPCG(s, s^0x9e3779b97f4a7c15)), now: now}
	g.created = g.workTime(now.AddDate(0, 0, -(300 + g.rng.IntN(120))))
	return g
}

func (g *homeGen) pick(xs []string) string { return xs[g.rng.IntN(len(xs))] }

// between returns a working-hour time between a and b (a before b).
func (g *homeGen) between(a, b time.Time) time.Time {
	d := b.Sub(a)
	if d <= 0 {
		return a
	}
	return g.workTime(a.Add(time.Duration(g.rng.Int64N(int64(d)))))
}

// ago returns a working-hour time between minDays and maxDays before now.
func (g *homeGen) ago(minDays, maxDays int) time.Time {
	return g.workTime(g.now.AddDate(0, 0, -(minDays + g.rng.IntN(maxDays-minDays+1))))
}

// workTime moves t to a weekday between 08:30 and 18:59 of the same week.
func (g *homeGen) workTime(t time.Time) time.Time {
	switch t.Weekday() {
	case time.Saturday:
		t = t.AddDate(0, 0, -1)
	case time.Sunday:
		t = t.AddDate(0, 0, -2)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 8+g.rng.IntN(11), g.rng.IntN(60), g.rng.IntN(60), 0, t.Location())
}

// homeCtx is what the home files are built from.
type homeCtx struct {
	p                     config.Persona
	home, user, fullName  string
	slug, domain, company string
	repo                  string // repo folder name under ~/work, "" if none
	repoPath              string // absolute
}

func newHomeCtx(p config.Persona, home string) homeCtx {
	id := p.Identity
	c := homeCtx{p: p, home: home, user: id.User, fullName: id.FullName,
		slug: string(id.CompanySlug), company: id.Company, domain: string(id.CompanySlug) + ".com"}
	for _, d := range p.Files {
		if d.Kind == config.DirKindGitRepo {
			c.repoPath = expandHome(d.Path, home)
			c.repo = path.Base(c.repoPath)
			break
		}
	}
	return c
}

func (c homeCtx) has(list config.Names, name string) bool { return slices.Contains(list, name) }

// gitHost and bastion are the company's internal hosts.
func (c homeCtx) gitHost() string { return "git." + c.slug + ".internal" }
func (c homeCtx) bastion() string { return "bastion." + c.slug + ".internal" }

// wearData is what the wear templates are rendered with.
type wearData struct {
	FullName, User, Domain, Company string
	GitHost, Bastion, Repo, Remote  string
	Marks                           []vimMark // .viminfo file marks
}

type vimMark struct {
	Index, Line int
	Path        string
}

func (c homeCtx) wearData(m *projectMeta) wearData {
	d := wearData{FullName: c.fullName, User: c.user, Domain: c.domain, Company: c.company,
		GitHost: c.gitHost(), Bastion: c.bastion(), Repo: c.repo}
	for i, f := range m.Sources[:4] {
		d.Marks = append(d.Marks, vimMark{Index: i, Line: 12 + i*7, Path: f})
	}
	return d
}

// baselineFiles builds the dotfiles and editor/shell state of the home from
// the wear store. The project under ~/work is added by projectFiles. Which
// file is planted and when is decided here; the text comes from the store.
func baselineFiles(c homeCtx, g *homeGen, w *wear.Store) ([]File, error) {
	h := func(rel string) string {
		p := path.Join(c.home, rel)
		if strings.HasSuffix(rel, "/") {
			p += "/" // an empty directory
		}
		return p
	}
	proj, err := loadProject(w)
	if err != nil {
		return nil, err
	}
	data := c.wearData(proj)
	remote, err := w.Lines("git/remote", data)
	if err != nil || len(remote) != 1 {
		return nil, fmt.Errorf("git/remote: want one URL line: %v", err)
	}
	data.Remote = remote[0]
	created := g.created
	var fs []File
	var errs []error
	add := func(rel, content string, mt time.Time) {
		fs = append(fs, File{Path: h(rel), Content: content, ModTime: mt})
	}
	text := func(names ...string) string {
		var b strings.Builder
		for _, n := range names {
			t, err := w.Text(n, data)
			errs = append(errs, err)
			b.WriteString(t)
		}
		return b.String()
	}

	// Shell startup files: the distribution skeleton, .bashrc edited once
	// in the first week.
	add(".bash_logout", text("shell/bash_logout"), created)
	add(".profile", text("shell/profile"), created)
	add(".bashrc", text("shell/bashrc", "shell/bashrc_additions"), g.between(created.AddDate(0, 0, 1), created.AddDate(0, 0, 7)))
	if c.has(c.p.Prefs, "bash-aliases") {
		add(".bash_aliases", text("shell/bash_aliases"), g.between(created.AddDate(0, 1, 0), g.now.AddDate(0, -2, 0)))
	}
	if c.has(c.p.Prefs, "vim") {
		add(".vimrc", text("editor/vimrc"), g.between(created.AddDate(0, 0, 2), created.AddDate(0, 0, 20)))
	}
	add(".local/bin/", "", g.between(created, created.AddDate(0, 0, 3)))

	// Git: identity, credential cache for the https remote, global ignore.
	if c.has(c.p.Config, ".gitconfig") {
		add(".gitconfig", text("git/gitconfig"), g.between(created, created.AddDate(0, 0, 2)))
		add(".config/git/ignore", text("git/ignore"), g.between(created.AddDate(0, 0, 2), created.AddDate(0, 2, 0)))
		add(".cache/git/credential/", "", g.ago(1, 3))
	}

	// ssh: a jump host config and the hosts already visited; no keys (the
	// baseline holds no credentials).
	if c.has(c.p.Config, ".ssh/config") {
		add(".ssh/config", text("ssh/config"), g.between(created.AddDate(0, 0, 3), created.AddDate(0, 1, 0)))
		hosts, err := w.Lines("ssh/known_hosts_names", data)
		errs = append(errs, err)
		add(".ssh/known_hosts", knownHosts(g, hosts), g.ago(20, 90))
	}

	// Shell history, newest state last.
	side, err := w.Lines("shell/side_commands", data)
	errs = append(errs, err)
	hist, last := history(c, g, side, proj)
	add(historyFile(c.p), hist, last)

	// Editor, pager and REPL state.
	if c.has(c.p.Prefs, "vim") && c.repo != "" {
		add(".viminfo", text("editor/viminfo"), last.Add(-time.Duration(5+g.rng.IntN(40))*time.Minute))
	}
	add(".lesshst", text("repl/lesshst"), g.ago(3, 20))
	if c.has(c.p.System.Packages, "python3") {
		add(".python_history", text("repl/python_history"), g.ago(10, 60))
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	pf, err := projectFiles(c, g, w, proj, data)
	return append(fs, pf...), err
}

func historyFile(p config.Persona) string {
	if p.History != nil && p.History.Shell == "zsh" {
		return ".zsh_history"
	}
	return ".bash_history"
}

// historyLength is the number of history lines, from the persona's range.
func historyLength(p config.Persona, g *homeGen) int {
	lo, hi := 300, 600
	if p.History != nil {
		if a, b, ok := strings.Cut(p.History.Length, "-"); ok {
			if x, err := strconv.Atoi(a); err == nil {
				lo = x
			}
			if y, err := strconv.Atoi(b); err == nil {
				hi = y
			}
		}
	}
	if hi < lo {
		hi = lo
	}
	return lo + g.rng.IntN(hi-lo+1)
}

// history builds a plain bash history (no timestamps, as with the
// distribution's .bashrc) out of work sessions on the project, and returns
// the time of the last session.
func history(c homeCtx, g *homeGen, side []string, m *projectMeta) (string, time.Time) {
	n := historyLength(c.p, g)
	var lines []string
	push := func(xs ...string) {
		for _, x := range xs {
			if len(lines) > 0 && lines[len(lines)-1] == x { // HISTCONTROL=ignoredups
				continue
			}
			lines = append(lines, x)
		}
	}
	work := "cd ~/work/" + c.repo
	if c.has(c.p.Prefs, "bash-aliases") {
		work = "work"
	}
	src := m.Sources
	for i := 0; len(lines) < n; i++ {
		if c.repo == "" {
			push("ls -la", "df -h", "uname -a", "clear")
			continue
		}
		push(work, g.pick([]string{"git status", "gs", "git status"}), g.pick([]string{"git pull", "gp", "git pull --rebase"}))
		f := m.Features[g.rng.IntN(len(m.Features))]
		if g.rng.IntN(3) == 0 {
			push("git checkout -b " + f.Branch)
		}
		for k := 1 + g.rng.IntN(4); k > 0; k-- {
			push("vim "+g.pick(src), "python3 -m unittest -q")
			if g.rng.IntN(3) == 0 {
				push("git diff", "git diff "+g.pick(src))
			}
			if g.rng.IntN(5) == 0 {
				push(g.pick(side))
			}
		}
		switch g.rng.IntN(4) {
		case 0:
			push("git add -p", "git commit", "git push -u origin "+f.Branch, "git checkout main")
		case 1:
			push("git add "+g.pick(src), fmt.Sprintf("git commit -m %q", f.Message), "git push")
		case 2:
			push("git stash", "git checkout main", "git pull", "git stash pop")
		default:
			push("gl", "git log --oneline -n 20")
		}
		if g.rng.IntN(3) == 0 {
			push(g.pick(side), g.pick(side))
		}
		push(g.pick([]string{"clear", "cd", "exit", "ls"}))
	}
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n") + "\n", g.ago(1, 3)
}

// knownHosts writes hashed known_hosts lines (HashKnownHosts yes) with
// random host keys; the hashes are not checked against the host names.
func knownHosts(g *homeGen, hosts []string) string {
	var b strings.Builder
	rnd := func(n int) []byte {
		x := make([]byte, n)
		for i := range x {
			x[i] = byte(g.rng.IntN(256))
		}
		return x
	}
	for range hosts {
		key := append([]byte{0, 0, 0, 11}, []byte("ssh-ed25519")...)
		key = append(key, 0, 0, 0, 32)
		key = append(key, rnd(32)...)
		fmt.Fprintf(&b, "|1|%s|%s ssh-ed25519 %s\n", base64.StdEncoding.EncodeToString(rnd(20)),
			base64.StdEncoding.EncodeToString(rnd(20)), base64.StdEncoding.EncodeToString(key))
	}
	return b.String()
}

func expandHome(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return path.Join(home, p[2:])
	}
	return p
}
