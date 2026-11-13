// Package tokensource separates the value of a honeytoken from its placement
// and detection. A TokenSource returns provider-neutral Material; honeytoken
// layers only see Material and never know where it came from.
package tokensource

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
)

// Token types.
const (
	AWSCredentials = "aws-credentials"
)

// TokenRequest describes the token a layer needs for one run.
type TokenRequest struct {
	TokenID string      // id from the sandbox configuration, e.g. "creds"
	Type    string      // token type, e.g. AWSCredentials
	RunID   string      // run identifier
	Seed    uint64      // run seed
	Level   string      // honeytoken realism level: low | medium | high
	Facts   facts.Facts // persona and environment facts
}

// Material is a token value, independent of its source.
type Material struct {
	TokenID string            `json:"token_id"`
	Type    string            `json:"type"`
	Fields  map[string]string `json:"fields"`          // e.g. access_key_id, secret_access_key
	Facts   facts.Facts       `json:"facts,omitempty"` // values chosen by the source (region, bucket, ...)
	Match   []string          `json:"match"`           // strings the detectors look for
	Source  string            `json:"source"`          // "generator", "pool:<origin>", ...
}

// TokenSource supplies token values.
type TokenSource interface {
	Name() string
	Supports(tokenType string) bool
	Issue(ctx context.Context, req TokenRequest) (Material, error)
}

// Registry maps source ids from the sandbox configuration to sources.
type Registry struct {
	sources map[string]TokenSource
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{sources: map[string]TokenSource{}} }

// Register adds a source under id. Ids must be unique.
func (r *Registry) Register(id string, s TokenSource) error {
	if _, ok := r.sources[id]; ok {
		return fmt.Errorf("token source %q already registered", id)
	}
	r.sources[id] = s
	return nil
}

// Issue asks the source registered under id for a token.
func (r *Registry) Issue(ctx context.Context, id string, req TokenRequest) (Material, error) {
	s, ok := r.sources[id]
	if !ok {
		return Material{}, fmt.Errorf("unknown token source %q", id)
	}
	if !s.Supports(req.Type) {
		return Material{}, fmt.Errorf("token source %q does not support type %q", id, req.Type)
	}
	m, err := s.Issue(ctx, req)
	if err != nil {
		return Material{}, fmt.Errorf("token source %q: %w", id, err)
	}
	m.TokenID, m.Type = req.TokenID, req.Type
	return m, nil
}

// IDs returns the registered source ids in sorted order.
func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.sources))
	for id := range r.sources {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// subSeed derives a per-token seed so that several tokens in one run differ.
func subSeed(seed uint64, tokenID string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(tokenID))
	return seed ^ h.Sum64()
}
