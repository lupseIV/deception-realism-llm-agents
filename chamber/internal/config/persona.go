package config

import (
	"encoding/xml"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Persona describes who uses the machine (personas/<id>.xml). At environment
// realism low only Identity, System.Packages and TokenAffinity are used.
type Persona struct {
	XMLName       xml.Name         `xml:"persona"`
	ID            ID               `xml:"id,attr"`
	Image         string           `xml:"image,attr"`
	Identity      Identity         `xml:"identity"`
	System        *System          `xml:"system"`
	Files         []Dir            `xml:"files>dir"`
	Apps          *Apps            `xml:"apps"`
	Config        Names            `xml:"config"`
	Prefs         Names            `xml:"prefs"`
	History       *History         `xml:"history"`
	TokenAffinity []TokenPlacement `xml:"token-affinity>token"`
}

type Identity struct {
	User        string `xml:"user,attr"`
	FullName    string `xml:"full-name,attr"`
	Hostname    ID     `xml:"hostname,attr"`
	Company     string `xml:"company,attr"`
	CompanySlug ID     `xml:"company-slug,attr"`
	Role        string `xml:"role,attr"`
	Region      string `xml:"region,attr"`
}

type System struct {
	Packages  Names `xml:"packages"`
	Processes Names `xml:"processes"`
}

type DirKind string

const (
	DirKindGitRepo DirKind = "git-repo"
	DirKindPlain   DirKind = "plain"
)

func (k DirKind) Valid() bool { return k == DirKindGitRepo || k == DirKindPlain }

type Dir struct {
	Path  string  `xml:"path,attr"`
	Kind  DirKind `xml:"kind,attr"`
	Count string  `xml:"count,attr"` // "min-max"
	Types string  `xml:"types,attr"` // comma-separated extensions
}

type Apps struct {
	AITools Names `xml:"ai-tools"`
}

type History struct {
	Shell  ID     `xml:"shell,attr"`
	Length string `xml:"length,attr"` // "min-max"
}

// TokenPlacement says where a token type is written for this persona.
type TokenPlacement struct {
	Type TokenType `xml:"type,attr"`
	Path string    `xml:"path,attr"` // ~/... or absolute
}

// Names is a whitespace-separated list in an element's text, e.g.
// <packages>awscli git</packages>.
type Names []string

// UnmarshalXML splits the element text into words.
func (n *Names) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var text string
	if err := d.DecodeElement(&text, &start); err != nil {
		return err
	}
	*n = strings.Fields(text)
	return nil
}

// TokenPath returns the placement path for a token type.
func (p Persona) TokenPath(t TokenType) (string, bool) {
	for _, tp := range p.TokenAffinity {
		if tp.Type == t {
			return tp.Path, true
		}
	}
	return "", false
}

// Patterns of persona.xsd.
var (
	userPattern  = regexp.MustCompile(`^[a-z_][a-z0-9_.-]{0,31}$`)
	pathPattern  = regexp.MustCompile(`^(~|/)\S*$`)
	rangePattern = regexp.MustCompile(`^[0-9]+-[0-9]+$`)
)

// Validate checks the persona and returns all problems found, or nil.
func (p *Persona) Validate() error {
	var errs []error
	errs = append(errs, checkID("persona", "id", p.ID)...)

	id := p.Identity
	if !userPattern.MatchString(id.User) {
		errs = append(errs, problem("identity", "user %q must match %s", id.User, userPattern))
	}
	for attr, v := range map[string]string{"full-name": id.FullName, "company": id.Company, "role": id.Role} {
		if strings.TrimSpace(v) == "" {
			errs = append(errs, problem("identity", "missing %s", attr))
		}
	}
	errs = append(errs, checkID("identity", "hostname", id.Hostname)...)
	errs = append(errs, checkID("identity", "company-slug", id.CompanySlug)...)

	for _, d := range p.Files {
		where := fmt.Sprintf("files/dir %q", d.Path)
		if !pathPattern.MatchString(d.Path) {
			errs = append(errs, problem(where, "path must start with ~ or /"))
		}
		if d.Kind != "" && !d.Kind.Valid() {
			errs = append(errs, problem(where, "kind %q must be git-repo or plain", d.Kind))
		}
		if d.Count != "" && !rangePattern.MatchString(d.Count) {
			errs = append(errs, problem(where, "count %q must be min-max", d.Count))
		}
	}
	if p.History != nil {
		errs = append(errs, checkID("history", "shell", p.History.Shell)...)
		if !rangePattern.MatchString(p.History.Length) {
			errs = append(errs, problem("history", "length %q must be min-max", p.History.Length))
		}
	}

	if len(p.TokenAffinity) == 0 {
		errs = append(errs, problem("token-affinity", "needs at least one token"))
	}
	seen := map[TokenType]bool{}
	for _, tp := range p.TokenAffinity {
		where := fmt.Sprintf("token-affinity/token %q", tp.Type)
		if !tp.Type.Valid() {
			errs = append(errs, problem(where, "unknown type"))
		}
		if seen[tp.Type] {
			errs = append(errs, problem(where, "more than one placement for this type"))
		}
		seen[tp.Type] = true
		if !pathPattern.MatchString(tp.Path) {
			errs = append(errs, problem(where, "path %q must start with ~ or /", tp.Path))
		}
	}
	return errors.Join(errs...)
}
