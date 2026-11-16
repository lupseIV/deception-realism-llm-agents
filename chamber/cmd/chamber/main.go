// Command chamber prepares and runs one detonation-chamber run.
//
//	chamber compose  writes the Docker Compose project of a run (on the host)
//	chamber serve    runs the master tree inside the gateway container
//	chamber facts    prints the facts of a run, for checking a configuration
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/assemble"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/config"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/layers"
)

const usage = `usage: chamber <command> [flags]

commands:
  compose   write the Docker Compose project of a run
  serve     run the chamber inside the gateway container
  facts     print the facts of a run

run "chamber <command> -h" for the flags of a command
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "compose":
		err = cmdCompose(os.Args[2:])
	case "serve":
		err = cmdServe(os.Args[2:])
	case "facts":
		err = cmdFacts(os.Args[2:])
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "chamber:", err)
		os.Exit(1)
	}
}

// runFlags are shared by every command.
type runFlags struct {
	config, runID, baseDir, backend string
	seed                            uint64
	seedSet                         bool
}

func (r *runFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&r.config, "config", "", "sandbox configuration, relative to -base-dir (required)")
	fs.StringVar(&r.runID, "run-id", "", "run id (required)")
	fs.Func("seed", "run seed (default: the seed in the configuration)", func(s string) error {
		_, err := fmt.Sscan(s, &r.seed)
		r.seedSet = err == nil
		return err
	})
	fs.StringVar(&r.baseDir, "base-dir", ".", "repository root")
	fs.StringVar(&r.backend, "backend", "http://backend:5000", "Moto URL as seen from the gateway")
}

// build loads the configuration, assembles the run and builds the tree.
func (r *runFlags) build(ctx context.Context) (config.Condition, assemble.Result, facts.Facts, error) {
	if r.config == "" || r.runID == "" {
		return config.Condition{}, assemble.Result{}, nil, errors.New("-config and -run-id are required")
	}
	c, err := config.Load(filepath.Join(r.baseDir, r.config), config.Options{BaseDir: r.baseDir})
	if err != nil {
		return c, assemble.Result{}, nil, err
	}
	run := assemble.Run{ID: r.runID}
	if r.seedSet {
		run.Seed = &r.seed
	}
	res, err := assemble.Build(c, run, layers.Factory(r.backend), r.baseDir)
	if err != nil {
		return c, res, nil, err
	}
	out, err := res.Root.Build(ctx, res.Facts)
	if err != nil {
		_ = res.Root.Stop(ctx)
		return c, res, nil, err
	}
	return c, res, res.Facts.Merge(out), nil
}

func cmdFacts(args []string) error {
	fs := flag.NewFlagSet("facts", flag.ExitOnError)
	var r runFlags
	r.register(fs)
	showSecrets := fs.Bool("secrets", false, "also print secret access keys")
	_ = fs.Parse(args)
	ctx := context.Background()
	_, res, f, err := r.build(ctx)
	if err != nil {
		return err
	}
	defer res.Root.Stop(ctx)
	for _, k := range f.Keys() {
		v := f[k]
		if strings.HasSuffix(k, "."+facts.TokenSecretAccessKey) && !*showSecrets {
			v = "(hidden, use -secrets)"
		}
		fmt.Printf("%s = %s\n", k, v)
	}
	return nil
}

// regionsAndBuckets collects the regions and buckets of all planted tokens.
func regionsAndBuckets(f facts.Facts) (regions, buckets []string) {
	seenR, seenB := map[string]bool{}, map[string]bool{}
	if r := f[facts.PersonaRegion]; r != "" {
		seenR[r] = true
	}
	for k, v := range f {
		if !strings.HasPrefix(k, "token.") || v == "" {
			continue
		}
		switch {
		case strings.HasSuffix(k, "."+facts.TokenRegion):
			seenR[v] = true
		case strings.HasSuffix(k, "."+facts.TokenBucket):
			seenB[v] = true
		}
	}
	for r := range seenR {
		regions = append(regions, r)
	}
	for b := range seenB {
		buckets = append(buckets, b)
	}
	sort.Strings(regions)
	sort.Strings(buckets)
	if len(regions) == 0 {
		regions = []string{"us-east-1"}
	}
	return regions, buckets
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}
