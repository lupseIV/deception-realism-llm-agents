package config

import (
	"encoding/xml"
	"errors"
	"fmt"
	"regexp"
)

type DockerRuntimeType string

const (
	DockerRuntimeRunc  DockerRuntimeType = "runc"
	DockerRuntimeRunsc DockerRuntimeType = "runsc"
)

func (r DockerRuntimeType) Valid() bool {
	return r == DockerRuntimeRunc || r == DockerRuntimeRunsc
}

type SourceType string

const (
	SourceTypeGenerator SourceType = "generator"
	SourceTypePool      SourceType = "pool"
)

func (t SourceType) Valid() bool {
	return t == SourceTypeGenerator || t == SourceTypePool
}

type EnvRealismLevel string

const (
	EnvRealismLevelLow    EnvRealismLevel = "low"
	EnvRealismLevelMedium EnvRealismLevel = "medium"
	EnvRealismLevelHigh   EnvRealismLevel = "high"
)

func (l EnvRealismLevel) Valid() bool {
	switch l {
	case EnvRealismLevelLow, EnvRealismLevelMedium, EnvRealismLevelHigh:
		return true
	}
	return false
}

type HoneyTokenRealismLevel string

const (
	HoneyTokenRealismLevelLow    HoneyTokenRealismLevel = "low"
	HoneyTokenRealismLevelMedium HoneyTokenRealismLevel = "medium"
	HoneyTokenRealismLevelHigh   HoneyTokenRealismLevel = "high"
)

func (l HoneyTokenRealismLevel) Valid() bool {
	switch l {
	case HoneyTokenRealismLevelLow, HoneyTokenRealismLevelMedium, HoneyTokenRealismLevelHigh:
		return true
	}
	return false
}

type NetworkRealismLevel string

const (
	NetworkRealismLevelRefused  NetworkRealismLevel = "refused"
	NetworkRealismLevelGeneric  NetworkRealismLevel = "generic"
	NetworkRealismLevelFaithful NetworkRealismLevel = "faithful"
)

func (l NetworkRealismLevel) Valid() bool {
	switch l {
	case NetworkRealismLevelRefused, NetworkRealismLevelGeneric, NetworkRealismLevelFaithful:
		return true
	}
	return false
}

type TokenType string

const (
	TokenTypeAWSCredentials TokenType = "aws-credentials"
	TokenTypeHFToken        TokenType = "hf-token"
	TokenTypeDBCredentials  TokenType = "db-credentials"
)

func (t TokenType) Valid() bool {
	switch t {
	case TokenTypeAWSCredentials, TokenTypeHFToken, TokenTypeDBCredentials:
		return true
	}
	return false
}

type NetworkServiceType string

const (
	NetworkServiceTypeAWS NetworkServiceType = "aws"
)

func (t NetworkServiceType) Valid() bool {
	return t == NetworkServiceTypeAWS
}

type EgressType string

const (
	EgressTypeBlocked    EgressType = "blocked"
	EgressTypeDetonation EgressType = "detonation"
)

func (e EgressType) Valid() bool {
	return e == EgressTypeBlocked || e == EgressTypeDetonation
}

type ID string

type Sandbox struct {
	XMLName     xml.Name     `xml:"sandbox"`
	ID          ID           `xml:"id,attr"`
	Seed        *uint64      `xml:"seed,attr"`
	Base        SandboxBase  `xml:"base"`
	Egress      EgressType   `xml:"egress,attr"`
	Sources     []Source     `xml:"sources>source"`
	Environment Environment  `xml:"environment"`
	HoneyTokens *HoneyTokens `xml:"honeytokens"`
	Network     *Network     `xml:"network"`
}

type Network struct {
	RealismLevel NetworkRealismLevel `xml:"realism,attr"`
	Latency      ID                  `xml:"latency,attr"`
	Services     []NetworkService    `xml:"service"`
}

type NetworkService struct {
	ID                 ID                 `xml:"id,attr"`
	NetworkServiceType NetworkServiceType `xml:"type,attr"`
	Bind               ID                 `xml:"bind,attr"`
}

type HoneyTokens struct {
	Realism HoneyTokenRealismLevel `xml:"realism,attr"`
	Tokens  []Token                `xml:"token"`
}

type Token struct {
	ID     ID        `xml:"id,attr"`
	Type   TokenType `xml:"type,attr"`
	Source ID        `xml:"source,attr"`
}

type SandboxBase struct {
	Image   string            `xml:"image,attr"`
	Runtime DockerRuntimeType `xml:"runtime,attr"`
}

type Source struct {
	ID   ID         `xml:"id,attr"`
	Type SourceType `xml:"type,attr"`
	File string     `xml:"file,attr"`
}

type Environment struct {
	Persona      string          `xml:"persona,attr"`
	RealismLevel EnvRealismLevel `xml:"realism,attr"`
	AgeDays      int             `xml:"age-days,attr"`
}

// --- validation ----------------------------------------------------------------

// idPattern is the Id type of the XSD schemas.
var idPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// Values the schema allows but the chamber does not implement yet. Later
// steps only add entries here.
var (
	supportedEnvRealism   = map[EnvRealismLevel]bool{EnvRealismLevelLow: true}
	supportedTokenTypes   = map[TokenType]bool{TokenTypeAWSCredentials: true}
	supportedServiceTypes = map[NetworkServiceType]bool{NetworkServiceTypeAWS: true}
)

// --- helpers -------------------------------------------------------------------

// problem builds an error that names the place in the file, e.g.
// `network/service "aws": bind "x" is not a token id`.
func problem(where, format string, args ...any) error {
	return fmt.Errorf("%s: "+format, append([]any{where}, args...)...)
}

// checkID reports a missing or malformed id attribute.
func checkID(where, attr string, id ID) []error {
	if id == "" {
		return []error{problem(where, "missing %s", attr)}
	}
	if !idPattern.MatchString(string(id)) {
		return []error{problem(where, "%s %q must match %s", attr, id, idPattern)}
	}
	return nil
}

// --- whole sandbox -------------------------------------------------------------

// Validate checks the sandbox configuration and returns all problems found,
// joined into one error, or nil. Pool files are checked for existence when the
// configuration is loaded, because that needs the repository root.
func (s *Sandbox) Validate() error {
	var errs []error
	errs = append(errs, checkID("sandbox", "id", s.ID)...)
	if !s.Egress.Valid() {
		errs = append(errs, problem("sandbox", "egress %q must be blocked or detonation", s.Egress))
	}
	errs = append(errs, s.Base.Validate()...)
	for _, src := range s.Sources {
		errs = append(errs, src.Validate()...)
	}
	errs = append(errs, s.Environment.Validate()...)
	errs = append(errs, s.HoneyTokens.Validate()...)
	errs = append(errs, s.Network.Validate()...)
	errs = append(errs, s.validateReferences()...)
	errs = append(errs, s.validateEgress()...)
	return errors.Join(errs...) // nil when errs is empty
}

// validateReferences checks unique ids and that token sources and service
// binds point to declared elements.
func (s *Sandbox) validateReferences() []error {
	var errs []error
	unique := func(kind string, ids []ID) map[ID]bool {
		seen := map[ID]bool{}
		for _, id := range ids {
			if id != "" && seen[id] {
				errs = append(errs, problem(fmt.Sprintf("%s %q", kind, id), "duplicate id"))
			}
			seen[id] = true
		}
		return seen
	}

	var sourceIDs, tokenIDs, serviceIDs []ID
	for _, src := range s.Sources {
		sourceIDs = append(sourceIDs, src.ID)
	}
	if s.HoneyTokens != nil {
		for _, t := range s.HoneyTokens.Tokens {
			tokenIDs = append(tokenIDs, t.ID)
		}
	}
	if s.Network != nil {
		for _, svc := range s.Network.Services {
			serviceIDs = append(serviceIDs, svc.ID)
		}
	}
	sources := unique("sources/source", sourceIDs)
	tokens := unique("honeytokens/token", tokenIDs)
	unique("network/service", serviceIDs)

	if s.HoneyTokens != nil {
		for _, t := range s.HoneyTokens.Tokens {
			if t.Source != "" && !sources[t.Source] {
				errs = append(errs, problem(fmt.Sprintf("honeytokens/token %q", t.ID),
					"source %q is not a declared source", t.Source))
			}
		}
	}
	if s.Network != nil {
		for _, svc := range s.Network.Services {
			if svc.Bind != "" && !tokens[svc.Bind] {
				errs = append(errs, problem(fmt.Sprintf("network/service %q", svc.ID),
					"bind %q is not a token id", svc.Bind))
			}
		}
	}
	return errs
}

// validateEgress checks that the egress mode fits the network section.
func (s *Sandbox) validateEgress() []error {
	switch s.Egress {
	case EgressTypeDetonation:
		if s.Network == nil {
			return []error{problem("sandbox", "egress detonation needs a <network> section")}
		}
	case EgressTypeBlocked:
		if s.Network != nil {
			return []error{problem("sandbox", "egress blocked with a <network> section: the fake services would be unreachable")}
		}
	}
	return nil
}

// --- parts -----------------------------------------------------------------------

func (b SandboxBase) Validate() []error {
	var errs []error
	if b.Image == "" {
		errs = append(errs, problem("base", "missing image"))
	}
	// Empty runtime means the default (runsc).
	if b.Runtime != "" && !b.Runtime.Valid() {
		errs = append(errs, problem("base", "runtime %q must be runc or runsc", b.Runtime))
	}
	return errs
}

func (src Source) Validate() []error {
	where := fmt.Sprintf("sources/source %q", src.ID)
	errs := checkID(where, "id", src.ID)
	switch {
	case !src.Type.Valid():
		errs = append(errs, problem(where, "type %q must be generator or pool", src.Type))
	case src.Type == SourceTypePool && src.File == "":
		errs = append(errs, problem(where, "pool source needs a file"))
	case src.Type == SourceTypeGenerator && src.File != "":
		errs = append(errs, problem(where, "generator source takes no file"))
	}
	return errs
}

func (e Environment) Validate() []error {
	errs := checkID("environment", "persona", ID(e.Persona))
	switch {
	case !e.RealismLevel.Valid():
		errs = append(errs, problem("environment", "realism %q must be low, medium or high", e.RealismLevel))
	case !supportedEnvRealism[e.RealismLevel]:
		errs = append(errs, problem("environment", "realism %q is not implemented yet", e.RealismLevel))
	}
	// age-days is ignored at realism low; only negative values are wrong.
	if e.AgeDays < 0 {
		errs = append(errs, problem("environment", "age-days %d must be positive", e.AgeDays))
	}
	return errs
}

// Validate on a nil *HoneyTokens (no <honeytokens> section) reports nothing.
func (h *HoneyTokens) Validate() []error {
	if h == nil {
		return nil
	}
	var errs []error
	if !h.Realism.Valid() {
		errs = append(errs, problem("honeytokens", "realism %q must be low, medium or high", h.Realism))
	}
	if len(h.Tokens) == 0 {
		errs = append(errs, problem("honeytokens", "needs at least one token"))
	}
	for _, t := range h.Tokens {
		errs = append(errs, t.Validate()...)
	}
	return errs
}

func (t Token) Validate() []error {
	where := fmt.Sprintf("honeytokens/token %q", t.ID)
	errs := checkID(where, "id", t.ID)
	switch {
	case !t.Type.Valid():
		errs = append(errs, problem(where, "unknown type %q", t.Type))
	case !supportedTokenTypes[t.Type]:
		errs = append(errs, problem(where, "type %q is not implemented yet", t.Type))
	}
	errs = append(errs, checkID(where, "source", t.Source)...)
	return errs
}

// Validate on a nil *Network (no <network> section) reports nothing.
func (n *Network) Validate() []error {
	if n == nil {
		return nil
	}
	var errs []error
	if !n.RealismLevel.Valid() {
		errs = append(errs, problem("network", "realism %q must be refused, generic or faithful", n.RealismLevel))
	}
	if n.Latency != "" {
		errs = append(errs, checkID("network", "latency", n.Latency)...)
	}
	if len(n.Services) == 0 {
		errs = append(errs, problem("network", "needs at least one service"))
	}
	for _, svc := range n.Services {
		errs = append(errs, svc.Validate()...)
	}
	return errs
}

func (svc NetworkService) Validate() []error {
	where := fmt.Sprintf("network/service %q", svc.ID)
	errs := checkID(where, "id", svc.ID)
	switch {
	case !svc.NetworkServiceType.Valid():
		errs = append(errs, problem(where, "unknown type %q", svc.NetworkServiceType))
	case !supportedServiceTypes[svc.NetworkServiceType]:
		errs = append(errs, problem(where, "type %q is not implemented yet", svc.NetworkServiceType))
	}
	if svc.Bind != "" {
		errs = append(errs, checkID(where, "bind", svc.Bind)...)
	}
	return errs
}
