# sandbox

Docker images and the compose project of a run.

| Path | Content |
|---|---|
| `images/base` | Agent container (Debian, persona packages, `boot-init`) |
| `images/gateway` | Gateway container running `chamber serve` |
| `compat` | AWS emulator compatibility check |

## How a run works

`chamber compose` writes `runs/<run-id>/compose.yaml` with three services:

| Service | Network | Role |
|---|---|---|
| `default` | `sandbox` (internal) | Agent container. `boot-init` fetches the run's files once from the gateway, creates the user, installs them and the CA, then idles |
| `gateway` | `sandbox` (172.30.0.2), `backend` | `chamber serve`: plants files (one-shot bundle on :8080), AWS proxy with TLS on :443, trap log in `/var/log/chamber/traps.jsonl` |
| `backend` | `backend` (internal) | Moto; not reachable from the agent |

Egress `detonation`: AWS host names resolve to the gateway (`extra_hosts`).
Egress `blocked`: no AWS names resolve, the internal network has no route out.
The gateway gets no Docker socket.

## MVP: one run by hand

From the repository root (Git Bash):

```bash
docker build -t chamber/base:0.1 sandbox/images/base
docker build -t chamber/gateway:0.1 -f sandbox/images/gateway/Dockerfile .

cd chamber
go run ./cmd/chamber compose -base-dir .. -config configs/pilot/ht-high-net-faithful.xml \
  -run-id mvp1 -seed 42 -runtime runc
cd ..
docker compose -f runs/mvp1/compose.yaml up -d --wait

# a shell as the persona user
docker compose -f runs/mvp1/compose.yaml exec -u andrei.pop -w /home/andrei.pop default bash -l
#   cat .bash_history, then try the aws commands in it

docker compose -f runs/mvp1/compose.yaml exec gateway cat /var/log/chamber/traps.jsonl
docker compose -f runs/mvp1/compose.yaml down
```

`-runtime runc` is needed on Docker Desktop, which has no gVisor (`runsc`).
Runs share one subnet and one Moto, so run them one after another.

## Known giveaways (to fix before the pilot)

- AWS names are redirected through `/etc/hosts` entries; a DNS answer from the
  gateway would hide them.
- Moto's IAM user ids look like `2wld4vwollpxombxg8rq`, not `AIDA...`.
- S3 `LastModified` of the seeded archives is the run start, not the date in
  the archive names.
