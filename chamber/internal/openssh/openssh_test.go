package openssh

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// RFC 8032, section 7.1, test 1.
const (
	rfcSeed   = "9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60"
	rfcPublic = "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a"
)

// Golden values printed by OpenSSH's ssh-keygen for the RFC 8032 key:
// ssh-keygen -y -f <PrivatePEM file> and ssh-keygen -l -f <same file>
// (OpenSSH 8.9p1), which also finds KnownHostsLine output with -F.
const (
	goldenPublic      = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINdamAGCsQq31Uv+08lkBzoO4XLz2qYjJa8CGmj3B1Ea"
	goldenFingerprint = "SHA256:bbXpuKG6zhzdmnxq256TlqzFBzRl2f6OOg722cYNbU8"
)

func rfcKey(t *testing.T) Key {
	t.Helper()
	seed, _ := hex.DecodeString(rfcSeed)
	k, err := FromSeed(seed)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestPublicMatchesRFC8032(t *testing.T) {
	k := rfcKey(t)
	if hex.EncodeToString(k.Public) != rfcPublic {
		t.Fatalf("public key %x", k.Public)
	}
	if got := k.AuthorizedKey(""); got != goldenPublic {
		t.Errorf("public line\n got %s\nwant %s", got, goldenPublic)
	}
	if got := k.Fingerprint(); got != goldenFingerprint {
		t.Errorf("fingerprint %s, want %s", got, goldenFingerprint)
	}
	if got := k.AuthorizedKey("ana@host"); got != goldenPublic+" ana@host" {
		t.Errorf("comment: %s", got)
	}
}

func TestPrivatePEMRoundTrip(t *testing.T) {
	k := rfcKey(t)
	for _, comment := range []string{"", "a", "andrei.pop@vt-dev-07", "a comment long enough to need more padding"} {
		pem := k.PrivatePEM(comment, 0x01020304)
		lines := strings.Split(strings.TrimSpace(pem), "\n")
		if lines[0] != pemBegin || lines[len(lines)-1] != pemEnd {
			t.Fatalf("armour:\n%s", pem)
		}
		for _, l := range lines[1 : len(lines)-2] {
			if len(l) != lineWidth {
				t.Fatalf("body line of %d characters", len(l))
			}
		}
		got, c, err := ParsePrivatePEM(pem)
		if err != nil {
			t.Fatalf("%q: %v", comment, err)
		}
		if c != comment || !bytes.Equal(got.Private, k.Private) {
			t.Errorf("round trip lost data for %q", comment)
		}
	}
	if k.PrivatePEM("x", 1) == k.PrivatePEM("x", 2) {
		t.Error("check value must be part of the file")
	}
}

func TestParseRejects(t *testing.T) {
	k := rfcKey(t)
	good := k.PrivatePEM("c", 7)
	for name, s := range map[string]string{
		"not pem":   "hello",
		"bad b64":   pemBegin + "\n!!!!\n" + pemEnd,
		"truncated": pemBegin + "\nb3BlbnNzaC1rZXktdjEA\n" + pemEnd,
		"tampered":  strings.Replace(good, "AAAA", "AAAB", 1),
	} {
		if _, _, err := ParsePrivatePEM(s); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestKnownHostsLine(t *testing.T) {
	k := rfcKey(t)
	salt := bytes.Repeat([]byte{7}, 20)
	line := KnownHostsLine(salt, "backup-01.valtera.internal", k)
	f := strings.Fields(line)
	if len(f) != 3 || f[1] != KeyType || f[2] != k.PublicBase64() || !strings.HasPrefix(f[0], "|1|") {
		t.Fatalf("line %q", line)
	}
	if HashHost(salt, "backup-01.valtera.internal") == HashHost(salt, "backup-02.valtera.internal") {
		t.Error("different hosts must hash differently")
	}
}

func TestFromSeedLength(t *testing.T) {
	if _, err := FromSeed(make([]byte, 31)); err == nil {
		t.Error("want error for a short seed")
	}
}
