package gateway

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/tlsca"
)

var ctx = context.Background()

func setup() Setup {
	return Setup{User: "andrei.pop", UID: "1000", Home: "/home/andrei.pop", Shell: "/bin/bash",
		CAName: "valtera-root-ca.crt", CAPEM: []byte("-----BEGIN CERTIFICATE-----\n")}
}

func TestBundleLayout(t *testing.T) {
	r := New(nil)
	mt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	if err := r.WriteFile(ctx, "/home/andrei.pop/.aws/credentials", []byte("[default]\n"), mt); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteFile(ctx, "/etc/motd", []byte("hi\n"), mt); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteFile(ctx, "relative", nil, mt); err == nil {
		t.Error("relative path must fail")
	}
	b, err := r.Bundle(setup())
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(bytes.NewReader(b))
	got := map[string]*tar.Header{}
	var script string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got[h.Name] = h
		if strings.HasSuffix(h.Name, "setup.sh") {
			s, _ := io.ReadAll(tr)
			script = string(s)
		}
	}
	creds := got["tmp/.chamber/root/home/andrei.pop/.aws/credentials"]
	if creds == nil || creds.Uid != 1000 || creds.Mode != 0o600 || !creds.ModTime.Equal(mt) {
		t.Errorf("credentials header: %+v", creds)
	}
	if motd := got["tmp/.chamber/root/etc/motd"]; motd == nil || motd.Uid != 0 {
		t.Errorf("motd header: %+v", motd)
	}
	if got["tmp/.chamber/root/usr/local/share/ca-certificates/valtera-root-ca.crt"] == nil {
		t.Error("CA missing")
	}
	for _, want := range []string{"useradd -m -u 1000 -s /bin/bash -d /home/andrei.pop andrei.pop", "cp -a /tmp/.chamber/root/. /", "update-ca-certificates", "chown -R andrei.pop:andrei.pop /home/andrei.pop"} {
		if !strings.Contains(script, want) {
			t.Errorf("setup script misses %q:\n%s", want, script)
		}
	}
	// useradd must run before the files are copied, so the home skeleton is created.
	if strings.Index(script, "useradd") > strings.Index(script, "cp -a") {
		t.Error("useradd must come before copying files")
	}
}

func TestServeBundleOnce(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()
	done := make(chan error, 1)
	go func() { done <- ServeBundleOnce(ctx, addr, []byte("bundle")) }()

	var body []byte
	for i := 0; i < 50; i++ {
		resp, err := http.Get("http://" + addr + "/bundle.tar")
		if err == nil {
			body, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if string(body) != "bundle" {
		t.Fatalf("body %q", body)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop after the first download")
	}
	if _, err := http.Get("http://" + addr + "/bundle.tar"); err == nil {
		t.Error("bundle must not be served twice")
	}
}

func TestListenOnlyOnce(t *testing.T) {
	ca, _ := tlsca.NewCA("Valtera Logistics", time.Now())
	leaf, _ := ca.Leaf(tlsca.CertNames("eu-central-1", nil), time.Now())
	r := New(&tls.Config{Certificates: []tls.Certificate{leaf}})
	r.ListenAddr = "127.0.0.1:0"
	ln, err := r.Listen(ctx, "aws")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if _, err := r.Listen(ctx, "aws2"); err == nil || !strings.Contains(err.Error(), "already taken") {
		t.Fatalf("want error for a second service, got %v", err)
	}
}
