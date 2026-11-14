# AWS emulator compatibility check

Checks whether the AWS emulator (Moto) supports the calls the chamber needs and
which fingerprints the chamber proxy must hide.

## Run (Windows, PowerShell, Docker Desktop)

Start Moto, either from VS Code (Docker extension: right-click
`docker-compose.yml` > Compose Up, or the "Run All Services" link above
`services:`) or from a terminal:

```powershell
cd sandbox\compat
docker compose up -d
```

Moto is then on http://localhost:5000. The `check` service is in the `check`
profile, so it is not started by Compose Up. Run it with:

```powershell
docker compose run --rm check moto http://moto:5000
docker compose down
```

Run `docker compose down` (or Compose Down in VS Code) before repeating a
check, so Moto starts with empty state. Results are written to
`results/moto.tsv`.

Record the exact images used, for reproducibility:

```powershell
docker image inspect --format "{{index .RepoDigests 0}}" motoserver/moto:latest
```

## Checks

| Check | Expected | Meaning |
|---|---|---|
| `endpoint_reachable` | PASS | Moto answers; if FAIL, the script stops (start Moto with `docker compose up -d`, not `run`) |
| `sts_get_caller_identity` | PASS | STS works |
| `account_configurable` | PASS | account id can be set per run (no default `000000000000` / `123456789012`) |
| `iam_user` | PASS | IAM user for the persona |
| `s3_upload`, `s3_ls`, `s3_list_objects_v2`, `s3_download` | PASS | S3 round trip, object download works |
| `s3_missing_key` | PASS | realistic `NoSuchKey` error |
| `secretsmanager` | PASS | secret round trip |
| `sts_get_access_key_info` | INFO | answer to the ownership check used to spot provider honeytokens |
| `accepts_any_key` | INFO | if yes, the proxy must reject keys that are not honeytokens |
| `admin_path ...` | INFO | admin endpoints reachable; the proxy must block them |
| `server_header` | INFO | server banner; the proxy must replace it |
| `duration_seconds` | INFO | total run time of the check |

The exit code is the number of failed checks.

## Second phase

After the chamber proxy forwards to Moto (implementation step 8), run
the same script against the proxy with the honeytoken credentials. Expected
then: `accepts_any_key` = no, admin paths 404, AWS-like server header.

## Reference result

`motoserver/moto:latest`, run on 2026-11-14 on Windows with Docker Desktop.
Raw output in `results/moto.tsv`.

- All functional checks PASS (STS, account id, IAM, S3 round trip,
  `NoSuchKey`, Secrets Manager).
- `sts get-access-key-info` returns the configured account.
- Any key is accepted by default.
- `/moto-api/` is reachable (200).
- Server header `Werkzeug/3.1.9 Python/3.13.18`.
- The check takes about 9 s.

Consequences for the proxy: reject keys that are not honeytokens, answer STS
`GetCallerIdentity` and `GetAccessKeyInfo` itself with the persona account,
block `/moto-api/`, and set an AWS-like server header.

Earlier pip check (`moto_server`): with `INITIAL_NO_AUTH_ACTION_COUNT=0`
unknown keys get `InvalidClientTokenId` (STS) and `InvalidAccessKeyId` (S3);
startup about 1.6 s, memory about 54 MiB idle, 79 MiB after a few calls.
