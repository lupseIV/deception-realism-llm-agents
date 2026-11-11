package honeytoken

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

var (
	now        = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	reAccessID = regexp.MustCompile(`^AKIA[A-Z2-7]{16}$`)
	reSecret   = regexp.MustCompile(`^[A-Za-z0-9/+]{40}$`)
)

func TestLowUsesKnownExampleValues(t *testing.T) {
	b := New(1, now).Generate("r1", Low)
	if b.Token.AccessKeyID != ExampleAccessKeyID || b.Token.SecretAccessKey != ExampleSecretAccessKey {
		t.Fatalf("low level must use example credentials, got %+v", b.Token)
	}
	if len(b.Files) != 1 || !strings.Contains(b.Files[0].Content, "EXAMPLE") {
		t.Fatalf("low bundle should be one credentials file with example key")
	}
}

func TestMediumAndHighFormat(t *testing.T) {
	for _, lvl := range []Level{Medium, High} {
		b := New(42, now).Generate("r1", lvl)
		if !reAccessID.MatchString(b.Token.AccessKeyID) {
			t.Errorf("%v: bad access key id %q", lvl, b.Token.AccessKeyID)
		}
		if !reSecret.MatchString(b.Token.SecretAccessKey) {
			t.Errorf("%v: bad secret %q", lvl, b.Token.SecretAccessKey)
		}
		if strings.Contains(b.Token.AccessKeyID, "EXAMPLE") {
			t.Errorf("%v: must not contain known artifact", lvl)
		}
	}
}

func TestDeterministicForSameSeed(t *testing.T) {
	a := New(7, now).Generate("r1", High)
	b := New(7, now).Generate("r1", High)
	if a.Token != b.Token || len(a.Files) != len(b.Files) {
		t.Fatal("same seed must give same bundle")
	}
	for i := range a.Files {
		if a.Files[i] != b.Files[i] {
			t.Fatalf("file %d differs", i)
		}
	}
}

func TestUniqueAcrossSeeds(t *testing.T) {
	seen := map[string]bool{}
	for s := uint64(0); s < 200; s++ {
		id := New(s, now).Generate("r", High).Token.AccessKeyID
		if seen[id] {
			t.Fatalf("duplicate access key id for seed %d", s)
		}
		seen[id] = true
	}
}

func TestHighHasContextAndWearAndTear(t *testing.T) {
	b := New(3, now).Generate("r1", High)
	files := map[string]File{}
	for _, f := range b.Files {
		files[f.Path] = f
		if !f.ModTime.Before(now) {
			t.Errorf("%s: mtime must be in the past", f.Path)
		}
	}
	for _, p := range []string{".aws/config", ".aws/credentials", ".bash_history"} {
		if _, ok := files[p]; !ok {
			t.Fatalf("missing %s", p)
		}
	}
	if !strings.Contains(files[".bash_history"].Content, "--profile "+b.Token.Profile) {
		t.Error("history must reference the token profile")
	}
	if !strings.Contains(files[".aws/credentials"].Content, b.Token.AccessKeyID) {
		t.Error("credentials file must contain the token")
	}
	if !files[".aws/config"].ModTime.Before(files[".aws/credentials"].ModTime) ||
		!files[".aws/credentials"].ModTime.Before(files[".bash_history"].ModTime) {
		t.Error("expected config < credentials < history in mtime order")
	}
}

func TestParseLevel(t *testing.T) {
	for _, l := range []Level{Low, Medium, High} {
		got, err := ParseLevel(l.String())
		if err != nil || got != l {
			t.Fatalf("round trip failed for %v", l)
		}
	}
	if _, err := ParseLevel("extreme"); err == nil {
		t.Fatal("expected error for unknown level")
	}
}
