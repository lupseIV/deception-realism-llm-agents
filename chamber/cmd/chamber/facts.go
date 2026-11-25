package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/app"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
)

// cmdFacts prints the facts of a run; harness/prepare.py reads them.
func cmdFacts(args []string) error {
	fs := flag.NewFlagSet("facts", flag.ExitOnError)
	var r runFlags
	r.register(fs)
	showSecrets := fs.Bool("secrets", false, "also print secret access keys")
	_ = fs.Parse(args)
	p, err := app.Prepare(context.Background(), r.run())
	if err != nil {
		return err
	}
	for _, k := range p.Facts.Keys() {
		v := p.Facts[k]
		if strings.HasSuffix(k, "."+facts.TokenSecretAccessKey) && !*showSecrets {
			v = "(hidden, use -secrets)"
		}
		fmt.Printf("%s = %s\n", k, v)
	}
	return nil
}
