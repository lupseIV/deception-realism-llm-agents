package assemble

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/config"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/master"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/tokensource"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/traplog"
)

const repo = "../../.."

// recorder is a fake layer that records what it got and emits one event.
type recorder struct {
	spec  LayerSpec
	gotIn facts.Facts
	issue func(ctx context.Context, in facts.Facts) error // optional extra work in Build
}

func (r *recorder) Kind() string { return r.spec.Kind }

func (r *recorder) Build(ctx context.Context, in facts.Facts) (facts.Facts, error) {
	r.gotIn = in
	if r.issue != nil {
		return nil, r.issue(ctx, in)
	}
	return nil, nil
}

func (r *recorder) Start(_ context.Context, _ master.Runtime, emit func(traplog.Event)) error {
	emit(traplog.Event{Kind: traplog.Request, Source: r.spec.Kind})
	return nil
}

func (r *recorder) Stop(context.Context) error { return nil }

// fakeFactory returns a factory for all pilot kinds; seen collects the
// recorders by "<kind>/<id>".
func fakeFactory(seen map[string]*recorder, kinds ...string) Factory {
	var mu sync.Mutex
	newRec := func(spec LayerSpec) (master.Layer, error) {
		r := &recorder{spec: spec}
		mu.Lock()
		seen[spec.Kind+"/"+spec.ID] = r
		mu.Unlock()
		return r, nil
	}
	if len(kinds) == 0 {
		kinds = []string{KindPersona, HoneytokenKind(config.TokenTypeAWSCredentials), NetworkKind(config.NetworkServiceTypeAWS)}
	}
	f := Factory{}
	for _, k := range kinds {
		f[k] = newRec
	}
	return f
}

type nopRuntime struct{}

func (nopRuntime) WriteFile(context.Context, string, []byte, time.Time) error { return nil }

func load(t *testing.T, name string) config.Condition {
	t.Helper()
	c, err := config.Load(filepath.Join(repo, "configs", "pilot", name+".xml"), config.Options{BaseDir: repo})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func seed(v uint64) *uint64 { return &v }

// run builds, starts and stops the tree and returns the event layer paths.
func run(t *testing.T, res Result) map[string]bool {
	t.Helper()
	ctx := context.Background()
	var buf bytes.Buffer
	done := master.Drain(res.Root, traplog.New(&buf, "run"))
	if _, err := res.Root.Build(ctx, res.Facts); err != nil {
		t.Fatal(err)
	}
	if err := res.Root.Start(ctx, nopRuntime{}); err != nil {
		t.Fatal(err)
	}
	if err := res.Root.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	<-done
	paths := map[string]bool{}
	sc := bufio.NewScanner(&buf)
	for sc.Scan() {
		var e traplog.Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		paths[e.Layer] = true
	}
	return paths
}

func TestControlHasOnlyEnvironment(t *testing.T) {
	seen := map[string]*recorder{}
	res, err := Build(load(t, "control"), Run{ID: "r1", Seed: seed(1)}, fakeFactory(seen), repo)
	if err != nil {
		t.Fatal(err)
	}
	paths := run(t, res)
	if !reflect.DeepEqual(paths, map[string]bool{"sandbox/environment/persona": true}) {
		t.Fatalf("paths: %v", paths)
	}
	if len(seen) != 1 {
		t.Fatalf("want only the persona layer, got %v", seen)
	}
}

func TestTrapConditionTree(t *testing.T) {
	seen := map[string]*recorder{}
	res, err := Build(load(t, "ht-high-net-faithful"), Run{ID: "r1", Seed: seed(42)}, fakeFactory(seen), repo)
	if err != nil {
		t.Fatal(err)
	}
	paths := run(t, res)
	for _, p := range []string{"sandbox/environment/persona", "sandbox/honeytoken/creds", "sandbox/network/aws"} {
		if !paths[p] {
			t.Errorf("missing path %s in %v", p, paths)
		}
	}

	persona := seen["environment/persona/persona"]
	token := seen["honeytoken/aws-credentials/creds"]
	svc := seen["network/aws/aws"]
	if persona == nil || token == nil || svc == nil {
		t.Fatalf("layers not created: %v", seen)
	}
	// Specs carry the configuration.
	if token.spec.Source != "own" || token.spec.Realism != "high" || svc.spec.Bind != "creds" || svc.spec.Realism != "faithful" {
		t.Errorf("specs: token %+v, service %+v", token.spec, svc.spec)
	}
	// Facts reach the layers, with each group's realism.
	if token.gotIn[master.RealismKey] != "high" || svc.gotIn[master.RealismKey] != "faithful" ||
		persona.gotIn[master.RealismKey] != "low" {
		t.Errorf("realism: persona %q token %q service %q",
			persona.gotIn[master.RealismKey], token.gotIn[master.RealismKey], svc.gotIn[master.RealismKey])
	}
	in := token.gotIn
	if in[facts.RunID] != "r1" || in[facts.RunSeed] != "42" || in[facts.PersonaUser] != "andrei.pop" ||
		in[facts.PersonaHome] != "/home/andrei.pop" || in[facts.PersonaCompanySlug] != "valtera" ||
		in[facts.TokenPathKey("aws-credentials")] != "~/.aws/credentials" {
		t.Errorf("facts: %v", in)
	}
}

func TestSeed(t *testing.T) {
	c := load(t, "control")
	f := fakeFactory(map[string]*recorder{})

	if _, err := Build(c, Run{ID: "r1"}, f, repo); err == nil || !strings.Contains(err.Error(), "no seed") {
		t.Fatalf("want missing seed error, got %v", err)
	}
	if _, err := Build(c, Run{Seed: seed(1)}, f, repo); err == nil || !strings.Contains(err.Error(), "run id") {
		t.Fatalf("want missing run id error, got %v", err)
	}

	c.Sandbox.Seed = seed(7)
	res, err := Build(c, Run{ID: "r1"}, f, repo)
	if err != nil || res.Facts[facts.RunSeed] != "7" {
		t.Fatalf("config seed not used: %v %v", err, res.Facts[facts.RunSeed])
	}
	_ = res.Root.Stop(context.Background())

	a, _ := Build(c, Run{ID: "r1", Seed: seed(9)}, f, repo)
	b, _ := Build(c, Run{ID: "r1", Seed: seed(9)}, f, repo)
	if a.Facts[facts.RunSeed] != "9" || !reflect.DeepEqual(a.Facts, b.Facts) {
		t.Fatalf("run seed must win and facts must be deterministic: %v %v", a.Facts, b.Facts)
	}
	_ = a.Root.Stop(context.Background())
	_ = b.Root.Stop(context.Background())
}

func TestPoolSourceIsRegistered(t *testing.T) {
	dir := t.TempDir()
	pool := tokensource.PoolFile{Entries: []tokensource.PoolEntry{{
		ID: "p1", Type: tokensource.AWSCredentials, Origin: "test",
		Fields: map[string]string{"access_key_id": "AKIAPOOLPOOLPOOLPOOL", "secret_access_key": "s"},
	}}}
	if err := pool.Save(filepath.Join(dir, "pool.json")); err != nil {
		t.Fatal(err)
	}
	c := load(t, "ht-low-net-refused")
	c.Sandbox.Sources = []config.Source{{ID: "pool", Type: config.SourceTypePool, File: "pool.json"}}
	c.Sandbox.HoneyTokens.Tokens[0].Source = "pool"

	var got tokensource.Material
	seen := map[string]*recorder{}
	f := fakeFactory(seen)
	f[HoneytokenKind(config.TokenTypeAWSCredentials)] = func(spec LayerSpec) (master.Layer, error) {
		r := &recorder{spec: spec}
		// The layer issues its token through the registry, as a real layer will.
		r.issue = func(ctx context.Context, in facts.Facts) error {
			m, err := spec.Tokens.Issue(ctx, spec.Source, tokensource.TokenRequest{
				TokenID: spec.ID, Type: tokensource.AWSCredentials, RunID: in[facts.RunID], Seed: 1, Level: spec.Realism})
			got = m
			return err
		}
		return r, nil
	}

	res, err := Build(c, Run{ID: "r1", Seed: seed(1)}, f, dir)
	if err != nil {
		t.Fatal(err)
	}
	run(t, res)
	if got.Source != "pool:test" || got.Fields["access_key_id"] != "AKIAPOOLPOOLPOOLPOOL" || got.TokenID != "creds" {
		t.Fatalf("token not issued from the pool: %+v", got)
	}

	// A missing pool file fails at assembly.
	if _, err := Build(c, Run{ID: "r1", Seed: seed(1)}, f, t.TempDir()); err == nil {
		t.Fatal("want error for a missing pool file")
	}
}

func TestUnknownKind(t *testing.T) {
	f := fakeFactory(map[string]*recorder{}, KindPersona, HoneytokenKind(config.TokenTypeAWSCredentials))
	_, err := Build(load(t, "ht-high-net-faithful"), Run{ID: "r1", Seed: seed(1)}, f, repo)
	if err == nil || !strings.Contains(err.Error(), `no layer for kind "network/aws"`) ||
		!strings.Contains(err.Error(), "environment/persona") {
		t.Fatalf("want unknown kind error listing known kinds, got %v", err)
	}
}
