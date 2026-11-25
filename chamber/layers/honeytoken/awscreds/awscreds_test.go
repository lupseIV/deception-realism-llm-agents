package awscreds

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/assemble"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/config"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/master"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/tokensource"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/traplog"
)

const repo = "../../../.."

var (
	ctx      = context.Background()
	fixedNow = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	clock    = WithClock(func() time.Time { return fixedNow })
	reKeyID  = regexp.MustCompile(`AKIA[A-Z2-7]{16}`)
)

func persona(t *testing.T) config.Persona {
	t.Helper()
	p, err := config.LoadPersona(filepath.Join(repo, "personas"), "aws-developer")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func registry(t *testing.T, extra ...tokensource.PoolEntry) *tokensource.Registry {
	t.Helper()
	r := tokensource.NewRegistry()
	if err := r.Register("own", tokensource.Generator{Now: func() time.Time { return fixedNow }}); err != nil {
		t.Fatal(err)
	}
	if len(extra) > 0 {
		if err := r.Register("pool", tokensource.NewPool(tokensource.PoolFile{Entries: extra})); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

// in returns the facts the assembler and the honeytoken group would pass.
func in(level string, seed string) facts.Facts {
	return facts.Facts{
		facts.RunID: "r1", facts.RunSeed: seed, master.RealismKey: level,
		facts.PersonaHome: "/home/andrei.pop", facts.PersonaRegion: "eu-central-1",
		facts.PersonaCompanySlug: "valtera",
	}
}

func build(t *testing.T, spec assemble.LayerSpec, level, seed string) (*Layer, facts.Facts) {
	t.Helper()
	ml, err := New(spec, clock)
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

func spec(t *testing.T, source string, r *tokensource.Registry) assemble.LayerSpec {
	return assemble.LayerSpec{Kind: Kind, ID: "creds", Source: source, Persona: persona(t), Tokens: r}
}

func files(l *Layer) map[string]master.File {
	m := map[string]master.File{}
	for _, f := range l.Files() {
		m[f.Path] = f
	}
	return m
}

const credsPath = "/home/andrei.pop/.aws/credentials"

func TestLow(t *testing.T) {
	l, out := build(t, spec(t, "own", registry(t)), "low", "1")
	fs := files(l)
	c, ok := fs[credsPath]
	if len(fs) != 1 || !ok {
		t.Fatalf("want only %s, got %v", credsPath, fs)
	}
	if !strings.HasPrefix(c.Content, "# test credentials") || !strings.Contains(c.Content, "[default]") ||
		!strings.Contains(c.Content, "EXAMPLE") {
		t.Errorf("low content:\n%s", c.Content)
	}
	if out[facts.TokenKey("creds", facts.TokenAccessKeyID)] != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("low must use the example key, got %v", out)
	}
}

func TestMedium(t *testing.T) {
	l, out := build(t, spec(t, "own", registry(t)), "medium", "1")
	fs := files(l)
	c := fs[credsPath]
	if len(fs) != 1 || strings.Contains(c.Content, "#") || !strings.Contains(c.Content, "[default]") {
		t.Fatalf("medium files: %v", fs)
	}
	key := out[facts.TokenKey("creds", facts.TokenAccessKeyID)]
	if !reKeyID.MatchString(key) || strings.Contains(key, "EXAMPLE") || !strings.Contains(c.Content, key) {
		t.Errorf("medium key %q in\n%s", key, c.Content)
	}
	if _, ok := out[facts.TokenKey("creds", facts.TokenBucket)]; ok {
		t.Error("medium has no bucket")
	}
}

func TestHigh(t *testing.T) {
	l, out := build(t, spec(t, "own", registry(t)), "high", "42")
	fs := files(l)
	cfg, creds, hist := fs["/home/andrei.pop/.aws/config"], fs[credsPath], fs["/home/andrei.pop/.bash_history"]
	if len(fs) != 3 || cfg.Path == "" || creds.Path == "" || hist.Path == "" {
		t.Fatalf("high files: %v", fs)
	}
	profile := out[facts.TokenKey("creds", facts.TokenProfile)]
	bucket := out[facts.TokenKey("creds", facts.TokenBucket)]
	if !strings.HasPrefix(profile, "valtera-") || !strings.HasPrefix(bucket, "valtera-") {
		t.Errorf("names must come from the persona: profile %q bucket %q", profile, bucket)
	}
	if !strings.Contains(creds.Content, "["+profile+"]") || !strings.Contains(cfg.Content, "[profile "+profile+"]") ||
		!strings.Contains(cfg.Content, "region = eu-central-1") {
		t.Errorf("config/credentials:\n%s\n%s", cfg.Content, creds.Content)
	}
	if !strings.Contains(hist.Content, "--profile "+profile) || !strings.Contains(hist.Content, "s3://"+bucket+"/") {
		t.Errorf("history:\n%s", hist.Content)
	}
	if !hist.Append || cfg.Append || creds.Append {
		t.Error("only the history lines are appended, to the environment's history")
	}
	if !(cfg.ModTime.Before(creds.ModTime) && creds.ModTime.Before(hist.ModTime) && hist.ModTime.Before(fixedNow)) {
		t.Errorf("want config < credentials < history < now: %v %v %v", cfg.ModTime, creds.ModTime, hist.ModTime)
	}
	for _, f := range fs {
		low := strings.ToLower(f.Content)
		for _, famous := range []string{"acme", "northwind", "contoso", "globex", "example"} {
			if strings.Contains(low, famous) {
				t.Errorf("%s contains well-known sample name %q", f.Path, famous)
			}
		}
	}
	if out[facts.TokenKey("creds", facts.TokenPath)] != credsPath || out[facts.TokenKey("creds", facts.TokenSource)] != "generator" {
		t.Errorf("exported facts: %v", out)
	}
}

func TestDeterminism(t *testing.T) {
	a, _ := build(t, spec(t, "own", registry(t)), "high", "7")
	b, _ := build(t, spec(t, "own", registry(t)), "high", "7")
	for i := range a.Files() {
		if a.Files()[i] != b.Files()[i] {
			t.Fatal("same seed must give the same files")
		}
	}
	s := spec(t, "own", registry(t))
	s.ID = "other"
	_, o1 := build(t, s, "high", "7")
	_, o2 := build(t, spec(t, "own", registry(t)), "high", "7")
	if o1[facts.TokenKey("other", facts.TokenAccessKeyID)] == o2[facts.TokenKey("creds", facts.TokenAccessKeyID)] {
		t.Fatal("different token ids must get different keys")
	}
}

func TestPoolSourceAndRegion(t *testing.T) {
	entry := tokensource.PoolEntry{ID: "ct-0", Type: tokensource.AWSCredentials, Origin: "canarytokens",
		Fields: map[string]string{"access_key_id": "AKIAYVP4CIPPG6FXFYHS", "secret_access_key": "secretsecret"},
		Facts:  facts.Facts{"region": "us-east-2", "profile": "default"}}
	_, out := build(t, spec(t, "pool", registry(t, entry)), "high", "1")
	if out[facts.TokenKey("creds", facts.TokenAccessKeyID)] != "AKIAYVP4CIPPG6FXFYHS" ||
		out[facts.TokenKey("creds", facts.TokenSource)] != "pool:canarytokens" {
		t.Errorf("pool token not used: %v", out)
	}
	if out[facts.TokenKey("creds", facts.TokenRegion)] != "us-east-2" {
		t.Errorf("region from the token source must win over the persona: %v", out)
	}
	if !strings.HasPrefix(out[facts.TokenKey("creds", facts.TokenProfile)], "valtera-") {
		t.Errorf("profile comes from the persona, not the pool: %v", out)
	}
}

func TestErrors(t *testing.T) {
	p := persona(t)
	p.TokenAffinity = nil
	if _, err := New(assemble.LayerSpec{Kind: Kind, ID: "creds", Source: "own", Persona: p, Tokens: registry(t)}); err == nil ||
		!strings.Contains(err.Error(), "no placement") {
		t.Errorf("want placement error, got %v", err)
	}
	if _, err := New(assemble.LayerSpec{Kind: Kind, ID: "creds", Source: "own", Persona: persona(t)}); err == nil {
		t.Error("want error without registry")
	}
	l, _ := New(spec(t, "missing", registry(t)), clock)
	if _, err := l.Build(ctx, in("high", "1")); err == nil || !strings.Contains(err.Error(), "unknown token source") {
		t.Errorf("want unknown source error, got %v", err)
	}
	l, _ = New(spec(t, "own", registry(t)), clock)
	if _, err := l.Build(ctx, in("high", "")); err == nil || !strings.Contains(err.Error(), facts.RunSeed) {
		t.Errorf("want seed error, got %v", err)
	}
}

// --- end to end through the assembler -----------------------------------------------

type fakeLayer struct{ kind string }

func (f fakeLayer) Kind() string                                          { return f.kind }
func (fakeLayer) Build(context.Context, facts.Facts) (facts.Facts, error) { return nil, nil }
func (fakeLayer) Start(context.Context, master.Runtime, func(traplog.Event)) error {
	return nil
}
func (fakeLayer) Stop(context.Context) error { return nil }

type memRuntime struct {
	mu    sync.Mutex
	files map[string]string
}

func (m *memRuntime) WriteFile(_ context.Context, p string, b []byte, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[p] = string(b)
	return nil
}

func (m *memRuntime) AppendFile(_ context.Context, p string, b []byte, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[p] += string(b)
	return nil
}

func TestThroughAssembler(t *testing.T) {
	c, err := config.Load(filepath.Join(repo, "configs/pilot/ht-high-net-faithful.xml"), config.Options{BaseDir: repo})
	if err != nil {
		t.Fatal(err)
	}
	fake := func(spec assemble.LayerSpec) (master.Layer, error) { return fakeLayer{spec.Kind}, nil }
	f := assemble.Factory{
		assemble.KindPersona: fake,
		Kind:                 func(s assemble.LayerSpec) (master.Layer, error) { return New(s, clock) },
		assemble.NetworkKind(config.NetworkServiceTypeAWS): fake,
	}
	seed := uint64(42)
	res, err := assemble.Build(c, assemble.Run{ID: "r1", Seed: &seed}, f, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Root.Stop(ctx)
	out, err := res.Root.Build(ctx, res.Facts)
	if err != nil {
		t.Fatal(err)
	}
	rt := &memRuntime{files: map[string]string{}}
	if err := res.Root.Start(ctx, rt, func(traplog.Event) {}); err != nil {
		t.Fatal(err)
	}
	key := out[facts.TokenKey("creds", facts.TokenAccessKeyID)]
	if !strings.Contains(rt.files[credsPath], key) || key == "" || len(rt.files) != 3 {
		t.Fatalf("planted files %v, key %q", rt.files, key)
	}
}
