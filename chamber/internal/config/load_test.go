package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// opts uses the repository's personas and resolves paths from the repo root.
var opts = Options{BaseDir: repo}

func TestLoadPilotConfigs(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(repo, "configs", "pilot", "*.xml"))
	if len(files) != 5 {
		t.Fatalf("expected 5 pilot configs, found %d", len(files))
	}
	for _, f := range files {
		c, err := Load(f, opts)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
			continue
		}
		if c.Sandbox.Base.Runtime != DockerRuntimeRunsc || c.Persona.ID != "aws-developer" || c.Path != f {
			t.Errorf("%s: unexpected condition %+v", filepath.Base(f), c)
		}
	}
}

func TestLoadedValues(t *testing.T) {
	ctl, err := Load(filepath.Join(repo, "configs/pilot/control.xml"), opts)
	if err != nil {
		t.Fatal(err)
	}
	if ctl.Sandbox.HoneyTokens != nil || ctl.Sandbox.Network != nil || ctl.Sandbox.Egress != EgressTypeBlocked {
		t.Errorf("control: want no traps and blocked egress, got %+v", ctl.Sandbox)
	}

	c, err := Load(filepath.Join(repo, "configs/pilot/ht-high-net-faithful.xml"), opts)
	if err != nil {
		t.Fatal(err)
	}
	s := c.Sandbox
	if s.HoneyTokens.Realism != HoneyTokenRealismLevelHigh || s.Network.RealismLevel != NetworkRealismLevelFaithful ||
		s.Network.Services[0].Bind != "creds" {
		t.Errorf("sandbox values: %+v %+v", s.HoneyTokens, s.Network)
	}
}

func TestLoadErrors(t *testing.T) {
	testOpts := Options{BaseDir: repo, PersonaDir: "testdata/personas"}
	cases := []struct {
		file string
		opt  Options
		want []string // every substring must appear
	}{
		{"pool-missing-file.xml", opts, []string{"pool-missing-file.xml", `pool file "tokens/does-not-exist.json"`}},
		{"unknown-persona.xml", opts, []string{`persona "nobody"`}},
		{"persona-wrong-id.xml", testOpts, []string{`persona id "other" does not match the file name "wrong-id"`}},
		{"no-placement.xml", testOpts, []string{`persona "db-only" has no placement for token type "aws-credentials"`}},
		{"two-mistakes.xml", opts, []string{"two-mistakes.xml: base: missing image", `two-mistakes.xml: network/service "aws": bind "missing"`}},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			_, err := Load(filepath.Join("testdata", tc.file), tc.opt)
			if err == nil {
				t.Fatal("want error")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("missing %q in:\n%v", w, err)
				}
			}
		})
	}
}

func TestLoadPoolFileExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tokens"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tokens", "pool.json"), []byte(`{"version":1,"entries":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "pool.xml")
	xml := `<sandbox id="pool-ok" egress="blocked">
  <base image="chamber/base:0.1"/>
  <sources><source id="pool" type="pool" file="tokens/pool.json"/></sources>
  <environment persona="aws-developer" realism="low"/>
  <honeytokens realism="low"><token id="creds" type="aws-credentials" source="pool"/></honeytokens>
</sandbox>`
	if err := os.WriteFile(cfg, []byte(xml), 0o600); err != nil {
		t.Fatal(err)
	}
	// Pool file resolves against BaseDir, personas come from the repo.
	if _, err := Load(cfg, Options{BaseDir: dir, PersonaDir: filepath.Join(repo, "personas")}); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRejectsWrongDocuments(t *testing.T) {
	// A persona file is not a sandbox.
	if _, err := LoadSandbox(filepath.Join(repo, "personas", "aws-developer.xml")); err == nil ||
		!strings.Contains(err.Error(), "expected element type <sandbox>") {
		t.Errorf("want root element error, got %v", err)
	}
	if _, err := LoadSandbox("testdata/does-not-exist.xml"); err == nil {
		t.Error("want error for a missing file")
	}
}
