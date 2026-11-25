// Command chamber prepares and runs one detonation-chamber run.
//
//	chamber compose  writes the Docker Compose project of a run (on the host)
//	chamber serve    runs the master tree inside the gateway container
//	chamber facts    prints the facts of a run, for checking a configuration
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/app"
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

// run returns the run the flags name.
func (r *runFlags) run() app.Run {
	out := app.Run{Config: r.config, ID: r.runID, BaseDir: r.baseDir, Backend: r.backend}
	if r.seedSet {
		out.Seed = &r.seed
	}
	return out
}
