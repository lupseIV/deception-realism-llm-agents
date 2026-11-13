// Package master implements the layered master tree (Composite pattern).
// Every node is a Master: a LayerMaster wraps one Layer (Strategy pattern), a
// GroupMaster holds child masters, and the sandbox master is the root group.
// Events flow up the tree; each level prepends its name, so the root sees
// paths such as "sandbox/honeytoken/aws-credentials".
package master

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/traplog"
)

// RealismKey is the fact through which a group passes its realism level to
// its children.
const RealismKey = "realism"

// Runtime is what masters use to act on the sandbox. Implemented by the
// container runtime; tests use a fake.
type Runtime interface {
	WriteFile(ctx context.Context, path string, data []byte, mtime time.Time) error
}

// Master is the common interface of every node in the tree.
type Master interface {
	Name() string
	Build(ctx context.Context, in facts.Facts) (facts.Facts, error)
	Start(ctx context.Context, rt Runtime) error
	Events() <-chan traplog.Event
	Stop(ctx context.Context) error
}

// Layer is one kind of deception or realism content. emit sends an event to
// the owning LayerMaster; it is valid between Start and Stop.
type Layer interface {
	Kind() string
	Build(ctx context.Context, in facts.Facts) (facts.Facts, error)
	Start(ctx context.Context, rt Runtime, emit func(traplog.Event)) error
	Stop(ctx context.Context) error
}

// --- leaf -----------------------------------------------------------------

// LayerMaster is a leaf master wrapping one Layer.
type LayerMaster struct {
	name   string
	layer  Layer
	events chan traplog.Event

	mu     sync.Mutex
	closed bool
}

// NewLayer returns a leaf master named name.
func NewLayer(name string, l Layer) *LayerMaster {
	return &LayerMaster{name: name, layer: l, events: make(chan traplog.Event, 256)}
}

func (m *LayerMaster) Name() string                 { return m.name }
func (m *LayerMaster) Events() <-chan traplog.Event { return m.events }

func (m *LayerMaster) Build(ctx context.Context, in facts.Facts) (facts.Facts, error) {
	out, err := m.layer.Build(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", m.name, err)
	}
	return out, nil
}

func (m *LayerMaster) Start(ctx context.Context, rt Runtime) error {
	if err := m.layer.Start(ctx, rt, m.emit); err != nil {
		return fmt.Errorf("%s: %w", m.name, err)
	}
	return nil
}

func (m *LayerMaster) emit(e traplog.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	e.Layer = m.name
	m.events <- e
}

// Stop stops the layer and closes the event channel. Safe to call twice.
func (m *LayerMaster) Stop(ctx context.Context) error {
	err := m.layer.Stop(ctx)
	m.mu.Lock()
	if !m.closed {
		m.closed = true
		close(m.events)
	}
	m.mu.Unlock()
	if err != nil {
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
	events   chan traplog.Event
}

// NewGroup returns a group master. If realism is not empty, it is passed to
// every child as the RealismKey fact. Children are built and started in the
// given order and stopped in reverse order.
func NewGroup(name, realism string, children ...Master) *GroupMaster {
	g := &GroupMaster{name: name, realism: realism, children: children,
		events: make(chan traplog.Event, 256)}
	var wg sync.WaitGroup
	for _, c := range children {
		wg.Add(1)
		go func(c Master) {
			defer wg.Done()
			for e := range c.Events() {
				e.Layer = g.name + "/" + e.Layer
				g.events <- e
			}
		}(c)
	}
	go func() { wg.Wait(); close(g.events) }()
	return g
}

func (g *GroupMaster) Name() string                 { return g.name }
func (g *GroupMaster) Events() <-chan traplog.Event { return g.events }
func (g *GroupMaster) Children() []Master           { return g.children }

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

// Start starts the children in order and stops at the first error.
func (g *GroupMaster) Start(ctx context.Context, rt Runtime) error {
	for _, c := range g.children {
		if err := c.Start(ctx, rt); err != nil {
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

// --- root -------------------------------------------------------------------

// Drain writes every event from m to the trap logger until m's event stream
// closes (after Stop). The returned channel closes when draining is done.
func Drain(m Master, log *traplog.Logger) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for e := range m.Events() {
			_ = log.Log(e)
		}
	}()
	return done
}
