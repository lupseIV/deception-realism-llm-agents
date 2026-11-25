package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/app"
)

func cmdCompose(args []string) error {
	fs := flag.NewFlagSet("compose", flag.ExitOnError)
	var r runFlags
	r.register(fs)
	var o app.ComposeOptions
	fs.StringVar(&o.Out, "out", "", "output directory (default: <base-dir>/runs/<run-id>)")
	fs.StringVar(&o.Runtime, "runtime", "", "container runtime; overrides the configuration (use runc on Docker Desktop)")
	fs.StringVar(&o.GatewayImage, "gateway-image", "chamber/gateway:0.1", "gateway image")
	fs.StringVar(&o.MotoImage, "moto-image", "motoserver/moto:latest", "Moto image")
	_ = fs.Parse(args)

	p, err := app.Prepare(context.Background(), r.run())
	if err != nil {
		return err
	}
	path, err := app.Compose(p, o)
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}
