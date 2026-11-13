// Package facts holds the key-value data that layers export to the layers
// built after them (user, company, paths, token ids, bucket names).
package facts

import "sort"

// Facts maps a key such as "persona.user" or "token.creds.bucket" to a value.
type Facts map[string]string

// Merge returns a new Facts with the entries of f overwritten by o.
func (f Facts) Merge(o Facts) Facts {
	out := make(Facts, len(f)+len(o))
	for k, v := range f {
		out[k] = v
	}
	for k, v := range o {
		out[k] = v
	}
	return out
}

// Keys returns the keys in sorted order.
func (f Facts) Keys() []string {
	ks := make([]string, 0, len(f))
	for k := range f {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
