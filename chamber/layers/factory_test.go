package layers

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/assemble"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/config"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/master"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/traplog"
)

const repo = "../.."

type recRuntime struct{ files map[string]string }

func (r *recRuntime) WriteFile(_ context.Context, p string, b []byte, _ time.Time) error {
	r.files[p] = string(b)
	return nil
}

func (r *recRuntime) AppendFile(_ context.Context, p string, b []byte, _ time.Time) error {
	r.files[p] += string(b)
	return nil
}

type nopLayer struct{ kind string }

func (n nopLayer) Kind() string                                                   { return n.kind }
func (nopLayer) Build(context.Context, facts.Facts) (facts.Facts, error)          { return facts.Facts{}, nil }
func (nopLayer) Start(context.Context, master.Runtime, func(traplog.Event)) error { return nil }
func (nopLayer) Stop(context.Context) error                                       { return nil }

// plant runs the persona and honeytoken layers of a pilot configuration and
// returns the planted files (the network layer is not started).
func plant(t *testing.T, name string) map[string]string {
	t.Helper()
	return plantFile(t, filepath.Join("pilot", name+".xml"))
}

// plantFile does the same for any configuration under configs/.
func plantFile(t *testing.T, rel string) map[string]string {
	t.Helper()
	ctx := context.Background()
	c, err := config.Load(filepath.Join(repo, "configs", rel), config.Options{BaseDir: repo})
	if err != nil {
		t.Fatal(err)
	}
	f := Factory("")
	f[assemble.NetworkKind(config.NetworkServiceTypeAWS)] = func(s assemble.LayerSpec) (master.Layer, error) {
		return nopLayer{s.Kind}, nil
	}
	seed := uint64(11)
	res, err := assemble.Build(c, assemble.Run{ID: "r-" + filepath.Base(rel), Seed: &seed}, f, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Root.Stop(ctx)
	if _, err := res.Root.Build(ctx, res.Facts); err != nil {
		t.Fatal(err)
	}
	rt := &recRuntime{files: map[string]string{}}
	if err := res.Root.Start(ctx, rt, func(traplog.Event) {}); err != nil {
		t.Fatal(err)
	}
	return rt.files
}

// Every condition has the control's home; the trap conditions add only the
// honeytoken files under ~/.aws and, at high realism, history lines appended
// to the same .bash_history.
func TestConditionsDifferOnlyByTheHoneytoken(t *testing.T) {
	control := plant(t, "control")
	hist := "/home/andrei.pop/.bash_history"
	if len(control) < 40 || control[hist] == "" || control["/home/andrei.pop/work/valtera-shipments-api/README.md"] == "" {
		t.Fatalf("control home is too thin: %d files", len(control))
	}
	for _, name := range []string{"ht-low-net-refused", "ht-low-net-faithful", "ht-high-net-refused", "ht-high-net-faithful"} {
		trap := plant(t, name)
		for p, content := range control {
			got, ok := trap[p]
			switch {
			case !ok:
				t.Errorf("%s: misses %s", name, p)
			case p == hist:
				if !strings.HasPrefix(got, content) {
					t.Errorf("%s: history does not start with the control history", name)
				}
				if strings.HasPrefix(name, "ht-low") && got != content {
					t.Errorf("%s: low honeytokens must not touch the history", name)
				}
			case got != content:
				t.Errorf("%s: %s differs from control", name, p)
			}
		}
		for p := range trap {
			if _, ok := control[p]; !ok && !strings.HasPrefix(p, "/home/andrei.pop/.aws/") {
				t.Errorf("%s: extra file %s outside ~/.aws", name, p)
			}
		}
	}
}

// The SSH key condition adds the key pair and appends to ~/.ssh/config,
// ~/.ssh/known_hosts and the shell history; everything else is the control.
func TestSSHKeyConditionDiffersOnlyByTheHoneytoken(t *testing.T) {
	control := plant(t, "control")
	trap := plantFile(t, filepath.Join("honeytokens", "ssh-private-key.xml"))
	h := "/home/andrei.pop"
	appended := map[string]bool{h + "/.ssh/config": true, h + "/.ssh/known_hosts": true, h + "/.bash_history": true}
	for p, content := range control {
		got, ok := trap[p]
		switch {
		case !ok:
			t.Errorf("misses %s", p)
		case appended[p]:
			if !strings.HasPrefix(got, content) || got == content {
				t.Errorf("%s must be the control content plus appended lines", p)
			}
		case got != content:
			t.Errorf("%s differs from control", p)
		}
	}
	for p := range trap {
		if _, ok := control[p]; !ok && p != h+"/.ssh/id_ed25519" && p != h+"/.ssh/id_ed25519.pub" {
			t.Errorf("extra file %s", p)
		}
	}
	if !strings.HasPrefix(trap[h+"/.ssh/id_ed25519"], "-----BEGIN OPENSSH PRIVATE KEY-----") {
		t.Error("key not planted")
	}
}
