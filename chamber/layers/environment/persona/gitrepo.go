package persona

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"path"
	"sort"
	"strings"
	"time"
)

// A minimal git writer: enough to plant a working clone (pack from the
// clone, loose objects from later commits, index, refs, reflog) that git
// itself accepts (git fsck, git status clean, git log -p).

type objID [20]byte

func (id objID) String() string { return hex.EncodeToString(id[:]) }

type gitObject struct {
	kind string // blob, tree, commit
	data []byte
}

// raw is the object as hashed: "<kind> <size>\0<data>".
func (o gitObject) raw() []byte {
	return append([]byte(fmt.Sprintf("%s %d\x00", o.kind, len(o.data))), o.data...)
}

func (o gitObject) id() objID { return sha1.Sum(o.raw()) }

func zlibBytes(b []byte) []byte {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, _ = w.Write(b)
	_ = w.Close()
	return buf.Bytes()
}

// objectStore keeps objects in insertion order.
type objectStore struct {
	byID  map[objID]gitObject
	order []objID
}

func newObjectStore() *objectStore { return &objectStore{byID: map[objID]gitObject{}} }

func (s *objectStore) put(o gitObject) objID {
	id := o.id()
	if _, ok := s.byID[id]; !ok {
		s.byID[id] = o
		s.order = append(s.order, id)
	}
	return id
}

// writeTree stores the tree of files (path -> content) and returns its id.
func (s *objectStore) writeTree(files map[string]string) objID {
	type entry struct {
		name string
		dir  bool
		id   objID
	}
	subs := map[string]map[string]string{}
	var entries []entry
	for p, content := range files {
		if dir, rest, ok := strings.Cut(p, "/"); ok {
			if subs[dir] == nil {
				subs[dir] = map[string]string{}
			}
			subs[dir][rest] = content
			continue
		}
		entries = append(entries, entry{name: p, id: s.put(gitObject{"blob", []byte(content)})})
	}
	for dir, sub := range subs {
		entries = append(entries, entry{name: dir, dir: true, id: s.writeTree(sub)})
	}
	// git sorts tree entries by name, with directories compared as "name/".
	key := func(e entry) string {
		if e.dir {
			return e.name + "/"
		}
		return e.name
	}
	sort.Slice(entries, func(i, j int) bool { return key(entries[i]) < key(entries[j]) })
	var b bytes.Buffer
	for _, e := range entries {
		mode := "100644"
		if e.dir {
			mode = "40000"
		}
		fmt.Fprintf(&b, "%s %s\x00", mode, e.name)
		b.Write(e.id[:])
	}
	return s.put(gitObject{"tree", b.Bytes()})
}

// ident formats "Name <email> <unix> <+hhmm>" in Romanian time.
func ident(name, email string, t time.Time) string {
	return fmt.Sprintf("%s <%s> %d %s", name, email, t.Unix(), tzOffset(t))
}

// tzOffset is Europe/Bucharest without tzdata: +0300 from the last Sunday
// of March to the last Sunday of October (01:00 UTC), +0200 otherwise.
func tzOffset(t time.Time) string {
	u := t.UTC()
	lastSunday := func(m time.Month) time.Time {
		d := time.Date(u.Year(), m+1, 0, 1, 0, 0, 0, time.UTC)
		return d.AddDate(0, 0, -int(d.Weekday()))
	}
	if !u.Before(lastSunday(time.March)) && u.Before(lastSunday(time.October)) {
		return "+0300"
	}
	return "+0200"
}

func (s *objectStore) writeCommit(tree objID, parent *objID, author, message string) objID {
	var b strings.Builder
	fmt.Fprintf(&b, "tree %s\n", tree)
	if parent != nil {
		fmt.Fprintf(&b, "parent %s\n", parent)
	}
	fmt.Fprintf(&b, "author %s\ncommitter %s\n\n%s\n", author, author, message)
	return s.put(gitObject{"commit", []byte(b.String())})
}

// loose returns the path under .git and the content of a loose object.
func (s *objectStore) loose(id objID) (string, []byte) {
	h := id.String()
	return "objects/" + h[:2] + "/" + h[2:], zlibBytes(s.byID[id].raw())
}

// pack writes a version 2 pack (no deltas) and its version 2 index.
func (s *objectStore) pack(ids []objID) (name string, pack, idx []byte) {
	typeCode := map[string]byte{"commit": 1, "tree": 2, "blob": 3}
	var p bytes.Buffer
	p.WriteString("PACK")
	_ = binary.Write(&p, binary.BigEndian, uint32(2))
	_ = binary.Write(&p, binary.BigEndian, uint32(len(ids)))
	offsets := map[objID]uint32{}
	crcs := map[objID]uint32{}
	for _, id := range ids {
		o := s.byID[id]
		start := p.Len()
		offsets[id] = uint32(start)
		size := len(o.data)
		hdr := []byte{typeCode[o.kind]<<4 | byte(size&0x0f)}
		size >>= 4
		for size > 0 {
			hdr[len(hdr)-1] |= 0x80
			hdr = append(hdr, byte(size&0x7f))
			size >>= 7
		}
		p.Write(hdr)
		p.Write(zlibBytes(o.data))
		crcs[id] = crc32.ChecksumIEEE(p.Bytes()[start:])
	}
	sum := sha1.Sum(p.Bytes())
	p.Write(sum[:])

	sorted := append([]objID(nil), ids...)
	sort.Slice(sorted, func(i, j int) bool { return bytes.Compare(sorted[i][:], sorted[j][:]) < 0 })
	var x bytes.Buffer
	x.Write([]byte{0xff, 't', 'O', 'c'})
	_ = binary.Write(&x, binary.BigEndian, uint32(2))
	var fanout [256]uint32
	for _, id := range sorted {
		for b := int(id[0]); b < 256; b++ {
			fanout[b]++
		}
	}
	_ = binary.Write(&x, binary.BigEndian, fanout)
	for _, id := range sorted {
		x.Write(id[:])
	}
	for _, id := range sorted {
		_ = binary.Write(&x, binary.BigEndian, crcs[id])
	}
	for _, id := range sorted {
		_ = binary.Write(&x, binary.BigEndian, offsets[id])
	}
	x.Write(sum[:])
	isum := sha1.Sum(x.Bytes())
	x.Write(isum[:])
	return "pack-" + hex.EncodeToString(sum[:]), p.Bytes(), x.Bytes()
}

// reachable returns the objects reachable from a commit, commit first.
func (s *objectStore) reachable(c objID) []objID {
	seen := map[objID]bool{}
	var out []objID
	var walk func(id objID)
	walk = func(id objID) {
		if seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
		o := s.byID[id]
		switch o.kind {
		case "commit":
			for _, line := range strings.Split(string(o.data), "\n") {
				if k, v, ok := strings.Cut(line, " "); ok && (k == "tree" || k == "parent") {
					var id objID
					_, _ = hex.Decode(id[:], []byte(v))
					walk(id)
				} else if line == "" {
					break
				}
			}
		case "tree":
			d := o.data
			for len(d) > 0 {
				nul := bytes.IndexByte(d, 0)
				var id objID
				copy(id[:], d[nul+1:nul+21])
				walk(id)
				d = d[nul+21:]
			}
		}
	}
	walk(c)
	return out
}

// indexEntry is one worktree file in the git index.
type indexEntry struct {
	path  string
	id    objID
	size  int
	mtime time.Time
}

// gitIndex writes a version 2 index. ctime equals mtime, device and inode
// are 0: git then re-reads the files once and finds them unchanged.
func gitIndex(entries []indexEntry, uid uint32) []byte {
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	var b bytes.Buffer
	b.WriteString("DIRC")
	_ = binary.Write(&b, binary.BigEndian, uint32(2))
	_ = binary.Write(&b, binary.BigEndian, uint32(len(entries)))
	for _, e := range entries {
		start := b.Len()
		sec, nsec := uint32(e.mtime.Unix()), uint32(e.mtime.Nanosecond())
		for _, v := range []uint32{sec, nsec, sec, nsec, 0, 0, 0o100644, uid, uid, uint32(e.size)} {
			_ = binary.Write(&b, binary.BigEndian, v)
		}
		b.Write(e.id[:])
		n := len(e.path)
		if n > 0xfff {
			n = 0xfff
		}
		_ = binary.Write(&b, binary.BigEndian, uint16(n))
		b.WriteString(e.path)
		pad := 8 - (b.Len()-start)%8 // at least one NUL
		b.Write(make([]byte, pad))
	}
	sum := sha1.Sum(b.Bytes())
	b.Write(sum[:])
	return b.Bytes()
}

// relPath joins a path below the repo.
func relPath(repo, rel string) string { return path.Join(repo, rel) }
