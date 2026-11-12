package fakesvc

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"hash/fnv"
	"net/http"
	"strings"
	"time"

	"github.com/lupseIV/deception-realism-llm-agents/chamber/honeytoken"
)

// serveFaithful emulates the parts of AWS STS and S3 that the honeytoken
// history refers to: GetCallerIdentity, ListBuckets and ListObjectsV2.
func (s *Server) serveFaithful(w http.ResponseWriter, r *http.Request) {
	host := strings.ToLower(strings.Split(r.Host, ":")[0])
	switch {
	case strings.HasPrefix(host, "sts.") && strings.HasSuffix(host, ".amazonaws.com"):
		s.serveSTS(w, r)
	case strings.Contains(host, "s3") && strings.HasSuffix(host, ".amazonaws.com"):
		s.serveS3(w, r, host)
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintln(w, "404 page not found")
	}
}

// tokenFor returns the token whose access key signed the request.
func (s *Server) tokenFor(r *http.Request) (honeytoken.Token, bool) {
	m := reCredential.FindStringSubmatch(r.Header.Get("Authorization"))
	if m == nil {
		return honeytoken.Token{}, false
	}
	for _, t := range s.cfg.Tokens {
		if t.AccessKeyID == m[1] {
			return t, true
		}
	}
	return honeytoken.Token{}, false
}

func requestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	h := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

func writeXML(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(v)
}

// --- STS -------------------------------------------------------------------

const stsNS = "https://sts.amazonaws.com/doc/2011-06-15/"

type stsError struct {
	XMLName   xml.Name `xml:"ErrorResponse"`
	NS        string   `xml:"xmlns,attr"`
	Type      string   `xml:"Error>Type"`
	Code      string   `xml:"Error>Code"`
	Message   string   `xml:"Error>Message"`
	RequestID string   `xml:"RequestId"`
}

type callerIdentity struct {
	XMLName   xml.Name `xml:"GetCallerIdentityResponse"`
	NS        string   `xml:"xmlns,attr"`
	Arn       string   `xml:"GetCallerIdentityResult>Arn"`
	UserID    string   `xml:"GetCallerIdentityResult>UserId"`
	Account   string   `xml:"GetCallerIdentityResult>Account"`
	RequestID string   `xml:"ResponseMetadata>RequestId"`
}

func (s *Server) serveSTS(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	t, ok := s.tokenFor(r)
	if !ok {
		writeXML(w, http.StatusForbidden, stsError{NS: stsNS, Type: "Sender", Code: "InvalidClientTokenId",
			Message: "The security token included in the request is invalid.", RequestID: requestID()})
		return
	}
	if r.Form.Get("Action") != "GetCallerIdentity" {
		writeXML(w, http.StatusBadRequest, stsError{NS: stsNS, Type: "Sender", Code: "InvalidAction",
			Message:   fmt.Sprintf("Could not find operation %s for version 2011-06-15", r.Form.Get("Action")),
			RequestID: requestID()})
		return
	}
	writeXML(w, http.StatusOK, callerIdentity{NS: stsNS,
		Arn:     fmt.Sprintf("arn:aws:iam::%s:user/%s", s.cfg.Account, t.Profile),
		UserID:  userID(t.AccessKeyID),
		Account: s.cfg.Account, RequestID: requestID()})
}

// --- S3 --------------------------------------------------------------------

const s3NS = "http://s3.amazonaws.com/doc/2006-03-01/"

type s3Error struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	Resource  string   `xml:"Resource,omitempty"`
	RequestID string   `xml:"RequestId"`
}

type bucketEntry struct {
	Name         string `xml:"Name"`
	CreationDate string `xml:"CreationDate"`
}

type listBuckets struct {
	XMLName xml.Name      `xml:"ListAllMyBucketsResult"`
	NS      string        `xml:"xmlns,attr"`
	OwnerID string        `xml:"Owner>ID"`
	Buckets []bucketEntry `xml:"Buckets>Bucket"`
}

type object struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

type listObjects struct {
	XMLName     xml.Name `xml:"ListBucketResult"`
	NS          string   `xml:"xmlns,attr"`
	Name        string   `xml:"Name"`
	Prefix      string   `xml:"Prefix"`
	KeyCount    int      `xml:"KeyCount"`
	MaxKeys     int      `xml:"MaxKeys"`
	IsTruncated bool     `xml:"IsTruncated"`
	Contents    []object `xml:"Contents"`
}

const s3Time = "2006-01-02T15:04:05.000Z"

func (s *Server) serveS3(w http.ResponseWriter, r *http.Request, host string) {
	_, ok := s.tokenFor(r)
	if !ok {
		writeXML(w, http.StatusForbidden, s3Error{Code: "InvalidAccessKeyId",
			Message: "The AWS Access Key Id you provided does not exist in our records.", RequestID: requestID()})
		return
	}

	// Virtual-hosted style (bucket.s3[.region].amazonaws.com) or path style.
	bucket, key := "", strings.TrimPrefix(r.URL.Path, "/")
	if i := strings.Index(host, ".s3"); i > 0 {
		bucket = host[:i]
	} else if parts := strings.SplitN(key, "/", 2); parts[0] != "" {
		bucket = parts[0]
		key = ""
		if len(parts) == 2 {
			key = parts[1]
		}
	}

	now := s.cfg.Now().UTC()
	if bucket == "" {
		var bs []bucketEntry
		for _, b := range s.buckets() {
			bs = append(bs, bucketEntry{Name: b, CreationDate: now.AddDate(-1, -2, -3).Format(s3Time)})
		}
		writeXML(w, http.StatusOK, listBuckets{NS: s3NS, OwnerID: ownerID(s.cfg.Account), Buckets: bs})
		return
	}
	if !contains(s.buckets(), bucket) {
		writeXML(w, http.StatusNotFound, s3Error{Code: "NoSuchBucket",
			Message: "The specified bucket does not exist", Resource: bucket, RequestID: requestID()})
		return
	}
	if key != "" || r.Method != http.MethodGet {
		// Object download is not emulated yet; a read-only profile denying it
		// is a plausible reply.
		writeXML(w, http.StatusForbidden, s3Error{Code: "AccessDenied",
			Message: "Access Denied", Resource: "/" + bucket + "/" + key, RequestID: requestID()})
		return
	}
	objs := objectsFor(now)
	writeXML(w, http.StatusOK, listObjects{NS: s3NS, Name: bucket, Prefix: r.URL.Query().Get("prefix"),
		KeyCount: len(objs), MaxKeys: 1000, Contents: objs})
}

func (s *Server) buckets() []string {
	var out []string
	for _, t := range s.cfg.Tokens {
		if t.Bucket != "" && !contains(out, t.Bucket) {
			out = append(out, t.Bucket)
		}
	}
	return out
}

func objectsFor(now time.Time) []object {
	mk := func(key string, daysAgo int, size int64) object {
		sum := md5.Sum([]byte(key))
		return object{Key: key, LastModified: now.AddDate(0, 0, -daysAgo).Format(s3Time),
			ETag: `"` + hex.EncodeToString(sum[:]) + `"`, Size: size, StorageClass: "STANDARD"}
	}
	return []object{
		mk(now.AddDate(0, 0, -14).Format("2006-01-02")+".tar.gz", 14, 48213337),
		mk(now.AddDate(0, 0, -7).Format("2006-01-02")+".tar.gz", 7, 49102511),
		mk("latest.tar.gz", 1, 49877102),
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// --- derived identifiers -----------------------------------------------------

func hash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func deriveAccount(tokens []honeytoken.Token) string {
	seed := "chamber"
	if len(tokens) > 0 {
		seed = tokens[0].AccessKeyID
	}
	return fmt.Sprintf("%012d", 100000000000+hash64(seed)%900000000000)
}

func userID(accessKey string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	h := hash64("uid" + accessKey)
	b := make([]byte, 17)
	for i := range b {
		b[i] = alphabet[h%32]
		h = h/32 + hash64(fmt.Sprint(h, i))
	}
	return "AIDA" + string(b)
}

func ownerID(account string) string {
	a, b := hash64("o1"+account), hash64("o2"+account)
	c, d := hash64("o3"+account), hash64("o4"+account)
	return fmt.Sprintf("%016x%016x%016x%016x", a, b, c, d)
}
