// Package openssh writes ed25519 keys in the formats OpenSSH reads and
// writes: the unencrypted "openssh-key-v1" private key file, the one-line
// public key, the SHA256 fingerprint and hashed known_hosts lines. It uses
// only the standard library, so the honeytoken layer and the SSH service
// derive the same bytes from the same seed.
package openssh

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// KeyType is the only key type written by this package.
const KeyType = "ssh-ed25519"

const (
	pemBegin  = "-----BEGIN OPENSSH PRIVATE KEY-----"
	pemEnd    = "-----END OPENSSH PRIVATE KEY-----"
	authMagic = "openssh-key-v1\x00"
	lineWidth = 70 // ssh-keygen wraps the base64 body at 70 characters
)

// Key is an ed25519 key pair.
type Key struct {
	Private ed25519.PrivateKey
	Public  ed25519.PublicKey
}

// FromSeed derives the key pair from a 32-byte seed.
func FromSeed(seed []byte) (Key, error) {
	if len(seed) != ed25519.SeedSize {
		return Key{}, fmt.Errorf("seed must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	priv := ed25519.NewKeyFromSeed(seed)
	return Key{Private: priv, Public: priv.Public().(ed25519.PublicKey)}, nil
}

// PublicBlob is the SSH wire encoding of the public key.
func (k Key) PublicBlob() []byte {
	var b bytes.Buffer
	putString(&b, []byte(KeyType))
	putString(&b, k.Public)
	return b.Bytes()
}

// PublicBase64 is the base64 field of the public key line.
func (k Key) PublicBase64() string { return base64.StdEncoding.EncodeToString(k.PublicBlob()) }

// AuthorizedKey is the line of an id_*.pub or authorized_keys file, without
// the trailing newline. The comment may be empty.
func (k Key) AuthorizedKey(comment string) string {
	line := KeyType + " " + k.PublicBase64()
	if comment != "" {
		line += " " + comment
	}
	return line
}

// Fingerprint is the SHA256 fingerprint as ssh-keygen -l prints it.
func (k Key) Fingerprint() string {
	sum := sha256.Sum256(k.PublicBlob())
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// PrivatePEM is the unencrypted private key file as ssh-keygen writes it.
// check is the random check value OpenSSH puts twice into the private
// section; pass a value from the run's generator so the file is reproducible.
func (k Key) PrivatePEM(comment string, check uint32) string {
	var priv bytes.Buffer
	binary.Write(&priv, binary.BigEndian, check)
	binary.Write(&priv, binary.BigEndian, check)
	putString(&priv, []byte(KeyType))
	putString(&priv, k.Public)
	putString(&priv, k.Private) // seed || public key, 64 bytes
	putString(&priv, []byte(comment))
	for i := byte(1); priv.Len()%8 != 0; i++ { // cipher "none" has block size 8
		priv.WriteByte(i)
	}

	var b bytes.Buffer
	b.WriteString(authMagic)
	putString(&b, []byte("none")) // cipher
	putString(&b, []byte("none")) // kdf
	putString(&b, nil)            // kdf options
	binary.Write(&b, binary.BigEndian, uint32(1))
	putString(&b, k.PublicBlob())
	putString(&b, priv.Bytes())

	body := base64.StdEncoding.EncodeToString(b.Bytes())
	var out strings.Builder
	out.WriteString(pemBegin + "\n")
	for len(body) > lineWidth {
		out.WriteString(body[:lineWidth] + "\n")
		body = body[lineWidth:]
	}
	out.WriteString(body + "\n" + pemEnd + "\n")
	return out.String()
}

// ParsePrivatePEM reads a file written by PrivatePEM (unencrypted ed25519
// only) and returns the key and its comment.
func ParsePrivatePEM(s string) (Key, string, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, pemBegin) || !strings.HasSuffix(s, pemEnd) {
		return Key{}, "", errors.New("not an OpenSSH private key")
	}
	body := strings.Join(strings.Fields(s[len(pemBegin):len(s)-len(pemEnd)]), "")
	raw, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return Key{}, "", err
	}
	if !bytes.HasPrefix(raw, []byte(authMagic)) {
		return Key{}, "", errors.New("missing openssh-key-v1 magic")
	}
	r := reader{raw[len(authMagic):], nil}
	cipher, kdf, _ := r.str(), r.str(), r.str()
	n := r.u32()
	pub := r.str()
	sec := r.str()
	if r.err != nil {
		return Key{}, "", r.err
	}
	if string(cipher) != "none" || string(kdf) != "none" || n != 1 {
		return Key{}, "", errors.New("only unencrypted single-key files are supported")
	}
	p := reader{sec, nil}
	c1, c2 := p.u32(), p.u32()
	typ, pk, sk, comment := p.str(), p.str(), p.str(), p.str()
	if p.err != nil {
		return Key{}, "", p.err
	}
	if c1 != c2 {
		return Key{}, "", errors.New("check values differ")
	}
	if string(typ) != KeyType || len(pk) != ed25519.PublicKeySize || len(sk) != ed25519.PrivateKeySize {
		return Key{}, "", errors.New("not an ed25519 key")
	}
	k, err := FromSeed(sk[:ed25519.SeedSize])
	if err != nil {
		return Key{}, "", err
	}
	if !bytes.Equal(k.Public, pk) || !bytes.Equal(k.PublicBlob(), pub) {
		return Key{}, "", errors.New("public key does not match private key")
	}
	for i, b := range p.buf { // padding 1, 2, 3, ...
		if int(b) != i+1 {
			return Key{}, "", errors.New("bad padding")
		}
	}
	return k, string(comment), nil
}

// HashHost is the |1|salt|hash form of a host name in a hashed known_hosts
// file (HashKnownHosts yes): HMAC-SHA1 of the name, keyed with a 20-byte salt.
func HashHost(salt []byte, host string) string {
	m := hmac.New(sha1.New, salt)
	m.Write([]byte(host))
	enc := base64.StdEncoding.EncodeToString
	return "|1|" + enc(salt) + "|" + enc(m.Sum(nil))
}

// KnownHostsLine is a hashed known_hosts line for host with host key k,
// without the trailing newline. ssh-keygen -F host finds it.
func KnownHostsLine(salt []byte, host string, k Key) string {
	return HashHost(salt, host) + " " + KeyType + " " + k.PublicBase64()
}

func putString(b *bytes.Buffer, s []byte) {
	binary.Write(b, binary.BigEndian, uint32(len(s)))
	b.Write(s)
}

type reader struct {
	buf []byte
	err error
}

func (r *reader) u32() uint32 {
	if r.err != nil || len(r.buf) < 4 {
		r.fail()
		return 0
	}
	v := binary.BigEndian.Uint32(r.buf)
	r.buf = r.buf[4:]
	return v
}

func (r *reader) str() []byte {
	n := r.u32()
	if r.err != nil || uint32(len(r.buf)) < n {
		r.fail()
		return nil
	}
	s := r.buf[:n]
	r.buf = r.buf[n:]
	return s
}

func (r *reader) fail() {
	if r.err == nil {
		r.err = errors.New("truncated key data")
	}
}
