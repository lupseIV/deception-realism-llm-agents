// Package app holds the use cases of the chamber CLI: prepare a run, write
// its compose project, serve it. cmd/chamber only parses flags and prints.
package app

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/assemble"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/config"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/master"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/layers"
)

// Run names one run of one sandbox configuration.
type Run struct {
	Config  string  // sandbox configuration, relative to BaseDir
	ID      string  // run id
	BaseDir string  // repository root
	Backend string  // Moto URL as seen from the gateway
	Seed    *uint64 // overrides the seed in the configuration
}

// Prepared is a run whose tree is built and ready to Start.
type Prepared struct {
	Run   Run
	Cond  config.Condition
	Root  *master.GroupMaster
	Facts facts.Facts // start facts plus every fact the layers exported
}

// Prepare loads the configuration, assembles the tree and builds it.
func Prepare(ctx context.Context, r Run) (Prepared, error) {
	if r.Config == "" || r.ID == "" {
		return Prepared{}, errors.New("-config and -run-id are required")
	}
	c, err := config.Load(filepath.Join(r.BaseDir, r.Config), config.Options{BaseDir: r.BaseDir})
	if err != nil {
		return Prepared{}, err
	}
	res, err := assemble.Build(c, assemble.Run{ID: r.ID, Seed: r.Seed}, layers.Factory(r.Backend), r.BaseDir)
	if err != nil {
		return Prepared{}, err
	}
	out, err := res.Root.Build(ctx, res.Facts)
	if err != nil {
		return Prepared{}, err
	}
	return Prepared{Run: r, Cond: c, Root: res.Root, Facts: res.Facts.Merge(out)}, nil
}
