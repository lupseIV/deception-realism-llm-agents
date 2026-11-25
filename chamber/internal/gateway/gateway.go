// Package gateway is the runtime of the chamber container ("gateway" in the
// compose project). Layers write files into a bundle; the agent container
// fetches the bundle once at start-up over the internal network and installs
// it. Network layers get a TLS listener on port 443 for the AWS host names.
//
// The gateway has no access to the Docker socket; it only talks to the agent
// container over the network.
package gateway

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// Staging is where the bundle unpacks inside the agent container before the
// setup script moves the files to their place.
const Staging = "/tmp/.chamber"

// Runtime collects planted files and hands out the TLS listener.
type Runtime struct {
	TLS        *tls.Config // server certificate for the AWS names
	ListenAddr string      // default ":443"

	mu       sync.Mutex
	files    map[string]file
	listened bool
}

type file struct {
	data  []byte
	mtime time.Time
	dir   bool
}

// New returns a runtime that serves TLS with cfg.
func New(cfg *tls.Config) *Runtime {
	return &Runtime{TLS: cfg, ListenAddr: ":443", files: map[string]file{}}
}

// WriteFile records a file for the bundle. The path must be absolute. A path
// ending in "/" records an empty directory with that mtime (data must be
// empty); directories that only hold files need no entry of their own.
func (r *Runtime) WriteFile(_ context.Context, p string, data []byte, mtime time.Time) error {
	if !path.IsAbs(p) {
		return fmt.Errorf("path %q is not absolute", p)
	}
	dir := strings.HasSuffix(p, "/")
	if dir && len(data) > 0 {
		return fmt.Errorf("directory %q cannot have content", p)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.files[path.Clean(p)] = file{data: append([]byte(nil), data...), mtime: mtime, dir: dir}
	return nil
}

// AppendFile adds data to the end of a recorded file, or records a new one.
// The file keeps the later of its old and the given mtime.
func (r *Runtime) AppendFile(_ context.Context, p string, data []byte, mtime time.Time) error {
	if !path.IsAbs(p) {
		return fmt.Errorf("path %q is not absolute", p)
	}
	p = path.Clean(p)
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.files[p]
	if old.mtime.After(mtime) {
		mtime = old.mtime
	}
	r.files[p] = file{data: append(append([]byte(nil), old.data...), data...), mtime: mtime}
	return nil
}

// Listen returns the TLS listener for the network layer. Only one network
// service can listen, because all AWS names resolve to the gateway.
func (r *Runtime) Listen(_ context.Context, serviceID string) (net.Listener, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listened {
		return nil, fmt.Errorf("service %q: port %s is already taken by another network service", serviceID, r.ListenAddr)
	}
	if r.TLS == nil {
		return nil, fmt.Errorf("service %q: no TLS configuration", serviceID)
	}
	ln, err := tls.Listen("tcp", r.ListenAddr, r.TLS)
	if err != nil {
		return nil, err
	}
	r.listened = true
	return ln, nil
}

// Paths returns the recorded file paths, sorted.
func (r *Runtime) Paths() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	ps := make([]string, 0, len(r.files))
	for p := range r.files {
		ps = append(ps, p)
	}
	sort.Strings(ps)
	return ps
}

// Setup describes the agent account and trust store changes.
type Setup struct {
	User, UID, Home, Shell string
	CAName                 string // file name under /usr/local/share/ca-certificates, "" = no CA
	CAPEM                  []byte
}

// Bundle builds the tar the agent container installs. It holds the setup
// script at <Staging>/setup.sh and every file under <Staging>/root/<path>.
// Files below the user's home belong to the user; the rest to root.
func (r *Runtime) Bundle(s Setup) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	now := time.Now()
	add := func(name string, data []byte, mode int64, mtime time.Time, uid int) error {
		if err := tw.WriteHeader(&tar.Header{Name: strings.TrimPrefix(name, "/"), Mode: mode, Size: int64(len(data)),
			ModTime: mtime, Uid: uid, Gid: uid, Format: tar.FormatPAX}); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}
	uid := 0
	if _, err := fmt.Sscan(s.UID, &uid); err != nil {
		return nil, fmt.Errorf("uid %q: %w", s.UID, err)
	}

	// Directories in the home get their own entries, so they keep a past
	// mtime (the newest file below them) instead of the install time. Entries
	// go out in depth-first path order: tar fixes a directory's mtime once it
	// leaves the directory, so every file must follow its directory entry.
	dirs := homeDirs(r.files, s.Home)
	type entry struct {
		key, path string
		dir       bool
	}
	var entries []entry
	for d := range dirs {
		entries = append(entries, entry{key: d + "/", path: d, dir: true})
	}
	for p, f := range r.files {
		if !f.dir {
			entries = append(entries, entry{key: p, path: p})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
	for _, e := range entries {
		if e.dir {
			if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: strings.TrimPrefix(Staging+"/root"+e.path, "/") + "/",
				Mode: dirMode(e.path), ModTime: dirs[e.path], Uid: uid, Gid: uid, Format: tar.FormatPAX}); err != nil {
				return nil, err
			}
			continue
		}
		f := r.files[e.path]
		owner := 0
		if e.path == s.Home || strings.HasPrefix(e.path, s.Home+"/") {
			owner = uid
		}
		if err := add(Staging+"/root"+e.path, f.data, fileMode(e.path), f.mtime, owner); err != nil {
			return nil, err
		}
	}
	if s.CAName != "" {
		if err := add(Staging+"/root/usr/local/share/ca-certificates/"+s.CAName, s.CAPEM, 0o644, now.AddDate(-1, 0, 0), 0); err != nil {
			return nil, err
		}
	}
	if err := add(Staging+"/setup.sh", []byte(setupScript(s)), 0o700, now, 0); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// homeDirs returns every directory from home down to the recorded paths below
// it, with its mtime: the recorded one for an explicit directory, otherwise
// the newest mtime below it.
func homeDirs(files map[string]file, home string) map[string]time.Time {
	dirs := map[string]time.Time{}
	newest := func(d string, t time.Time) {
		if cur, ok := dirs[d]; !ok || t.After(cur) {
			dirs[d] = t
		}
	}
	for p, f := range files {
		if p != home && !strings.HasPrefix(p, home+"/") {
			continue
		}
		if f.dir {
			newest(p, f.mtime)
		}
		for d := path.Dir(p); d == home || strings.HasPrefix(d, home+"/"); d = path.Dir(d) {
			newest(d, f.mtime)
		}
	}
	for p, f := range files { // an explicit directory keeps its own mtime
		if f.dir && dirs[p].After(f.mtime) {
			dirs[p] = f.mtime
		}
	}
	return dirs
}

// dirMode follows what the tools create: ssh and the git credential cache
// keep their folders private.
func dirMode(d string) int64 {
	switch path.Base(d) {
	case ".ssh", "credential", ".gnupg":
		return 0o700
	}
	return 0o755
}

// fileMode follows what the owning tool creates: secrets, histories and
// editor state are private, git objects are read-only, ssh public keys are
// readable by all.
func fileMode(p string) int64 {
	base := path.Base(p)
	switch {
	case strings.Contains(p, "/.git/objects/"):
		return 0o444
	case strings.Contains(p, "/.ssh/") && strings.HasSuffix(base, ".pub"):
		return 0o644
	case strings.Contains(p, "/.aws/"), strings.Contains(p, "/.ssh/") && base != "known_hosts" && base != "config",
		strings.HasSuffix(base, "_history"), base == ".viminfo", base == ".lesshst":
		return 0o600
	}
	return 0o644
}

// setupScript creates the user with the distribution's home skeleton (also
// when the home folder already exists), then
// moves the planted files into place, keeping their timestamps.
func setupScript(s Setup) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#!/bin/sh\nset -e\n")
	fmt.Fprintf(&b, "id -u %[1]s >/dev/null 2>&1 || useradd -m -u %[2]s -s %[3]s -d %[4]s %[1]s\n", s.User, s.UID, s.Shell, s.Home)
	// Docker creates the home folder first when it is the container's working
	// directory; useradd then skips the skeleton, so copy what is missing.
	fmt.Fprintf(&b, "for f in /etc/skel/.[!.]* /etc/skel/*; do [ -e \"$f\" ] && [ ! -e %[1]s/\"${f##*/}\" ] && cp -a \"$f\" %[1]s/; done; true\n", s.Home)
	fmt.Fprintf(&b, "cp -a %s/root/. /\n", Staging)
	fmt.Fprintf(&b, "chown -R %[1]s:%[1]s %[2]s\n", s.User, s.Home) // also directories the bundle created
	if s.CAName != "" {
		fmt.Fprintf(&b, "update-ca-certificates >/dev/null 2>&1\n")
	}
	return b.String()
}

// ServeBundleOnce serves the bundle at GET /bundle.tar on addr and returns
// after the first successful download, or when ctx ends.
func ServeBundleOnce(ctx context.Context, addr string, bundle []byte) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	done := make(chan struct{})
	var once sync.Once
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/bundle.tar" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/x-tar")
		if _, err := w.Write(bundle); err == nil {
			once.Do(func() { close(done) })
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	select {
	case <-done:
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
	return ctx.Err()
}
