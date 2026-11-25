package tokensource

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var (
	ctx = context.Background()
	now = func() time.Time { return time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC) }
)

func req(id, level string, seed uint64) TokenRequest {
	return TokenRequest{TokenID: id, Type: AWSCredentials, RunID: "r1", Seed: seed, Level: level}
}

func TestGeneratorLevelsAndDeterminism(t *testing.T) {
	g := Generator{Now: now}
	low, err := g.Issue(ctx, req("creds", "low", 1))
	if err != nil || !strings.Contains(low.Fields["access_key_id"], "EXAMPLE") {
		t.Fatalf("low: %v %+v", err, low)
	}
	a, _ := g.Issue(ctx, req("creds", "high", 7))
	b, _ := g.Issue(ctx, req("creds", "high", 7))
	c, _ := g.Issue(ctx, req("other", "high", 7))
	if a.Fields["access_key_id"] != b.Fields["access_key_id"] {
		t.Fatal("same seed and token id must give same token")
	}
	if a.Fields["access_key_id"] == c.Fields["access_key_id"] {
		t.Fatal("different token ids in one run must differ")
	}
	if !regexp.MustCompile(`^AKIA[A-Z2-7]{16}$`).MatchString(a.Fields["access_key_id"]) || len(a.Facts) != 0 {
		t.Fatalf("bad high material %+v", a)
	}
	if len(a.Match) != 2 || a.Source != "generator" {
		t.Fatalf("bad match/source %+v", a)
	}
	if _, err := g.Issue(ctx, req("creds", "extreme", 1)); err == nil {
		t.Fatal("expected error for unknown level")
	}
}

const credsFile = `# downloaded from provider
[default]
aws_access_key_id = AKIAEXAMPLEPROVIDER1
aws_secret_access_key = secretsecretsecretsecretsecretsecret0001
region = us-east-2
output = json

[profile-only]
region = eu-west-1

[second]
aws_access_key_id = AKIAEXAMPLEPROVIDER2
aws_secret_access_key = secretsecretsecretsecretsecretsecret0002
`

func TestImportAWSCredentials(t *testing.T) {
	es, err := ImportAWSCredentials(strings.NewReader(credsFile), "canarytokens", "ct")
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(es))
	}
	if es[0].Facts["region"] != "us-east-2" || es[0].Origin != "canarytokens" || es[0].Type != AWSCredentials {
		t.Fatalf("bad entry %+v", es[0])
	}
	if _, err := ImportAWSCredentials(strings.NewReader("key = value\n"), "x", "x"); err == nil {
		t.Fatal("expected error for key outside section")
	}
}

func TestPoolSaveLoadIssue(t *testing.T) {
	es, _ := ImportAWSCredentials(strings.NewReader(credsFile), "canarytokens", "ct")
	var pf PoolFile
	if n := pf.Add(es...); n != 2 {
		t.Fatalf("added %d", n)
	}
	if n := pf.Add(es...); n != 0 {
		t.Fatal("duplicates must be skipped")
	}
	path := filepath.Join(t.TempDir(), "pool.json")
	if err := pf.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPool(path)
	if err != nil || loaded.Version != 1 || len(loaded.Entries) != 2 {
		t.Fatalf("load: %v %+v", err, loaded)
	}

	p := NewPool(loaded)
	m1, err := p.Issue(ctx, req("creds", "high", 3))
	if err != nil {
		t.Fatal(err)
	}
	m2, _ := p.Issue(ctx, req("creds", "high", 3))
	if m1.Fields["access_key_id"] != m2.Fields["access_key_id"] {
		t.Fatal("pool choice must be deterministic")
	}
	if m1.Source != "pool:canarytokens" || len(m1.Match) != 2 {
		t.Fatalf("bad pool material %+v", m1)
	}
	if p.Supports("hf-token") {
		t.Fatal("pool must not support types it has no entries for")
	}
}

func TestRegistrySwitchesSourcesTransparently(t *testing.T) {
	es, _ := ImportAWSCredentials(strings.NewReader(credsFile), "canarytokens", "ct")
	r := NewRegistry()
	if err := r.Register("own", Generator{Now: now}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register("pool", NewPool(PoolFile{Entries: es})); err != nil {
		t.Fatal(err)
	}
	if err := r.Register("own", Generator{}); err == nil {
		t.Fatal("duplicate id must fail")
	}
	for _, id := range []string{"own", "pool"} {
		m, err := r.Issue(ctx, id, req("creds", "medium", 9))
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		// Same shape regardless of the source.
		if m.TokenID != "creds" || m.Type != AWSCredentials ||
			m.Fields["access_key_id"] == "" || m.Fields["secret_access_key"] == "" {
			t.Fatalf("%s: bad material %+v", id, m)
		}
	}
	if _, err := r.Issue(ctx, "missing", req("creds", "low", 1)); err == nil {
		t.Fatal("unknown source must fail")
	}
}

func TestGeneratorSSHKey(t *testing.T) {
	g := Generator{Now: now}
	sshReq := func(id, level string, seed uint64) TokenRequest {
		r := req(id, level, seed)
		r.Type = SSHPrivateKey
		return r
	}
	low1, err := g.Issue(ctx, sshReq("key", "low", 1))
	if err != nil {
		t.Fatal(err)
	}
	low2, _ := g.Issue(ctx, sshReq("key", "low", 99))
	if low1.Fields["private_key_seed"] != ExampleSSHSeed || low2.Fields["private_key_seed"] != ExampleSSHSeed {
		t.Fatalf("low must use the RFC 8032 example key: %+v", low1)
	}
	a, _ := g.Issue(ctx, sshReq("key", "high", 7))
	b, _ := g.Issue(ctx, sshReq("key", "high", 7))
	c, _ := g.Issue(ctx, sshReq("other", "high", 7))
	m, _ := g.Issue(ctx, sshReq("key", "medium", 8))
	if a.Fields["private_key_seed"] != b.Fields["private_key_seed"] {
		t.Fatal("same seed and token id must give the same key")
	}
	for _, x := range []Material{c, m} {
		if x.Fields["private_key_seed"] == a.Fields["private_key_seed"] || x.Fields["private_key_seed"] == ExampleSSHSeed {
			t.Fatal("keys must differ between token ids and runs, and from the example key")
		}
	}
	if !strings.HasPrefix(a.Fields["fingerprint"], "SHA256:") || len(a.Fields["private_key_seed"]) != 64 ||
		!strings.HasPrefix(a.Fields["public_key"], "AAAAC3NzaC1lZDI1NTE5") {
		t.Fatalf("bad ssh material %+v", a)
	}
	if len(a.Match) != 2 || a.Match[0] != a.Fields["public_key"] || a.Source != "generator" {
		t.Fatalf("bad match/source %+v", a)
	}
	for _, s := range a.Match {
		if strings.Contains(s, a.Fields["private_key_seed"]) {
			t.Fatal("the private seed is not a match string")
		}
	}
}
