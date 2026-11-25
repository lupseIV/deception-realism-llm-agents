// Package helpers holds the small helpers every layer uses: deterministic
// randomness from the run seed, timestamps and home paths. No domain logic.
// Changing NewRand or Salt changes every generated sandbox, so runs made
// before and after the change are not comparable.
package helpers

import (
	"hash/fnv"
	"math/rand/v2"
	"path"
	"strings"
	"time"
)

// Salt mixes a name into a seed, so that layers and tokens of one run draw
// different numbers from the same run seed.
func Salt(seed uint64, name string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	return seed ^ h.Sum64()
}

// NewRand returns the deterministic generator for seed.
func NewRand(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
}

// Pick returns one element of xs.
func Pick[T any](r *rand.Rand, xs []T) T { return xs[r.IntN(len(xs))] }

// Bytes returns n random bytes.
func Bytes(r *rand.Rand, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.IntN(256))
	}
	return b
}

// Ago returns a time between minDays and maxDays before now, at a working
// hour (09:00-17:59).
func Ago(r *rand.Rand, now time.Time, minDays, maxDays int) time.Time {
	d := minDays + r.IntN(maxDays-minDays+1)
	t := now.AddDate(0, 0, -d)
	return time.Date(t.Year(), t.Month(), t.Day(), 9+r.IntN(9), r.IntN(60), r.IntN(60), 0, t.Location())
}

// ExpandHome turns "~" and "~/x" into home and home/x.
func ExpandHome(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return path.Join(home, p[2:])
	}
	return p
}
