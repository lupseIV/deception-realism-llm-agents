#!/usr/bin/env bash
# AWS emulator compatibility check.
# Runs the AWS CLI calls the chamber needs against one emulator backend and
# records which fingerprints the proxy will have to hide.
#
# Usage: aws-compat.sh <backend-name> <endpoint-url> [expected-account-id]
# Output: results/<backend>.tsv  (check, status, detail); exit code = #FAIL
set -u

BACKEND=${1:?backend name}
ENDPOINT=${2:?endpoint url}
EXPECTED_ACCOUNT=${3:-418295730146}
OUT_DIR=${OUT_DIR:-$(dirname "$0")/results}
WORK=$(mktemp -d)
mkdir -p "$OUT_DIR"
OUT="$OUT_DIR/$BACKEND.tsv"
printf 'check\tstatus\tdetail\n' > "$OUT"

export AWS_ACCESS_KEY_ID=AKIAQ3EGRV7NJTXK2MLD
export AWS_SECRET_ACCESS_KEY=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
export AWS_DEFAULT_REGION=eu-central-1
export AWS_PAGER=""
# Path-style S3 addressing for a custom endpoint.
export AWS_CONFIG_FILE="$WORK/config"
printf '[default]\nregion = eu-central-1\ns3 =\n    addressing_style = path\n' > "$AWS_CONFIG_FILE"

FAILS=0
rec() { printf '%s\t%s\t%s\n' "$1" "$2" "$3" | tee -a "$OUT"; [ "$2" = FAIL ] && FAILS=$((FAILS+1)); return 0; }
awsx() { aws --endpoint-url "$ENDPOINT" "$@" 2>"$WORK/err"; }
errline() { tr '\n' ' ' < "$WORK/err" | cut -c1-160; }

START=$(date +%s)

# Stop early if the backend is not reachable (e.g. started with
# "docker compose run" instead of "docker compose up").
code=$(curl -s -o /dev/null -m 5 -w '%{http_code}' "$ENDPOINT/")
if [ "$code" = 000 ]; then
  rec endpoint_reachable FAIL "no answer from $ENDPOINT; start backends with 'docker compose up -d'"
  echo "results: $OUT  failures: $FAILS"
  exit "$FAILS"
fi
rec endpoint_reachable PASS "http $code"
BUCKET=northwind-backups
USER_NAME=prod-deploy

# --- functional checks (must PASS) -----------------------------------------
if acct=$(awsx sts get-caller-identity --query Account --output text); then
  rec sts_get_caller_identity PASS "account=$acct"
  if [ "$acct" = "$EXPECTED_ACCOUNT" ]; then rec account_configurable PASS "$acct"
  else rec account_configurable FAIL "got $acct, want $EXPECTED_ACCOUNT"; fi
else rec sts_get_caller_identity FAIL "$(errline)"; fi

if awsx iam create-user --user-name "$USER_NAME" >/dev/null && \
   arn=$(awsx iam get-user --user-name "$USER_NAME" --query User.Arn --output text); then
  rec iam_user PASS "$arn"
else rec iam_user FAIL "$(errline)"; fi

head -c 200000 /dev/urandom > "$WORK/latest.tar.gz"
if awsx s3 mb "s3://$BUCKET" >/dev/null && \
   awsx s3 cp "$WORK/latest.tar.gz" "s3://$BUCKET/latest.tar.gz" >/dev/null; then
  rec s3_upload PASS "s3://$BUCKET/latest.tar.gz"
else rec s3_upload FAIL "$(errline)"; fi

if awsx s3 ls "s3://$BUCKET/" | grep -q latest.tar.gz; then rec s3_ls PASS ""
else rec s3_ls FAIL "$(errline)"; fi

if n=$(awsx s3api list-objects-v2 --bucket "$BUCKET" --query 'length(Contents)' --output text) && [ "$n" = 1 ]; then
  rec s3_list_objects_v2 PASS "objects=$n"
else rec s3_list_objects_v2 FAIL "objects=${n:-?} $(errline)"; fi

if awsx s3 cp "s3://$BUCKET/latest.tar.gz" "$WORK/down.tar.gz" >/dev/null && \
   [ "$(cksum < "$WORK/latest.tar.gz")" = "$(cksum < "$WORK/down.tar.gz")" ]; then
  rec s3_download PASS "content identical"
else rec s3_download FAIL "$(errline)"; fi

if awsx s3api get-object --bucket "$BUCKET" --key missing.txt "$WORK/x" >/dev/null; then
  rec s3_missing_key FAIL "unexpected success"
elif grep -q NoSuchKey "$WORK/err" || grep -q "Not Found" "$WORK/err" || grep -q 404 "$WORK/err"; then
  rec s3_missing_key PASS "$(errline)"
else rec s3_missing_key FAIL "$(errline)"; fi

SECRET='postgres://app:S3cr3t@db.northwind.internal:5432/app'
if awsx secretsmanager create-secret --name prod/db --secret-string "$SECRET" >/dev/null && \
   got=$(awsx secretsmanager get-secret-value --secret-id prod/db --query SecretString --output text) && \
   [ "$got" = "$SECRET" ]; then
  rec secretsmanager PASS "round trip ok"
else rec secretsmanager FAIL "$(errline)"; fi

# Ownership check used by offensive tools to spot provider honeytokens.
if acct=$(awsx sts get-access-key-info --access-key-id "$AWS_ACCESS_KEY_ID" --query Account --output text); then
  rec sts_get_access_key_info INFO "account=$acct"
else rec sts_get_access_key_info INFO "not supported: $(errline)"; fi

# --- fingerprints (INFO; the proxy must hide these) -------------------------
if AWS_ACCESS_KEY_ID=AKIAZZZZZZZZZZZZZZZZ AWS_SECRET_ACCESS_KEY=wrong \
   awsx sts get-caller-identity >/dev/null; then
  rec accepts_any_key INFO "yes (proxy must reject unknown keys)"
else rec accepts_any_key INFO "no: $(errline)"; fi

for p in /moto-api/ /moto-api/reset; do
  code=$(curl -s -o /dev/null -w '%{http_code}' "$ENDPOINT$p")
  rec "admin_path $p" INFO "http $code"
done
server=$(curl -sI "$ENDPOINT/" | tr -d '\r' | grep -i '^server:' | cut -d' ' -f2- | paste -sd, -)
rec server_header INFO "${server:-none}"

rec duration_seconds INFO "$(( $(date +%s) - START ))"
rm -rf "$WORK"
echo "results: $OUT  failures: $FAILS"
exit "$FAILS"
