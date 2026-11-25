// Package master implements the layered master tree (Composite pattern).
// Every node is a Master: a LayerMaster wraps one Layer (Strategy pattern), a
// GroupMaster holds child masters, and the sandbox master is the root group.
//
// Two things cross the tree, in opposite directions:
//
//   - down: files to plant. A layer describes them as data (File); its
//     LayerMaster plants them through the Runtime on Start.
//   - up: trap events. Start hands every child an emit function that adds
//     the child's name and calls the parent's emit, so the root's emit (the
//     trap log) sees paths such as "sandbox/honeytoken/aws-credentials".
//     There are no channels or goroutines: emit is a plain call.
package master

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/traplog"
)

// RealismKey is the fact through which a group passes its realism level to
// its children.
const RealismKey = "realism"

// Emit receives one trap event. It must be safe for concurrent use.
type Emit = func(traplog.Event)

// Runtime is what masters use to act on the sandbox. Implemented by the
// container runtime; tests use a fake.
type Runtime interface {
	// WriteFile plants a file, replacing an earlier one at the same path.
	WriteFile(ctx context.Context, path string, data []byte, mtime time.Time) error
	// AppendFile adds data to the end of a planted file (or creates it) and
	// sets its mtime. Layers use it to add to a file another layer owns, e.g.
	// honeytoken lines on top of the environment's shell history.
	AppendFile(ctx context.Context, path string, data []byte, mtime time.Time) error
}

// File is one file a layer plants. A Path ending in "/" is an empty
// directory. Append adds Content to a file an earlier layer planted.
type File struct {
	Path    string // absolute
	Content string
	ModTime time.Time
	Append  bool
}

// Plant writes files through rt in order.
func Plant(ctx context.Context, rt Runtime, files []File) error {
	for _, f := range files {
		write := rt.WriteFile
		if f.Append {
			write = rt.AppendFile
		}
		if err := write(ctx, f.Path, []byte(f.Content), f.ModTime); err != nil {
			return fmt.Errorf("write %s: %w", f.Path, err)
		}
	}
	return nil
}

// Master is the common interface of every node in the tree.
type Master interface {
	Name() string
	Build(ctx context.Context, in facts.Facts) (facts.Facts, error)
	Start(ctx context.Context, rt Runtime, emit Emit) error
	Stop(ctx context.Context) error
}

// Layer is one kind of deception or realism content. Build computes it from
// the facts. A layer that plants files also implements Planter; a layer
// that runs something (a fake service) also implements Service.
type Layer interface {
	Kind() string
	Build(ctx context.Context, in facts.Facts) (facts.Facts, error)
}

// Planter is a layer with files to plant; Files is valid after Build.
type Planter interface {
	Files() []File
}

// Service is a layer that runs between Start and Stop. emit is valid until
// Stop returns.
type Service interface {
	Start(ctx context.Context, rt Runtime, emit Emit) error
	Stop(ctx context.Context) error
}

// --- leaf -----------------------------------------------------------------

// LayerMaster is a leaf master wrapping one Layer.
type LayerMaster struct {
	name  string
	layer Layer
}

// NewLayer returns a leaf master named name.
func NewLayer(name string, l Layer) *LayerMaster { return &LayerMaster{name: name, layer: l} }

func (m *LayerMaster) Name() string { return m.name }

func (m *LayerMaster) Build(ctx context.Context, in facts.Facts) (facts.Facts, error) {
	out, err := m.layer.Build(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", m.name, err)
	}
	return out, nil
}

// Start plants the layer's files, then starts it if it is a Service.
func (m *LayerMaster) Start(ctx context.Context, rt Runtime, emit Emit) error {
	if p, ok := m.layer.(Planter); ok {
		if err := Plant(ctx, rt, p.Files()); err != nil {
			return fmt.Errorf("%s: %w", m.name, err)
		}
	}
	if s, ok := m.layer.(Service); ok {
		if err := s.Start(ctx, rt, func(e traplog.Event) { e.Layer = m.name; emit(e) }); err != nil {
			return fmt.Errorf("%s: %w", m.name, err)
		}
	}
	return nil
}

// Stop stops the layer if it is a Service.
func (m *LayerMaster) Stop(ctx context.Context) error {
	s, ok := m.layer.(Service)
	if !ok {
		return nil
	}
	if err := s.Stop(ctx); err != nil {
		return fmt.Errorf("%s: %w", m.name, err)
	}
	return nil
}

// --- composite --------------------------------------------------------------

// GroupMaster holds child masters, in dependency order.
type GroupMaster struct {
	name     string
	realism  string
	children []Master
}

// NewGroup returns a group master. If realism is not empty, it is passed to
// every child as the RealismKey fact. Children are built and started in the
// given order and stopped in reverse order.
func NewGroup(name, realism string, children ...Master) *GroupMaster {
	return &GroupMaster{name: name, realism: realism, children: children}
}

func (g *GroupMaster) Name() string       { return g.name }
func (g *GroupMaster) Children() []Master { return g.children }

// Build builds the children in order. Each child sees the input facts plus
// the facts exported by the children before it. The group returns all facts
// exported by its children.
func (g *GroupMaster) Build(ctx context.Context, in facts.Facts) (facts.Facts, error) {
	exported := facts.Facts{}
	for _, c := range g.children {
		childIn := in.Merge(exported)
		if g.realism != "" {
			childIn = childIn.Merge(facts.Facts{RealismKey: g.realism})
		}
		out, err := c.Build(ctx, childIn)
		if err != nil {
			return nil, fmt.Errorf("%s/%w", g.name, err)
		}
		exported = exported.Merge(out)
	}
	return exported, nil
}

// Start starts the children in order and stops at the first error. Events
// of the children reach emit with this group's name in front of their path.
func (g *GroupMaster) Start(ctx context.Context, rt Runtime, emit Emit) error {
	up := func(e traplog.Event) { e.Layer = g.name + "/" + e.Layer; emit(e) }
	for _, c := range g.children {
		if err := c.Start(ctx, rt, up); err != nil {
			return fmt.Errorf("%s/%w", g.name, err)
		}
	}
	return nil
}

// Stop stops all children in reverse order and returns all errors.
func (g *GroupMaster) Stop(ctx context.Context) error {
	var errs []error
	for i := len(g.children) - 1; i >= 0; i-- {
		if err := g.children[i].Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s/%w", g.name, err))
		}
	}
	return errors.Join(errs...)
}
