package persona

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/assemble"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/config"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/master"
)

const repo = "../../../.."

var ctx = context.Background()

func load(t *testing.T) config.Persona {
	t.Helper()
	p, err := config.LoadPersona(filepath.Join(repo, "personas"), "aws-developer")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLowExportsIdentity(t *testing.T) {
	l, err := New(assemble.LayerSpec{Kind: Kind, ID: "persona", Persona: load(t)})
	if err != nil {
		t.Fatal(err)
	}
	out, err := l.Build(ctx, facts.Facts{master.RealismKey: "low"})
	if err != nil {
		t.Fatal(err)
	}
	want := facts.Facts{
		facts.EnvUser: "andrei.pop", facts.EnvUID: "1000", facts.EnvHome: "/home/andrei.pop",
		facts.EnvShell: "/bin/bash", facts.EnvHostname: "vt-dev-07",
	}
	for k, v := range want {
		if out[k] != v {
			t.Errorf("%s = %q, want %q", k, out[k], v)
		}
	}
	if err := l.Start(ctx, nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestShellFromHistory(t *testing.T) {
	p := load(t)
	p.History.Shell = "zsh"
	if s := shell(p); s != "/bin/zsh" {
		t.Errorf("shell %q", s)
	}
	p.History = nil
	if s := shell(p); s != "/bin/bash" {
		t.Errorf("default shell %q", s)
	}
}

func TestOnlyLowIsImplemented(t *testing.T) {
	l, _ := New(assemble.LayerSpec{Kind: Kind, ID: "persona", Persona: load(t)})
	for level, want := range map[string]string{"medium": "not implemented yet", "high": "not implemented yet", "max": "unknown"} {
		if _, err := l.Build(ctx, facts.Facts{master.RealismKey: level}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want %q, got %v", level, want, err)
		}
	}
	p := load(t)
	p.Identity.User = ""
	if _, err := New(assemble.LayerSpec{Persona: p}); err == nil {
		t.Error("want error for a persona without user")
	}
}

// The control configuration assembles with this layer alone.
func TestControlThroughAssembler(t *testing.T) {
	c, err := config.Load(filepath.Join(repo, "configs/pilot/control.xml"), config.Options{BaseDir: repo})
	if err != nil {
		t.Fatal(err)
	}
	seed := uint64(1)
	res, err := assemble.Build(c, assemble.Run{ID: "r1", Seed: &seed},
		assemble.Factory{Kind: New}, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Root.Stop(ctx)
	out, err := res.Root.Build(ctx, res.Facts)
	if err != nil {
		t.Fatal(err)
	}
	if out[facts.EnvHostname] != "vt-dev-07" || out[facts.EnvUser] != "andrei.pop" {
		t.Errorf("facts: %v", out)
	}
}
