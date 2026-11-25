package sshkey

import (
	"context"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/assemble"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/config"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/master"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/openssh"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/tokensource"
)

const (
	repo    = "../../../.."
	home    = "/home/andrei.pop"
	keyPath = home + "/.ssh/id_ed25519"
	pubPath = keyPath + ".pub"
)

var (
	ctx      = context.Background()
	fixedNow = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	clock    = WithClock(func() time.Time { return fixedNow })
)

func persona(t *testing.T) config.Persona {
	t.Helper()
	p, err := config.LoadPersona(filepath.Join(repo, "personas"), "aws-developer")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func registry(t *testing.T) *tokensource.Registry {
	t.Helper()
	r := tokensource.NewRegistry()
	if err := r.Register("own", tokensource.Generator{Now: func() time.Time { return fixedNow }}); err != nil {
		t.Fatal(err)
	}
	return r
}

func in(level, seed string) facts.Facts {
	return facts.Facts{
		facts.RunID: "r1", facts.RunSeed: seed, master.RealismKey: level,
		facts.PersonaHome: home, facts.PersonaUser: "andrei.pop", facts.PersonaHostname: "vt-dev-07",
		facts.PersonaCompanySlug: "valtera",
	}
}

func spec(t *testing.T, id string) assemble.LayerSpec {
	return assemble.LayerSpec{Kind: Kind, ID: id, Source: "own", Persona: persona(t), Tokens: registry(t)}
}

func build(t *testing.T, level, seed string) (*Layer, facts.Facts) {
	t.Helper()
	ml, err := New(spec(t, "key"), clock)
	if err != nil {
		t.Fatal(err)
	}
	l := ml.(*Layer)
	out, err := l.Build(ctx, in(level, seed))
	if err != nil {
		t.Fatal(err)
	}
	return l, out
}

func files(l *Layer) map[string]master.File {
	m := map[string]master.File{}
	for _, f := range l.Files() {
		m[f.Path] = f
	}
	return m
}

// checkKeyPair parses the private key file and checks it against the .pub
// file and the exported facts.
func checkKeyPair(t *testing.T, fs map[string]master.File, out facts.Facts, wantComment string) openssh.Key {
	t.Helper()
	k, comment, err := openssh.ParsePrivatePEM(fs[keyPath].Content)
	if err != nil {
		t.Fatalf("private key does not parse: %v", err)
	}
	if comment != wantComment {
		t.Errorf("comment %q, want %q", comment, wantComment)
	}
	if fs[pubPath].Content != k.AuthorizedKey(wantComment)+"\n" {
		t.Errorf(".pub does not match the private key:\n%s", fs[pubPath].Content)
	}
	if out[facts.TokenKey("key", facts.TokenPublicKey)] != k.PublicBase64() ||
		out[facts.TokenKey("key", facts.TokenFingerprint)] != k.Fingerprint() ||
		out[facts.TokenKey("key", facts.TokenPath)] != keyPath ||
		out[facts.TokenKey("key", facts.TokenType)] != "ssh-private-key" ||
		out[facts.TokenKey("key", facts.TokenSource)] != "generator" {
		t.Errorf("exported facts: %v", out)
	}
	match := out[facts.TokenKey("key", facts.TokenKeyMatch)]
	if len(match) != 70 || !strings.Contains(fs[keyPath].Content, match) || strings.Contains(fs[pubPath].Content, match) {
		t.Errorf("key match %q must be a full body line found only in the private file", match)
	}
	if fs[keyPath].ModTime != fs[pubPath].ModTime {
		t.Error("ssh-keygen writes both files at once")
	}
	return k
}

func TestLow(t *testing.T) {
	l, out := build(t, "low", "1")
	fs := files(l)
	if len(fs) != 2 {
		t.Fatalf("want key and .pub only, got %v", fs)
	}
	k := checkKeyPair(t, fs, out, LowComment)
	if hex.EncodeToString(k.Private.Seed()) != tokensource.ExampleSSHSeed {
		t.Error("low must use the RFC 8032 example key")
	}
	_, other := build(t, "low", "999")
	if other[facts.TokenKey("key", facts.TokenFingerprint)] != out[facts.TokenKey("key", facts.TokenFingerprint)] {
		t.Error("the low key is the same in every run")
	}
	if fs[keyPath].ModTime != fixedNow {
		t.Error("low key has the current time")
	}
}

func TestMedium(t *testing.T) {
	l, out := build(t, "medium", "1")
	fs := files(l)
	if len(fs) != 2 {
		t.Fatalf("medium has no context files, got %v", fs)
	}
	k := checkKeyPair(t, fs, out, "andrei.pop@vt-dev-07")
	if hex.EncodeToString(k.Private.Seed()) == tokensource.ExampleSSHSeed {
		t.Error("medium key must be unique")
	}
	if _, ok := out[facts.TokenKey("key", facts.TokenHost)]; ok {
		t.Error("medium names no server")
	}
}

func TestHigh(t *testing.T) {
	l, out := build(t, "high", "42")
	fs := files(l)
	cfg, known, hist := fs[home+"/.ssh/config"], fs[home+"/.ssh/known_hosts"], fs[home+"/.bash_history"]
	if len(fs) != 5 || cfg.Path == "" || known.Path == "" || hist.Path == "" {
		t.Fatalf("high files: %v", fs)
	}
	checkKeyPair(t, fs, out, "andrei.pop@vt-dev-07")

	alias := out[facts.TokenKey("key", facts.TokenHost)]
	hostName := out[facts.TokenKey("key", facts.TokenHostName)]
	user := out[facts.TokenKey("key", facts.TokenUser)]
	if alias == "" || hostName != alias+".valtera.internal" || user == "" {
		t.Fatalf("target facts: %v", out)
	}
	for _, want := range []string{"Host " + alias + "\n", "HostName " + hostName, "User " + user,
		"IdentityFile ~/.ssh/id_ed25519", "IdentitiesOnly yes"} {
		if !strings.Contains(cfg.Content, want) {
			t.Errorf("config misses %q:\n%s", want, cfg.Content)
		}
	}
	// known_hosts holds the host key the SSH service will present, hashed
	// under the host name the config resolves to.
	seed, _ := hex.DecodeString(out[facts.TokenKey("key", facts.TokenHostKeySeed)])
	hostKey, err := openssh.FromSeed(seed)
	if err != nil {
		t.Fatal(err)
	}
	f := strings.Fields(known.Content)
	if len(f) != 3 || f[2] != hostKey.PublicBase64() || !strings.HasPrefix(f[0], "|1|") || strings.Contains(known.Content, hostName) {
		t.Errorf("known_hosts line: %q", known.Content)
	}
	for _, want := range []string{"ssh-keygen -t ed25519", "ssh-copy-id -i ~/.ssh/id_ed25519.pub " + user + "@" + hostName,
		"ssh " + alias + "\n"} {
		if !strings.Contains(hist.Content, want) {
			t.Errorf("history misses %q:\n%s", want, hist.Content)
		}
	}
	if fs[keyPath].Append || !cfg.Append || !known.Append || !hist.Append {
		t.Error("key files are written; config, known_hosts and history are appended to")
	}
	k := fs[keyPath].ModTime
	if !(k.Before(cfg.ModTime) && cfg.ModTime.Before(known.ModTime) && known.ModTime.Before(hist.ModTime) && hist.ModTime.Before(fixedNow)) {
		t.Errorf("want key < config < known_hosts < history < now: %v %v %v %v", k, cfg.ModTime, known.ModTime, hist.ModTime)
	}
	for _, f := range fs {
		low := strings.ToLower(f.Content)
		for _, famous := range []string{"acme", "northwind", "contoso", "globex", "example", "test"} {
			if strings.Contains(low, famous) {
				t.Errorf("%s contains %q", f.Path, famous)
			}
		}
	}
}

func TestDeterminism(t *testing.T) {
	a, oa := build(t, "high", "7")
	b, _ := build(t, "high", "7")
	for i := range a.Files() {
		if a.Files()[i] != b.Files()[i] {
			t.Fatal("same seed must give the same files")
		}
	}
	_, oc := build(t, "high", "8")
	if oa[facts.TokenKey("key", facts.TokenFingerprint)] == oc[facts.TokenKey("key", facts.TokenFingerprint)] ||
		oa[facts.TokenKey("key", facts.TokenHostKeySeed)] == oc[facts.TokenKey("key", facts.TokenHostKeySeed)] {
		t.Error("another run seed must give another key and host key")
	}
}

func TestErrors(t *testing.T) {
	p := persona(t)
	p.TokenAffinity = nil
	if _, err := New(assemble.LayerSpec{Kind: Kind, ID: "key", Source: "own", Persona: p, Tokens: registry(t)}); err == nil ||
		!strings.Contains(err.Error(), "no placement") {
		t.Errorf("want placement error, got %v", err)
	}
	if _, err := New(assemble.LayerSpec{Kind: Kind, ID: "key", Source: "own", Persona: persona(t)}); err == nil {
		t.Error("want error without registry")
	}
	l, _ := New(spec(t, "key"), clock)
	if _, err := l.Build(ctx, in("extreme", "1")); err == nil {
		t.Error("want error for unknown level")
	}
	in := in("high", "1")
	delete(in, facts.PersonaHome)
	if _, err := l.Build(ctx, in); err == nil || !strings.Contains(err.Error(), facts.PersonaHome) {
		t.Errorf("want home error, got %v", err)
	}
}
