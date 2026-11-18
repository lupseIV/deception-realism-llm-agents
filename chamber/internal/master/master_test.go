package master

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/traplog"
)

// fakeLayer records calls in a shared journal and emits one event on Start.
type fakeLayer struct {
	kind    string
	exports facts.Facts
	gotIn   facts.Facts
	journal *[]string
	mu      *sync.Mutex
	failOn  string
}

func (f *fakeLayer) log(s string) {
	f.mu.Lock()
	*f.journal = append(*f.journal, s)
	f.mu.Unlock()
}

func (f *fakeLayer) Kind() string { return f.kind }

func (f *fakeLayer) Build(_ context.Context, in facts.Facts) (facts.Facts, error) {
	f.gotIn = in
	f.log("build " + f.kind)
	if f.failOn == "build" {
		return nil, errors.New("boom")
	}
	return f.exports, nil
}

func (f *fakeLayer) Start(_ context.Context, rt Runtime, emit func(traplog.Event)) error {
	f.log("start " + f.kind)
	_ = rt.WriteFile(context.Background(), "/home/user/"+f.kind, []byte("x"), time.Time{})
	emit(traplog.Event{Kind: traplog.Request, Source: f.kind})
	return nil
}

func (f *fakeLayer) Stop(context.Context) error {
	f.log("stop " + f.kind)
	if f.failOn == "stop" {
		return errors.New("stop failed")
	}
	return nil
}

type fakeRuntime struct {
	mu    sync.Mutex
	files []string
}

func (r *fakeRuntime) WriteFile(_ context.Context, p string, _ []byte, _ time.Time) error {
	r.mu.Lock()
	r.files = append(r.files, p)
	r.mu.Unlock()
	return nil
}

func (r *fakeRuntime) AppendFile(ctx context.Context, p string, b []byte, mt time.Time) error {
	return r.WriteFile(ctx, p, b, mt)
}

type tree struct {
	root           *GroupMaster
	persona, creds *fakeLayer
	aws            *fakeLayer
	journal        *[]string
}

func newTree() tree {
	j := &[]string{}
	mu := &sync.Mutex{}
	persona := &fakeLayer{kind: "persona", exports: facts.Facts{"persona.user": "andrei"}, journal: j, mu: mu}
	creds := &fakeLayer{kind: "aws-credentials", exports: facts.Facts{"token.creds.bucket": "acme-backups"}, journal: j, mu: mu}
	aws := &fakeLayer{kind: "aws", journal: j, mu: mu}
	root := NewGroup("sandbox", "",
		NewGroup("environment", "low", NewLayer("persona", persona)),
		NewGroup("honeytoken", "high", NewLayer("aws-credentials", creds)),
		NewGroup("network", "faithful", NewLayer("aws", aws)),
	)
	return tree{root: root, persona: persona, creds: creds, aws: aws, journal: j}
}

func TestFactsFlowAndRealismPerGroup(t *testing.T) {
	tr := newTree()
	out, err := tr.root.Build(context.Background(), facts.Facts{"run.seed": "42"})
	if err != nil {
		t.Fatal(err)
	}
	if tr.creds.gotIn["persona.user"] != "andrei" || tr.creds.gotIn["run.seed"] != "42" {
		t.Fatalf("honeytoken layer must see persona facts: %v", tr.creds.gotIn)
	}
	if tr.aws.gotIn["token.creds.bucket"] != "acme-backups" {
		t.Fatalf("network layer must see honeytoken facts: %v", tr.aws.gotIn)
	}
	if tr.persona.gotIn[RealismKey] != "low" || tr.creds.gotIn[RealismKey] != "high" ||
		tr.aws.gotIn[RealismKey] != "faithful" {
		t.Fatal("each sub-master must pass its own realism level")
	}
	if out["persona.user"] == "" || out["token.creds.bucket"] == "" {
		t.Fatalf("root must return all exported facts: %v", out)
	}
	_ = tr.root.Stop(context.Background())
}

func TestLifecycleOrderAndEventPaths(t *testing.T) {
	tr := newTree()
	ctx := context.Background()
	var buf bytes.Buffer
	done := Drain(tr.root, traplog.New(&buf, "run-1"))

	rt := &fakeRuntime{}
	if _, err := tr.root.Build(ctx, facts.Facts{}); err != nil {
		t.Fatal(err)
	}
	if err := tr.root.Start(ctx, rt); err != nil {
		t.Fatal(err)
	}
	if err := tr.root.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	<-done

	want := []string{
		"build persona", "build aws-credentials", "build aws",
		"start persona", "start aws-credentials", "start aws",
		"stop aws", "stop aws-credentials", "stop persona",
	}
	if strings.Join(*tr.journal, ",") != strings.Join(want, ",") {
		t.Fatalf("order\n got %v\nwant %v", *tr.journal, want)
	}
	if len(rt.files) != 3 {
		t.Fatalf("expected 3 files written, got %v", rt.files)
	}

	paths := map[string]bool{}
	sc := bufio.NewScanner(&buf)
	for sc.Scan() {
		var e traplog.Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		if e.RunID != "run-1" {
			t.Fatalf("missing run id: %+v", e)
		}
		paths[e.Layer] = true
	}
	for _, p := range []string{"sandbox/environment/persona", "sandbox/honeytoken/aws-credentials", "sandbox/network/aws"} {
		if !paths[p] {
			t.Errorf("missing event path %s, got %v", p, paths)
		}
	}
}

func TestBuildErrorCarriesPath(t *testing.T) {
	tr := newTree()
	tr.creds.failOn = "build"
	_, err := tr.root.Build(context.Background(), facts.Facts{})
	if err == nil || !strings.Contains(err.Error(), "sandbox/honeytoken/aws-credentials") {
		t.Fatalf("error must name the failing master, got %v", err)
	}
	_ = tr.root.Stop(context.Background())
}

func TestStopContinuesAfterError(t *testing.T) {
	tr := newTree()
	tr.aws.failOn = "stop"
	err := tr.root.Stop(context.Background())
	if err == nil || !strings.Contains(err.Error(), "sandbox/network/aws") {
		t.Fatalf("expected stop error with path, got %v", err)
	}
	if len(*tr.journal) != 3 {
		t.Fatalf("all layers must be stopped, got %v", *tr.journal)
	}
	// Second stop must not panic.
	_ = tr.root.Stop(context.Background())
}
