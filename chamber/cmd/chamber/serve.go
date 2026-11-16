package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/gateway"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/master"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/tlsca"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/traplog"
)

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	var r runFlags
	r.register(fs)
	logDir := fs.String("log-dir", "/var/log/chamber", "where traps.jsonl and run.json are written")
	bundleAddr := fs.String("bundle-addr", ":8080", "address the bundle is served on, once")
	_ = fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	c, res, f, err := r.build(ctx)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*logDir, 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(*logDir, "traps.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	drained := master.Drain(res.Root, traplog.New(logFile, r.runID))

	// TLS for the AWS names, signed by a CA named after the persona's company.
	now := time.Now()
	ca, err := tlsca.NewCA(c.Persona.Identity.Company, now)
	if err != nil {
		return err
	}
	regions, buckets := regionsAndBuckets(f)
	var names []string
	for _, region := range regions {
		names = append(names, tlsca.CertNames(region, buckets)...)
	}
	leaf, err := ca.Leaf(names, now)
	if err != nil {
		return err
	}
	rt := gateway.New(&tls.Config{Certificates: []tls.Certificate{leaf}, MinVersion: tls.VersionTLS12})

	if needsBackend(f) {
		if err := waitFor(ctx, r.backend+"/moto-api/", 90*time.Second); err != nil {
			return fmt.Errorf("Moto not reachable: %w", err)
		}
	}
	if err := res.Root.Start(ctx, rt); err != nil {
		_ = res.Root.Stop(context.Background())
		return err
	}
	log.Printf("run %s started: %d planted files", r.runID, len(rt.Paths()))
	if err := writeJSON(filepath.Join(*logDir, "run.json"), map[string]any{
		"run_id": r.runID, "config": r.config, "facts": f, "planted": rt.Paths(),
	}); err != nil {
		log.Printf("write run.json: %v", err)
	}

	bundle, err := rt.Bundle(gateway.Setup{
		User: f[facts.EnvUser], UID: f[facts.EnvUID], Home: f[facts.EnvHome], Shell: f[facts.EnvShell],
		CAName: f[facts.PersonaCompanySlug] + "-root-ca.crt", CAPEM: ca.PEM,
	})
	if err != nil {
		return err
	}
	if err := gateway.ServeBundleOnce(ctx, *bundleAddr, bundle); err == nil {
		log.Printf("bundle delivered")
	}

	<-ctx.Done()
	log.Printf("stopping")
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = res.Root.Stop(stopCtx)
	<-drained
	return err
}

// needsBackend reports whether a network layer runs in faithful mode.
func needsBackend(f facts.Facts) bool {
	for k, v := range f {
		if strings.HasPrefix(k, "net.") && strings.HasSuffix(k, "."+facts.NetMode) && v == "faithful" {
			return true
		}
	}
	return false
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
