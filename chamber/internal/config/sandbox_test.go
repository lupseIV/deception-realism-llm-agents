package config

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repo is the repository root seen from this package directory.
const repo = "../../.."

func readSandbox(t *testing.T, path string) Sandbox {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var s Sandbox
	if err := xml.Unmarshal(b, &s); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return s
}

func TestPilotConfigsAreValid(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(repo, "configs", "pilot", "*.xml"))
	if len(files) != 5 {
		t.Fatalf("expected 5 pilot configs, found %d", len(files))
	}
	for _, f := range files {
		s := readSandbox(t, f)
		if err := s.Validate(); err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
		}
	}
}

func TestSchemaInvalidSandboxesAreRejected(t *testing.T) {
	cases := map[string]string{
		"sandbox-invalid-bind.xml":         `bind "missing" is not a token id`,
		"sandbox-invalid-source.xml":       `source "pool" is not a declared source`,
		"sandbox-invalid-realism.xml":      `realism "perfect"`,
		"sandbox-invalid-duplicate-id.xml": `honeytokens/token "creds": duplicate id`,
	}
	for file, want := range cases {
		s := readSandbox(t, filepath.Join(repo, "schema", "testdata", file))
		err := s.Validate()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want error containing %q, got %v", file, want, err)
		}
	}
}

// valid returns a sandbox like ht-high-net-faithful.xml.
func valid() Sandbox {
	return Sandbox{
		ID:          "ht-high-net-faithful",
		Egress:      EgressTypeDetonation,
		Base:        SandboxBase{Image: "chamber/base:0.1", Runtime: DockerRuntimeRunsc},
		Sources:     []Source{{ID: "own", Type: SourceTypeGenerator}},
		Environment: Environment{Persona: "aws-developer", RealismLevel: EnvRealismLevelLow},
		HoneyTokens: &HoneyTokens{Realism: HoneyTokenRealismLevelHigh,
			Tokens: []Token{{ID: "creds", Type: TokenTypeAWSCredentials, Source: "own"}}},
		Network: &Network{RealismLevel: NetworkRealismLevelFaithful,
			Services: []NetworkService{{ID: "aws", NetworkServiceType: NetworkServiceTypeAWS, Bind: "creds"}}},
	}
}

func TestSandboxRules(t *testing.T) {
	cases := []struct {
		name   string
		change func(s *Sandbox)
		want   string // "" = must be valid
	}{
		{"valid as is", func(s *Sandbox) {}, ""},
		{"empty runtime means default", func(s *Sandbox) { s.Base.Runtime = "" }, ""},
		{"missing id", func(s *Sandbox) { s.ID = "" }, "sandbox: missing id"},
		{"id with capitals", func(s *Sandbox) { s.ID = "Pilot" }, `id "Pilot" must match`},
		{"unknown egress", func(s *Sandbox) { s.Egress = "open" }, `egress "open"`},
		{"missing image", func(s *Sandbox) { s.Base.Image = "" }, "base: missing image"},
		{"unknown runtime", func(s *Sandbox) { s.Base.Runtime = "kata" }, `runtime "kata"`},
		{"pool without file", func(s *Sandbox) { s.Sources[0].Type = SourceTypePool }, "pool source needs a file"},
		{"generator with file", func(s *Sandbox) { s.Sources[0].File = "x.json" }, "generator source takes no file"},
		{"unknown source type", func(s *Sandbox) { s.Sources[0].Type = "vault" }, `type "vault"`},
		{"missing persona", func(s *Sandbox) { s.Environment.Persona = "" }, "environment: missing persona"},
		{"env realism medium", func(s *Sandbox) { s.Environment.RealismLevel = EnvRealismLevelMedium }, "not implemented yet"},
		{"env realism unknown", func(s *Sandbox) { s.Environment.RealismLevel = "max" }, `realism "max"`},
		{"negative age", func(s *Sandbox) { s.Environment.AgeDays = -1 }, "age-days -1"},
		{"age ignored at low", func(s *Sandbox) { s.Environment.AgeDays = 240 }, ""},
		{"no tokens", func(s *Sandbox) { s.HoneyTokens.Tokens = nil }, "needs at least one token"},
		{"token type not implemented", func(s *Sandbox) { s.HoneyTokens.Tokens[0].Type = TokenTypeHFToken }, `type "hf-token" is not implemented yet`},
		{"token without source", func(s *Sandbox) { s.HoneyTokens.Tokens[0].Source = "" }, "missing source"},
		{"no services", func(s *Sandbox) { s.Network.Services = nil }, "needs at least one service"},
		{"unbound service is allowed", func(s *Sandbox) { s.Network.Services[0].Bind = "" }, ""},
		{"detonation without network", func(s *Sandbox) { s.Network = nil }, "egress detonation needs a <network> section"},
		{"blocked with network", func(s *Sandbox) { s.Egress = EgressTypeBlocked }, "egress blocked with a <network> section"},
		{"control shape", func(s *Sandbox) {
			s.Egress, s.Sources, s.HoneyTokens, s.Network = EgressTypeBlocked, nil, nil, nil
		}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := valid()
			tc.change(&s)
			err := s.Validate()
			switch {
			case tc.want == "" && err != nil:
				t.Fatalf("want valid, got %v", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestSandboxReportsAllProblems(t *testing.T) {
	s := valid()
	s.Base.Image = ""
	s.Network.Services[0].Bind = "missing"
	err := s.Validate()
	if err == nil {
		t.Fatal("want errors")
	}
	for _, want := range []string{"missing image", `bind "missing"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
}
