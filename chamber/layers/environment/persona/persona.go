// Package persona is the environment layer: it turns the persona into the
// sandbox user and machine.
//
//	every level  identity (hostname, user account, uid, home, login shell)
//	             and the baseline home: persona-shaped dotfiles, shell
//	             history, editor state and the project under ~/work, with
//	             past timestamps (see home.go). Control gets it too, so the
//	             conditions differ only by the honeytoken.
//	low          the baseline only
//	medium       plus the persona's other folders and apps (not implemented yet)
//	high         plus logs and longer aging (not implemented yet)
package persona

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/assemble"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/config"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/master"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/layers/environment/wear"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/traplog"
)

// Kind is the factory key of this layer.
const Kind = assemble.KindPersona

// UID is the uid of the sandbox user, the first regular user on Debian.
const UID = "1000"

// Layer is the environment layer of one sandbox.
type Layer struct {
	persona config.Persona
	now     func() time.Time
	files   []File
}

// Option changes a Layer.
type Option func(*Layer)

// WithClock sets the reference time for file timestamps (tests).
func WithClock(now func() time.Time) Option { return func(l *Layer) { l.now = now } }

// New is the factory constructor.
func New(spec assemble.LayerSpec, opts ...Option) (master.Layer, error) {
	if spec.Persona.Identity.User == "" {
		return nil, fmt.Errorf("persona %q has no user", spec.Persona.ID)
	}
	l := &Layer{persona: spec.Persona, now: time.Now}
	for _, o := range opts {
		o(l)
	}
	return l, nil
}

// Files returns the files built by Build (for tests and inspection).
func (l *Layer) Files() []File { return l.files }

func (l *Layer) Kind() string { return Kind }

// Build exports the user and machine facts for the runtime and builds the
// baseline home.
func (l *Layer) Build(_ context.Context, in facts.Facts) (facts.Facts, error) {
	switch level := in[master.RealismKey]; level {
	case string(config.EnvRealismLevelLow):
	case string(config.EnvRealismLevelMedium), string(config.EnvRealismLevelHigh):
		return nil, fmt.Errorf("environment realism %q is not implemented yet", level)
	default:
		return nil, fmt.Errorf("unknown environment realism %q", level)
	}
	id := l.persona.Identity
	home := "/home/" + id.User
	w, err := wear.Open(in[master.RealismKey])
	if err != nil {
		return nil, err
	}
	g := newHomeGen(l.persona, l.now())
	if l.files, err = baselineFiles(newHomeCtx(l.persona, home), g, w); err != nil {
		return nil, err
	}
	return facts.Facts{
		facts.EnvUser:     id.User,
		facts.EnvUID:      UID,
		facts.EnvHome:     home,
		facts.EnvShell:    shell(l.persona),
		facts.EnvHostname: string(id.Hostname),
	}, nil
}

// shell is the login shell named in the persona's history, bash by default.
func shell(p config.Persona) string {
	if p.History != nil && p.History.Shell != "" {
		return "/bin/" + strings.TrimPrefix(string(p.History.Shell), "/bin/")
	}
	return "/bin/bash"
}

// Start plants the home files; the runtime creates the account itself.
func (l *Layer) Start(ctx context.Context, rt master.Runtime, _ func(traplog.Event)) error {
	for _, f := range l.files {
		if err := rt.WriteFile(ctx, f.Path, []byte(f.Content), f.ModTime); err != nil {
			return fmt.Errorf("write %s: %w", f.Path, err)
		}
	}
	return nil
}

func (l *Layer) Stop(context.Context) error { return nil }
