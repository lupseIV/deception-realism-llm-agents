package tlsca

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

func TestLeafVerifiesForAWSNames(t *testing.T) {
	ca, err := NewCA("Valtera Logistics", now)
	if err != nil {
		t.Fatal(err)
	}
	if ca.Cert.Subject.CommonName != "Valtera Logistics Internal Root CA" || !ca.Cert.NotBefore.Before(now.AddDate(-2, 0, 0)) {
		t.Errorf("CA: %v %v", ca.Cert.Subject, ca.Cert.NotBefore)
	}
	leaf, err := ca.Leaf(CertNames("eu-central-1", []string{"valtera-backups"}), now)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(leaf.Certificate[0])
	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	for _, h := range []string{
		"sts.amazonaws.com", "sts.eu-central-1.amazonaws.com", "s3.eu-central-1.amazonaws.com",
		"valtera-backups.s3.eu-central-1.amazonaws.com", "secretsmanager.eu-central-1.amazonaws.com",
		"other-bucket.s3.eu-central-1.amazonaws.com", "ec2.eu-central-1.amazonaws.com",
	} {
		if _, err := cert.Verify(x509.VerifyOptions{DNSName: h, Roots: pool, CurrentTime: now}); err != nil {
			t.Errorf("%s: %v", h, err)
		}
	}
	if _, err := cert.Verify(x509.VerifyOptions{DNSName: "example.com", Roots: pool, CurrentTime: now}); err == nil {
		t.Error("certificate must not cover non-AWS names")
	}
}

func TestHTTPSWithTheCA(t *testing.T) {
	ca, _ := NewCA("Valtera Logistics", time.Now())
	leaf, _ := ca.Leaf(CertNames("eu-central-1", nil), time.Now())
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{leaf}})
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") })}
	go srv.Serve(ln)
	defer srv.Close()

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(ca.PEM)
	addr := ln.Addr().String()
	cfg := &tls.Config{RootCAs: pool, ServerName: "sts.amazonaws.com"}
	conn, err := tls.Dial("tcp", addr, cfg)
	if err != nil {
		t.Fatalf("TLS handshake as sts.amazonaws.com: %v", err)
	}
	io.WriteString(conn, "GET / HTTP/1.0\r\nHost: sts.amazonaws.com\r\n\r\n")
	b, _ := io.ReadAll(conn)
	if !strings.HasSuffix(string(b), "ok") {
		t.Fatalf("response: %q", b)
	}
}
