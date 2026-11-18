// Package wear is the store of the text the environment layer plants: the
// dotfiles, history vocabulary and project files of a lived-in home. The
// text lives in files, one folder per environment realism level and one
// subfolder per kind of data:
//
//	<level>/shell    shell startup files and history vocabulary
//	<level>/git      git config, global ignore, clone metadata files
//	<level>/ssh      ssh client config
//	<level>/editor   vim config and state
//	<level>/repl     pager and REPL history
//	<level>/project  the persona's project: meta.json and files/
//
// Open(level) looks a name up in that level first, then in every lower one
// (high -> medium -> low): a higher level replaces a file of the same name
// and can add new ones, so the level is the knob for how much is planted.
// Files ending in .tmpl are Go text/template files rendered with the data
// the caller passes; a missing field is an error. Other files are verbatim.
package wear

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"text/template"
)

//go:embed all:low all:medium all:high
var data embed.FS

// Levels from least to most; each level falls back to the ones before it.
var Levels = []string{"low", "medium", "high"}

// Store reads the text of one level.
type Store struct {
	fsys  fs.FS
	chain []string // the level first, then lower ones
}

// Open returns the store for a level.
func Open(level string) (*Store, error) { return openFS(data, level) }

// OpenFS is Open over another file tree with the same layout (tests).
func OpenFS(fsys fs.FS, level string) (*Store, error) { return openFS(fsys, level) }

func openFS(fsys fs.FS, level string) (*Store, error) {
	for i, l := range Levels {
		if l == level {
			chain := make([]string, 0, i+1)
			for j := i; j >= 0; j-- {
				chain = append(chain, Levels[j])
			}
			return &Store{fsys: fsys, chain: chain}, nil
		}
	}
	return nil, fmt.Errorf("wear: unknown level %q", level)
}

// resolve returns the stored path of a name and whether it is a template:
// "<level>/<name>" or "<level>/<name>.tmpl", in level order.
func (s *Store) resolve(name string) (string, bool, error) {
	for _, l := range s.chain {
		for _, tmpl := range []bool{false, true} {
			p := path.Join(l, name)
			if tmpl {
				p += ".tmpl"
			}
			if st, err := fs.Stat(s.fsys, p); err == nil && !st.IsDir() {
				return p, tmpl, nil
			}
		}
	}
	return "", false, fmt.Errorf("wear: %s not found in %s", name, strings.Join(s.chain, ", "))
}

// Text returns a file rendered with data (data is ignored for plain files).
func (s *Store) Text(name string, data any) (string, error) {
	p, tmpl, err := s.resolve(name)
	if err != nil {
		return "", err
	}
	b, err := fs.ReadFile(s.fsys, p)
	if err != nil {
		return "", err
	}
	if !tmpl {
		return string(b), nil
	}
	t, err := template.New(p).Option("missingkey=error").Parse(string(b))
	if err != nil {
		return "", fmt.Errorf("wear: %s: %w", p, err)
	}
	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return "", fmt.Errorf("wear: %s: %w", p, err)
	}
	return out.String(), nil
}

// Lines returns the non-empty lines of a file, without comment lines (#).
func (s *Store) Lines(name string, data any) ([]string, error) {
	text, err := s.Text(name, data)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out, nil
}

// Files lists the files below a folder name across the level chain, as
// paths relative to that folder (".tmpl" removed), sorted. A higher level's
// file hides the lower one of the same path.
func (s *Store) Files(dir string) ([]string, error) {
	seen := map[string]bool{}
	for _, l := range s.chain {
		root := path.Join(l, dir)
		if _, err := fs.Stat(s.fsys, root); err != nil {
			continue
		}
		err := fs.WalkDir(s.fsys, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if path.Base(p) == ".keep" {
				return nil
			}
			seen[strings.TrimSuffix(strings.TrimPrefix(p, root+"/"), ".tmpl")] = true
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("wear: folder %s not found in %s", dir, strings.Join(s.chain, ", "))
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}
