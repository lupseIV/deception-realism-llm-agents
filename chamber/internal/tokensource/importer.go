package tokensource

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/facts"
)

// ImportAWSCredentials converts an AWS shared-credentials file into pool
// entries. Providers that hand out AWS key honeytokens (e.g. Canarytokens)
// deliver them in this format, so one importer covers them without depending
// on a provider API. origin names the provider; idPrefix makes ids unique.
func ImportAWSCredentials(r io.Reader, origin, idPrefix string) ([]PoolEntry, error) {
	type section struct {
		name   string
		fields map[string]string
	}
	var secs []*section
	var cur *section
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			cur = &section{name: strings.TrimSpace(line[1 : len(line)-1]), fields: map[string]string{}}
			secs = append(secs, cur)
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || cur == nil {
			return nil, fmt.Errorf("line %d: expected key = value inside a [section]", n)
		}
		cur.fields[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	var out []PoolEntry
	for i, s := range secs {
		id, secret := s.fields["aws_access_key_id"], s.fields["aws_secret_access_key"]
		if id == "" || secret == "" {
			continue // config-only section
		}
		f := facts.Facts{"profile": s.name}
		if reg := s.fields["region"]; reg != "" {
			f["region"] = reg
		}
		out = append(out, PoolEntry{
			ID:     fmt.Sprintf("%s-%d", idPrefix, i),
			Type:   AWSCredentials,
			Origin: origin,
			Fields: map[string]string{"access_key_id": id, "secret_access_key": secret},
			Facts:  f,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no credentials found")
	}
	return out, nil
}
