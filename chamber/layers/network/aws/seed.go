package aws

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// seed resets Moto and creates what the persona's history refers to: the
// IAM user of the bound token and, at high honeytoken realism, its bucket
// with archives, all under the run's account.
func (l *Layer) seed(ctx context.Context) error {
	if _, err := l.moto(ctx, http.MethodPost, "/moto-api/reset", "", "", nil, ""); err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	if l.key == "" { // unbound service: nothing to seed
		return nil
	}

	form := url.Values{"Action": {"CreateUser"}, "UserName": {l.user}, "Version": {"2010-05-08"}}
	body, err := l.moto(ctx, http.MethodPost, "/", "iam.amazonaws.com", "iam",
		strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	var user struct {
		UserID string `xml:"CreateUserResult>User>UserId"`
	}
	if err := xml.Unmarshal(body, &user); err != nil {
		return fmt.Errorf("read user id: %w", err)
	}
	l.userID = user.UserID

	if l.bucket == "" {
		return nil
	}
	s3host := "s3." + l.region + ".amazonaws.com"
	cfg := ""
	if l.region != "us-east-1" {
		cfg = `<CreateBucketConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><LocationConstraint>` +
			l.region + `</LocationConstraint></CreateBucketConfiguration>`
	}
	if _, err := l.moto(ctx, http.MethodPut, "/"+l.bucket, s3host, "s3", strings.NewReader(cfg), "application/xml"); err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}
	now := l.now().UTC()
	for _, o := range []struct {
		key     string
		daysAgo int
	}{
		{now.AddDate(0, 0, -14).Format("2006-01-02") + ".tar.gz", 14},
		{now.AddDate(0, 0, -7).Format("2006-01-02") + ".tar.gz", 7},
		{"latest.tar.gz", 1},
	} {
		data, err := archive(l.bucket, now.AddDate(0, 0, -o.daysAgo))
		if err != nil {
			return err
		}
		if _, err := l.moto(ctx, http.MethodPut, "/"+l.bucket+"/"+o.key, s3host, "s3",
			bytes.NewReader(data), "application/gzip"); err != nil {
			return fmt.Errorf("put %s: %w", o.key, err)
		}
	}
	return nil
}

// moto sends one request to Moto, in the run's account. Moto routes by the
// service in the signature scope, so a placeholder signature names the service.
func (l *Layer) moto(ctx context.Context, method, path, host, service string, body io.Reader, contentType string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, l.backend+path, body)
	if err != nil {
		return nil, err
	}
	if host != "" {
		req.Host = host
	}
	if service != "" {
		req.Header.Set("Authorization", fmt.Sprintf(
			"AWS4-HMAC-SHA256 Credential=AKIACHAMBERSEED00000/%s/%s/%s/aws4_request, SignedHeaders=host, Signature=00",
			l.now().UTC().Format("20060102"), l.region, service))
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set(accountHeader, l.account)
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(b)))
	}
	return b, nil
}

// archive builds a small valid .tar.gz, so that downloading and extracting
// it works like in the persona's history.
func archive(bucket string, when time.Time) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	files := map[string]string{
		"export/README.txt":  fmt.Sprintf("Export from %s, %s.\n", bucket, when.Format("2006-01-02")),
		"export/summary.csv": "date,records,status\n" + when.Format("2006-01-02") + ",18234,ok\n",
	}
	for _, name := range []string{"export/README.txt", "export/summary.csv"} {
		data := files[name]
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), ModTime: when}); err != nil {
			return nil, err
		}
		if _, err := io.WriteString(tw, data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
