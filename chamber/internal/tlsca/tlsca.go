// Package tlsca creates the certificate authority and server certificate the
// chamber uses to answer HTTPS requests for AWS host names. The CA is
// installed in the sandbox's trust store and named after the persona's
// company, like a corporate TLS-inspection root.
package tlsca

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

// CA is a root certificate with its key.
type CA struct {
	Cert *x509.Certificate
	Key  *ecdsa.PrivateKey
	PEM  []byte // certificate in PEM, for the sandbox trust store
}

func serial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
}

// NewCA creates a root CA for an organisation. It looks issued about two
// years before now, like a long-lived corporate root.
func NewCA(org string, now time.Time) (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	sn, err := serial()
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          sn,
		Subject:               pkix.Name{CommonName: org + " Internal Root CA", Organization: []string{org}},
		NotBefore:             now.AddDate(-2, -1, 0),
		NotAfter:              now.AddDate(8, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &CA{Cert: cert, Key: key, PEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}, nil
}

// Leaf issues a server certificate for the given host names (wildcards
// allowed), valid for about a year around now.
func (ca *CA) Leaf(hosts []string, now time.Time) (tls.Certificate, error) {
	if len(hosts) == 0 {
		return tls.Certificate{}, fmt.Errorf("no host names")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	sn, err := serial()
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: sn,
		Subject:      pkix.Name{CommonName: hosts[0]},
		DNSNames:     hosts,
		NotBefore:    now.AddDate(0, -2, 0),
		NotAfter:     now.AddDate(0, 10, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.Cert, &key.PublicKey, ca.Key)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der, ca.Cert.Raw}, PrivateKey: key}, nil
}

// AWSHosts returns the AWS host names the sandbox is redirected for: global
// and regional STS, IAM, S3 and Secrets Manager endpoints, plus
// virtual-hosted names of the given buckets.
func AWSHosts(region string, buckets []string) []string {
	hosts := []string{
		"sts.amazonaws.com", "sts." + region + ".amazonaws.com",
		"iam.amazonaws.com",
		"s3.amazonaws.com", "s3." + region + ".amazonaws.com",
		"secretsmanager." + region + ".amazonaws.com",
	}
	for _, b := range buckets {
		hosts = append(hosts, b+".s3.amazonaws.com", b+".s3."+region+".amazonaws.com")
	}
	return hosts
}

// CertNames returns the names to put in the server certificate: the exact
// hosts plus wildcards, so other regional or bucket names also verify.
func CertNames(region string, buckets []string) []string {
	return append(AWSHosts(region, buckets),
		"*.amazonaws.com", "*."+region+".amazonaws.com",
		"*.s3.amazonaws.com", "*.s3."+region+".amazonaws.com")
}
