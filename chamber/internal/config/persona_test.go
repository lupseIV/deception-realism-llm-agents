package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPersonaAWSDeveloper(t *testing.T) {
	p, err := LoadPersona(filepath.Join(repo, "personas"), "aws-developer")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if p.Identity.User != "andrei.pop" || p.Identity.Region != "eu-central-1" || p.Identity.CompanySlug != "valtera" {
		t.Errorf("identity: %+v", p.Identity)
	}
	if len(p.System.Packages) != 7 || p.System.Packages[0] != "awscli" {
		t.Errorf("packages not split into words: %q", p.System.Packages)
	}
	if len(p.Config) != 2 || p.Config[1] != ".ssh/config" {
		t.Errorf("config not split into words: %q", p.Config)
	}
	if path, ok := p.TokenPath(TokenTypeAWSCredentials); !ok || path != "~/.aws/credentials" {
		t.Errorf("aws placement: %q %v", path, ok)
	}
	if _, ok := p.TokenPath(TokenTypeHFToken); ok {
		t.Error("aws-developer has no hf-token placement")
	}
}

func TestSchemaInvalidPersonasAreRejected(t *testing.T) {
	cases := map[string]string{
		"persona-invalid-duplicate-token.xml": "more than one placement",
		"persona-invalid-path.xml":            `path "aws/credentials" must start with ~ or /`,
	}
	for file, want := range cases {
		var p Persona
		if err := readXML(filepath.Join(repo, "schema", "testdata", file), &p); err != nil {
			t.Fatal(err)
		}
		err := p.Validate()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want error containing %q, got %v", file, want, err)
		}
	}
}

// validPersona returns a minimal persona that passes Validate.
func validPersona() Persona {
	return Persona{
		ID: "dev",
		Identity: Identity{User: "a.b", FullName: "A B", Hostname: "h1",
			Company: "C", CompanySlug: "c", Role: "developer"},
		Files:         []Dir{{Path: "~/work", Kind: DirKindGitRepo, Count: "1-5"}},
		History:       &History{Shell: "bash", Length: "10-20"},
		TokenAffinity: []TokenPlacement{{Type: TokenTypeAWSCredentials, Path: "~/.aws/credentials"}},
	}
}

func TestPersonaRules(t *testing.T) {
	cases := []struct {
		name   string
		change func(p *Persona)
		want   string // "" = must be valid
	}{
		{"valid as is", func(p *Persona) {}, ""},
		{"no optional sections", func(p *Persona) { p.Files, p.History = nil, nil }, ""},
		{"absolute token path", func(p *Persona) { p.TokenAffinity[0].Path = "/root/.aws/credentials" }, ""},
		{"missing id", func(p *Persona) { p.ID = "" }, "persona: missing id"},
		{"user with capitals", func(p *Persona) { p.Identity.User = "Andrei" }, `user "Andrei"`},
		{"missing full name", func(p *Persona) { p.Identity.FullName = " " }, "missing full-name"},
		{"missing company", func(p *Persona) { p.Identity.Company = "" }, "missing company"},
		{"bad hostname", func(p *Persona) { p.Identity.Hostname = "Dev_PC" }, `hostname "Dev_PC"`},
		{"missing company slug", func(p *Persona) { p.Identity.CompanySlug = "" }, "missing company-slug"},
		{"relative dir", func(p *Persona) { p.Files[0].Path = "work" }, "path must start with ~ or /"},
		{"unknown dir kind", func(p *Persona) { p.Files[0].Kind = "svn" }, `kind "svn"`},
		{"bad count", func(p *Persona) { p.Files[0].Count = "many" }, `count "many"`},
		{"bad history length", func(p *Persona) { p.History.Length = "500" }, `length "500"`},
		{"no token placement", func(p *Persona) { p.TokenAffinity = nil }, "needs at least one token"},
		{"unknown token type", func(p *Persona) { p.TokenAffinity[0].Type = "ssh-key" }, "unknown type"},
		{"two placements for one type", func(p *Persona) {
			p.TokenAffinity = append(p.TokenAffinity, TokenPlacement{Type: TokenTypeAWSCredentials, Path: "/x"})
		}, "more than one placement"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validPersona()
			tc.change(&p)
			err := p.Validate()
			switch {
			case tc.want == "" && err != nil:
				t.Fatalf("want valid, got %v", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}
