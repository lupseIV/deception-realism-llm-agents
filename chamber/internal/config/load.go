package config

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Options says where the loader finds files.
type Options struct {
	// BaseDir is the repository root; relative paths in a configuration
	// (pool files) resolve against it. Default ".".
	BaseDir string
	// PersonaDir holds <id>.xml persona files. Default <BaseDir>/personas.
	PersonaDir string
}

func (o Options) withDefaults() Options {
	if o.BaseDir == "" {
		o.BaseDir = "."
	}
	if o.PersonaDir == "" {
		o.PersonaDir = filepath.Join(o.BaseDir, "personas")
	}
	return o
}

// Condition is one checked sandbox configuration with its persona.
type Condition struct {
	Path    string
	Sandbox Sandbox
	Persona Persona
}

// LoadSandbox reads and parses a sandbox configuration and applies defaults.
// It does not validate; use Load for that.
func LoadSandbox(path string) (Sandbox, error) {
	var s Sandbox
	if err := readXML(path, &s); err != nil {
		return Sandbox{}, err
	}
	if s.Base.Runtime == "" {
		s.Base.Runtime = DockerRuntimeRunsc
	}
	return s, nil
}

// LoadPersona reads <dir>/<id>.xml. The id attribute in the file must equal id.
func LoadPersona(dir string, id ID) (Persona, error) {
	path := filepath.Join(dir, string(id)+".xml")
	var p Persona
	if err := readXML(path, &p); err != nil {
		return Persona{}, err
	}
	if p.ID != id {
		return Persona{}, fmt.Errorf("%s: persona id %q does not match the file name %q", path, p.ID, id)
	}
	return p, nil
}

// Load reads a sandbox configuration and its persona and checks both,
// including the rules that need both files or the file system. All problems
// are reported, each prefixed with the file it comes from.
func Load(path string, opt Options) (Condition, error) {
	opt = opt.withDefaults()
	s, err := LoadSandbox(path)
	if err != nil {
		return Condition{}, err
	}

	var errs []error
	errs = append(errs, prefixed(path, s.Validate())...)
	errs = append(errs, prefixed(path, checkPoolFiles(s, opt.BaseDir))...)

	var p Persona
	if idPattern.MatchString(s.Environment.Persona) {
		personaPath := filepath.Join(opt.PersonaDir, s.Environment.Persona+".xml")
		p, err = LoadPersona(opt.PersonaDir, ID(s.Environment.Persona))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: environment: persona %q: %w", path, s.Environment.Persona, err))
		} else {
			errs = append(errs, prefixed(personaPath, p.Validate())...)
			errs = append(errs, prefixed(path, checkPlacements(s, p))...)
		}
	}

	if err := errors.Join(errs...); err != nil {
		return Condition{}, err
	}
	return Condition{Path: path, Sandbox: s, Persona: p}, nil
}

// checkPoolFiles reports pool sources whose file does not exist.
func checkPoolFiles(s Sandbox, baseDir string) error {
	var errs []error
	for _, src := range s.Sources {
		if src.Type != SourceTypePool || src.File == "" {
			continue
		}
		f := src.File
		if !filepath.IsAbs(f) {
			f = filepath.Join(baseDir, f)
		}
		if _, err := os.Stat(f); err != nil {
			errs = append(errs, problem(fmt.Sprintf("sources/source %q", src.ID), "pool file %q: %v", src.File, err))
		}
	}
	return errors.Join(errs...)
}

// checkPlacements reports token types the persona has no placement for.
func checkPlacements(s Sandbox, p Persona) error {
	if s.HoneyTokens == nil {
		return nil
	}
	var errs []error
	for _, t := range s.HoneyTokens.Tokens {
		if _, ok := p.TokenPath(t.Type); !ok && t.Type.Valid() {
			errs = append(errs, problem(fmt.Sprintf("honeytokens/token %q", t.ID),
				"persona %q has no placement for token type %q", p.ID, t.Type))
		}
	}
	return errors.Join(errs...)
}

// readXML reads and unmarshals one file; errors name the file.
func readXML(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := xml.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// prefixed splits a joined error and prefixes every part with the file path.
func prefixed(path string, err error) []error {
	if err == nil {
		return nil
	}
	parts := []error{err}
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		parts = j.Unwrap()
	}
	out := make([]error, 0, len(parts))
	for _, e := range parts {
		out = append(out, fmt.Errorf("%s: %w", path, e))
	}
	return out
}
