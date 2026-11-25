package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/gateway"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/tlsca"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/traplog"
)

// ServeOptions are the gateway-side choices of a run.
type ServeOptions struct {
	LogDir     string // where traps.jsonl and run.json are written
	BundleAddr string // address the setup bundle is served on, once
}

// Serve runs a prepared run inside the gateway container until ctx ends:
// plant the files, serve the setup bundle, log trap events, stop.
//
// It returns an error if any trap event could not be written, and then also
// records "lost_events" in run.json: such a run's trap log is incomplete and
// must not be counted as a run without triggers.
func Serve(ctx context.Context, p Prepared, o ServeOptions) error {
	r, f := p.Run, p.Facts
	if err := os.MkdirAll(o.LogDir, 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(o.LogDir, "traps.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	traps := traplog.New(logFile, r.ID)
	emit := func(e traplog.Event) {
		if err := traps.Log(e); err != nil {
			log.Printf("trap event lost: %v", err)
		}
	}

	// TLS for the AWS names, signed by a CA named after the persona's company.
	now := time.Now()
	ca, err := tlsca.NewCA(p.Cond.Persona.Identity.Company, now)
	if err != nil {
		return err
	}
	regions, buckets := RegionsAndBuckets(f)
	var names []string
	for _, region := range regions {
		names = append(names, tlsca.CertNames(region, buckets)...)
	}
	leaf, err := ca.Leaf(names, now)
	if err != nil {
		return err
	}
	rt := gateway.New(&tls.Config{Certificates: []tls.Certificate{leaf}, MinVersion: tls.VersionTLS12})

	if NeedsBackend(f) {
		if err := waitFor(ctx, r.Backend+"/moto-api/", 90*time.Second); err != nil {
			return fmt.Errorf("Moto not reachable: %w", err)
		}
	}

	runJSON := filepath.Join(o.LogDir, "run.json")
	info := map[string]any{"run_id": r.ID, "config": r.Config, "facts": f}
	// stop stops the tree and reports lost trap events. Every return after
	// Start goes through it.
	stop := func(err error) error {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err = errors.Join(err, p.Root.Stop(stopCtx))
		if n, first := traps.Lost(); n > 0 {
			info["lost_events"] = n
			if werr := writeJSON(runJSON, info); werr != nil {
				log.Printf("write run.json: %v", werr)
			}
			err = errors.Join(err, fmt.Errorf("%d trap events lost, the trap log of this run is incomplete: %w", n, first))
		}
		return err
	}

	if err := p.Root.Start(ctx, rt, emit); err != nil {
		return stop(err)
	}
	log.Printf("run %s started: %d planted files", r.ID, len(rt.Paths()))
	info["planted"] = rt.Paths()
	if err := writeJSON(runJSON, info); err != nil {
		log.Printf("write run.json: %v", err)
	}

	bundle, err := rt.Bundle(gateway.Setup{
		User: f[facts.EnvUser], UID: f[facts.EnvUID], Home: f[facts.EnvHome], Shell: f[facts.EnvShell],
		CAName: f[facts.PersonaCompanySlug] + "-root-ca.crt", CAPEM: ca.PEM,
	})
	if err != nil {
		return stop(err)
	}
	if err := gateway.ServeBundleOnce(ctx, o.BundleAddr, bundle); err == nil {
		log.Printf("bundle delivered")
	}

	<-ctx.Done()
	log.Printf("stopping")
	return stop(nil)
}

// waitFor polls url until it answers or the timeout passes.
func waitFor(ctx context.Context, url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return err
		}
		time.Sleep(time.Second)
	}
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}
