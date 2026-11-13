package tokensource

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
)

// PoolEntry is one pre-issued token, e.g. created by a provider or by hand.
type PoolEntry struct {
	ID     string            `json:"id"`
	Type   string            `json:"type"`
	Origin string            `json:"origin"` // provider name or "own"
	Fields map[string]string `json:"fields"`
	Facts  facts.Facts       `json:"facts,omitempty"`
}

// PoolFile is the on-disk pool format (tokens/pool.json, not committed).
type PoolFile struct {
	Version int         `json:"version"`
	Entries []PoolEntry `json:"entries"`
}

// LoadPool reads a pool file.
func LoadPool(path string) (PoolFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return PoolFile{}, err
	}
	var p PoolFile
	if err := json.Unmarshal(b, &p); err != nil {
		return PoolFile{}, fmt.Errorf("pool %s: %w", path, err)
	}
	return p, nil
}

// Save writes the pool file with owner-only permissions.
func (p PoolFile) Save(path string) error {
	if p.Version == 0 {
		p.Version = 1
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

// Add appends entries, skipping ids that already exist. Returns how many were added.
func (p *PoolFile) Add(entries ...PoolEntry) int {
	seen := map[string]bool{}
	for _, e := range p.Entries {
		seen[e.ID] = true
	}
	n := 0
	for _, e := range entries {
		if !seen[e.ID] {
			p.Entries = append(p.Entries, e)
			seen[e.ID] = true
			n++
		}
	}
	return n
}

// Pool issues pre-issued tokens. The entry is chosen deterministically from
// the run seed and token id, so a run can be repeated with the same token.
type Pool struct {
	byType map[string][]PoolEntry
}

// NewPool builds a pool source from a pool file.
func NewPool(p PoolFile) *Pool {
	byType := map[string][]PoolEntry{}
	for _, e := range p.Entries {
		byType[e.Type] = append(byType[e.Type], e)
	}
	for _, es := range byType {
		sort.Slice(es, func(i, j int) bool { return es[i].ID < es[j].ID })
	}
	return &Pool{byType: byType}
}

func (p *Pool) Name() string { return "pool" }

func (p *Pool) Supports(t string) bool { return len(p.byType[t]) > 0 }

func (p *Pool) Issue(_ context.Context, req TokenRequest) (Material, error) {
	es := p.byType[req.Type]
	if len(es) == 0 {
		return Material{}, fmt.Errorf("pool has no tokens of type %q", req.Type)
	}
	e := es[subSeed(req.Seed, req.TokenID)%uint64(len(es))]
	fields := map[string]string{}
	var match []string
	for _, k := range sortedKeys(e.Fields) {
		fields[k] = e.Fields[k]
		if isSecretField(k) {
			match = append(match, e.Fields[k])
		}
	}
	return Material{Fields: fields, Facts: facts.Facts{}.Merge(e.Facts), Match: match,
		Source: "pool:" + e.Origin}, nil
}

func sortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// isSecretField reports whether a field value identifies the token and should
// be matched by the detectors.
func isSecretField(k string) bool {
	switch k {
	case "access_key_id", "secret_access_key", "token", "password", "url", "connection_string":
		return true
	}
	return false
}
