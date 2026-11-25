package app

import (
	"sort"
	"strings"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
)

// RegionsAndBuckets collects the regions and buckets of all planted tokens.
// The persona's region is always included; with none at all it is us-east-1.
func RegionsAndBuckets(f facts.Facts) (regions, buckets []string) {
	seenR, seenB := map[string]bool{}, map[string]bool{}
	if r := f[facts.PersonaRegion]; r != "" {
		seenR[r] = true
	}
	for k, v := range f {
		if !strings.HasPrefix(k, "token.") || v == "" {
			continue
		}
		switch {
		case strings.HasSuffix(k, "."+facts.TokenRegion):
			seenR[v] = true
		case strings.HasSuffix(k, "."+facts.TokenBucket):
			seenB[v] = true
		}
	}
	for r := range seenR {
		regions = append(regions, r)
	}
	for b := range seenB {
		buckets = append(buckets, b)
	}
	sort.Strings(regions)
	sort.Strings(buckets)
	if len(regions) == 0 {
		regions = []string{"us-east-1"}
	}
	return regions, buckets
}

// NeedsBackend reports whether a network layer runs in faithful mode, i.e.
// in front of Moto.
func NeedsBackend(f facts.Facts) bool {
	for k, v := range f {
		if strings.HasPrefix(k, "net.") && strings.HasSuffix(k, "."+facts.NetMode) && v == "faithful" {
			return true
		}
	}
	return false
}
