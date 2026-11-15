// Package assemble builds the master tree of one run from a checked sandbox
// configuration: run facts, the token-source registry, and one layer per
// configured element, created through a factory so that this package never
// imports a layer implementation.
package assemble

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/config"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/master"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/tokensource"
)

// Layer kinds, as used as keys of a Factory.
const (
	KindPersona = "environment/persona"
)

// HoneytokenKind returns the layer kind for a token type, e.g. "honeytoken/aws-credentials".
func HoneytokenKind(t config.TokenType) string { return "honeytoken/" + string(t) }

// NetworkKind returns the layer kind for a service type, e.g. "network/aws".
func NetworkKind(t config.NetworkServiceType) string { return "network/" + string(t) }

// Master and group names; they form the event paths, e.g. sandbox/honeytoken/creds.
const (
	RootName        = "sandbox"
	EnvGroup        = "environment"
	HoneytokenGroup = "honeytoken"
	NetworkGroup    = "network"
	PersonaLayer    = "persona"
)

// Run holds the parameters of one run.
type Run struct {
	ID   string  // required
	Seed *uint64 // optional; falls back to the sandbox seed attribute
}

// LayerSpec is everything a layer constructor gets.
type LayerSpec struct {
	Kind    string
	ID      string // token or service id; PersonaLayer for the environment layer
	Realism string // the realism level of the layer's group
	Source  string // honeytoken layers: token source id
	Bind    string // network layers: bound token id, may be empty
	Persona config.Persona
	Tokens  *tokensource.Registry
}

// Factory maps a layer kind to its constructor.
type Factory map[string]func(LayerSpec) (master.Layer, error)

func (f Factory) create(spec LayerSpec) (master.Layer, error) {
	ctor, ok := f[spec.Kind]
	if !ok {
		known := make([]string, 0, len(f))
		for k := range f {
			known = append(known, k)
		}
		sort.Strings(known)
		return nil, fmt.Errorf("no layer for kind %q (known: %v)", spec.Kind, known)
	}
	l, err := ctor(spec)
	if err != nil {
		return nil, fmt.Errorf("%s %q: %w", spec.Kind, spec.ID, err)
	}
	return l, nil
}

// Result is an assembled run, ready for Build, Start and Stop on Root.
type Result struct {
	Root   *master.GroupMaster
	Facts  facts.Facts // starting facts for Root.Build
	Tokens *tokensource.Registry
}

// Build assembles the master tree for one run of the condition c. baseDir is
// the repository root, used to resolve pool files ("" means ".").
func Build(c config.Condition, run Run, f Factory, baseDir string) (Result, error) {
	if run.ID == "" {
		return Result{}, errors.New("run id is required")
	}
	seed := run.Seed
	if seed == nil {
		seed = c.Sandbox.Seed
	}
	if seed == nil {
		return Result{}, fmt.Errorf("%s: no seed given and the configuration has none", c.Path)
	}

	tokens, err := registry(c.Sandbox, baseDir)
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", c.Path, err)
	}

	// Create every layer before any group: groups start goroutines, so an
	// error must not leave a half-built tree behind.
	s, p := c.Sandbox, c.Persona
	env, err := f.create(LayerSpec{Kind: KindPersona, ID: PersonaLayer,
		Realism: string(s.Environment.RealismLevel), Persona: p, Tokens: tokens})
	if err != nil {
		return Result{}, err
	}
	var htLayers, netLayers []master.Master
	if s.HoneyTokens != nil {
		for _, t := range s.HoneyTokens.Tokens {
			l, err := f.create(LayerSpec{Kind: HoneytokenKind(t.Type), ID: string(t.ID),
				Realism: string(s.HoneyTokens.Realism), Source: string(t.Source), Persona: p, Tokens: tokens})
			if err != nil {
				return Result{}, err
			}
			htLayers = append(htLayers, master.NewLayer(string(t.ID), l))
		}
	}
	if s.Network != nil {
		for _, svc := range s.Network.Services {
			l, err := f.create(LayerSpec{Kind: NetworkKind(svc.NetworkServiceType), ID: string(svc.ID),
				Realism: string(s.Network.RealismLevel), Bind: string(svc.Bind), Persona: p, Tokens: tokens})
			if err != nil {
				return Result{}, err
			}
			netLayers = append(netLayers, master.NewLayer(string(svc.ID), l))
		}
	}

	groups := []master.Master{
		master.NewGroup(EnvGroup, string(s.Environment.RealismLevel), master.NewLayer(PersonaLayer, env)),
	}
	if len(htLayers) > 0 {
		groups = append(groups, master.NewGroup(HoneytokenGroup, string(s.HoneyTokens.Realism), htLayers...))
	}
	if len(netLayers) > 0 {
		groups = append(groups, master.NewGroup(NetworkGroup, string(s.Network.RealismLevel), netLayers...))
	}

	return Result{
		Root:   master.NewGroup(RootName, "", groups...),
		Facts:  startFacts(run.ID, *seed, p),
		Tokens: tokens,
	}, nil
}

// startFacts are the facts every layer can read.
func startFacts(runID string, seed uint64, p config.Persona) facts.Facts {
	id := p.Identity
	f := facts.Facts{
		facts.RunID:              runID,
		facts.RunSeed:            strconv.FormatUint(seed, 10),
		facts.PersonaID:          string(p.ID),
		facts.PersonaUser:        id.User,
		facts.PersonaFullName:    id.FullName,
		facts.PersonaHostname:    string(id.Hostname),
		facts.PersonaCompany:     id.Company,
		facts.PersonaCompanySlug: string(id.CompanySlug),
		facts.PersonaRole:        id.Role,
		facts.PersonaRegion:      id.Region,
		facts.PersonaHome:        "/home/" + id.User,
	}
	for _, tp := range p.TokenAffinity {
		f[facts.TokenPathKey(string(tp.Type))] = tp.Path
	}
	return f
}

// registry registers one token source per <source> element.
func registry(s config.Sandbox, baseDir string) (*tokensource.Registry, error) {
	if baseDir == "" {
		baseDir = "."
	}
	r := tokensource.NewRegistry()
	for _, src := range s.Sources {
		var ts tokensource.TokenSource
		switch src.Type {
		case config.SourceTypeGenerator:
			ts = tokensource.Generator{}
		case config.SourceTypePool:
			path := src.File
			if !filepath.IsAbs(path) {
				path = filepath.Join(baseDir, path)
			}
			pf, err := tokensource.LoadPool(path)
			if err != nil {
				return nil, fmt.Errorf("sources/source %q: %w", src.ID, err)
			}
			ts = tokensource.NewPool(pf)
		default:
			return nil, fmt.Errorf("sources/source %q: unknown type %q", src.ID, src.Type)
		}
		if err := r.Register(string(src.ID), ts); err != nil {
			return nil, err
		}
	}
	return r, nil
}
