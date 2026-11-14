#!/usr/bin/env bash
# Validate personas and sandbox configurations against the XSD schemas, and
# check that the invalid examples in schema/testdata are rejected.
# Uses xmllint if installed, otherwise runs itself in an Alpine container.
set -u
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

if ! command -v xmllint >/dev/null 2>&1; then
  if [ "${IN_CONTAINER:-}" = 1 ]; then echo "xmllint missing"; exit 2; fi
  HOST_ROOT=$ROOT
  command -v cygpath >/dev/null 2>&1 && HOST_ROOT=$(cygpath -w "$ROOT")   # Git Bash on Windows
  echo "xmllint not found, running in Docker (alpine)"
  MSYS_NO_PATHCONV=1 exec docker run --rm -e IN_CONTAINER=1 -v "$HOST_ROOT:/w" -w /w alpine:3 \
    sh -c 'apk add -q --no-cache bash libxml2-utils >/dev/null && bash schema/check.sh'
fi

fails=0
valid() {   # schema file...
  local xsd=$1; shift
  for f in "$@"; do
    if xmllint --noout --schema "$xsd" "$f" 2>/tmp/xmllint.err; then
      echo "ok       $f"
    else
      echo "INVALID  $f"; sed 's/^/         /' /tmp/xmllint.err; fails=$((fails+1))
    fi
  done
}
invalid() { # schema file...
  local xsd=$1; shift
  for f in "$@"; do
    if xmllint --noout --schema "$xsd" "$f" >/dev/null 2>&1; then
      echo "ACCEPTED $f (expected rejection)"; fails=$((fails+1))
    else
      echo "rejected $f"
    fi
  done
}

valid   schema/persona.xsd personas/*.xml
valid   schema/sandbox.xsd configs/*/*.xml
invalid schema/persona.xsd schema/testdata/persona-invalid-*.xml
invalid schema/sandbox.xsd schema/testdata/sandbox-invalid-*.xml

echo "problems: $fails"
exit "$fails"
