// Package persona is the environment layer: it turns the persona into the
// sandbox user and machine.
//
//	low     identity only: hostname, user account, uid, home and login shell.
//	        The runtime creates the account with the distribution's own home
//	        skeleton (useradd -m), so no files are written here.
//	medium  files, apps and config of the persona (not implemented yet)
//	high    plus history, logs and aging (not implemented yet)
package persona

import (
	"context"
	"fmt"
	"strings"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/assemble"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/config"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/master"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/traplog"
)

// Kind is the factory key of this layer.
const Kind = assemble.KindPersona

// UID is the uid of the sandbox user, the first regular user on Debian.
const UID = "1000"

// Layer is the environment layer of one sandbox.
type Layer struct {
	persona config.Persona
}

// New is the factory constructor.
func New(spec assemble.LayerSpec) (master.Layer, error) {
	if spec.Persona.Identity.User == "" {
		return nil, fmt.Errorf("persona %q has no user", spec.Persona.ID)
	}
	return &Layer{persona: spec.Persona}, nil
}

func (l *Layer) Kind() string { return Kind }

// Build exports the user and machine facts for the runtime.
func (l *Layer) Build(_ context.Context, in facts.Facts) (facts.Facts, error) {
	switch level := in[master.RealismKey]; level {
	case string(config.EnvRealismLevelLow):
	case string(config.EnvRealismLevelMedium), string(config.EnvRealismLevelHigh):
		return nil, fmt.Errorf("environment realism %q is not implemented yet", level)
	default:
		return nil, fmt.Errorf("unknown environment realism %q", level)
	}
	id := l.persona.Identity
	return facts.Facts{
		facts.EnvUser:     id.User,
		facts.EnvUID:      UID,
		facts.EnvHome:     "/home/" + id.User,
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

// Start writes nothing at low realism; the runtime has created the account.
func (l *Layer) Start(context.Context, master.Runtime, func(traplog.Event)) error { return nil }

func (l *Layer) Stop(context.Context) error { return nil }
