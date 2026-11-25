package main

import (
	"context"
	"flag"
	"os/signal"
	"syscall"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/app"
)

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	var r runFlags
	r.register(fs)
	var o app.ServeOptions
	fs.StringVar(&o.LogDir, "log-dir", "/var/log/chamber", "where traps.jsonl and run.json are written")
	fs.StringVar(&o.BundleAddr, "bundle-addr", ":8080", "address the bundle is served on, once")
	_ = fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	p, err := app.Prepare(ctx, r.run())
	if err != nil {
		return err
	}
	return app.Serve(ctx, p, o)
}
