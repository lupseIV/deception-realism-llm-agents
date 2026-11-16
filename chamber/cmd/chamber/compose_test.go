package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func compose(t *testing.T, cfg string) string {
	t.Helper()
	dir := t.TempDir()
	if err := cmdCompose([]string{"-base-dir", "../../..", "-config", cfg, "-run-id", "T1", "-seed", "42",
		"-runtime", "runc", "-out", dir}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestComposeDetonation(t *testing.T) {
	y := compose(t, "configs/pilot/ht-high-net-faithful.xml")
	for _, want := range []string{
		`name: "chamber-t1"`,
		`hostname: "vt-dev-07"`,
		`"sts.amazonaws.com:172.30.0.2"`,
		`"s3.eu-central-1.amazonaws.com:172.30.0.2"`,
		`.s3.eu-central-1.amazonaws.com:172.30.0.2"`, // the bucket of the planted history
		`"serve", "-config", "configs/pilot/ht-high-net-faithful.xml", "-run-id", "T1"`,
		`"-seed", "42"`,
		"internal: true",
	} {
		if !strings.Contains(y, want) {
			t.Errorf("missing %s in\n%s", want, y)
		}
	}
	if strings.Contains(y, "runtime:") {
		t.Error("runc must use Docker's default runtime")
	}
}

func TestComposeBlockedHasNoRedirect(t *testing.T) {
	y := compose(t, "configs/pilot/control.xml")
	if strings.Contains(y, "extra_hosts") {
		t.Errorf("blocked egress must not redirect AWS names:\n%s", y)
	}
}
