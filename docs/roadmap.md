# Roadmap: honeytokens, personas, network deception, wear and tear

One step = one commit. Each step is done when `go vet ./...`, `go test ./...`
and `schema/check.sh` pass and the docs it touches are updated. Steps marked
**core** are needed for the thesis; the others extend the chamber and can be
dropped without changing Exp 1-3, which stay on the AWS pair.

Phase order follows the dependencies: tokens first, so personas can place them
and network services can bind to them; wear last, because it fills the medium
and high environment levels.

## Phase H - Honeytokens

Every token step adds: the type to `TokenType`, `supportedTokenTypes` and
`sandbox.xsd`; a layer under `layers/honeytoken/<type>/` registered in
`layers/factory.go`; generator support in `tokensource`; the match strings for
the detectors; tests for all three levels.

Levels, as for `aws-credentials`:
low = obviously fake (known example values, "test" markers);
medium = valid format, no context;
high = unique per run, persona context, history lines, past timestamps.

| Step | Token type | Planted as | Honeyquest honeywire | Binds to | |
|---|---|---|---|---|---|
| H1 | `ssh-private-key` | `~/.ssh/id_ed25519` + `.pub`, `Host` entry in `~/.ssh/config`, `known_hosts` line | filesystem-private-key | ssh, sftp | core, done |
| H2 | `db-credentials` | `~/work/<repo>/.env`, `~/.pgpass` | filesystem-config | postgres | core |
| H3 | `ftp-credentials` | `~/.netrc`, FileZilla `sitemanager.xml` | filesystem-config | ftp | core |
| H4 | `telnet-credentials` | device notes file, expect script in `~/bin` | filesystem-passwords | telnet | core |
| H5 | `api-token` | `.env` API key, `curl -H` line in history | httpheaders-devtoken | http | core |
| H6 | `hf-token` | `~/.cache/huggingface/token` | - | http | |
| H7 | `smb-credentials` | `~/.smbcredentials`, cifs line in a mount script | filesystem-config | smb | |
| H8 | `rdp-credentials` | `.rdp` file, Remmina profile | filesystem-config | rdp | |
| H9 | `kubeconfig` | `~/.kube/config` | risky: k8s-manifests | http | |
| H10 | `keys-json` | `keys.json` in a project folder | filesystem-keys | http | |
| H11 | `passwords-file` | `passwords.txt` / `logins.csv` | filesystem-passwords | any bound service | |
| H12 | `backup-archive` | `backup-<date>.tar.gz` with a config inside | filesystem-backup | - | |
| H13 | `customer-list` | `customers.csv` export | filesystem-customer-list | - | |

## Phase P - Personas

Every persona step adds `personas/<id>.xml` (identity, packages, folders,
`token-affinity`), its wear text and project, a sandbox config under
`configs/`, and tests that the control home is unchanged by token layers.

| Step | Persona | Token affinity | Services it talks to | |
|---|---|---|---|---|
| P0 | (refactor) wear store keyed by persona: `wear/<level>/<persona>/project`, per-persona ssh hosts | - | - | core |
| P1 | `server-host` (Linux web and backup server admin) | ssh-private-key, ftp-credentials, db-credentials, backup-archive | ssh, sftp, ftp, postgres | core |
| P2 | `ml-engineer` | hf-token, api-token, keys-json | http | core |
| P3 | `network-admin` (routers, cameras, IoT gateways) | telnet-credentials, passwords-file | telnet, ssh | |
| P4 | `it-admin` (mixed Linux and Windows office network) | smb-credentials, rdp-credentials, passwords-file | smb, rdp | |

`aws-developer` exists; it gains `db-credentials` affinity in H2.

## Phase N - Network deception

Every service step adds the type to `NetworkServiceType`,
`supportedServiceTypes` and `sandbox.xsd`; a layer under
`layers/network/<type>/` with the three modes; the `bind` check against the
token type; tests per mode.

Modes: refused = accept, log `connect`, reset; generic = protocol banner,
every login denied; faithful = protocol-correct, accepts only the bound token.

| Step | Service | Ports | Faithful mode | |
|---|---|---|---|---|
| N0 | (base) shared TCP service: listener, `connect` logging, refused and generic modes; `traplog` gains `auth`, `command`, `protocol`, `username` | - | - | core |
| N1 | `dns` | 53/udp, 53/tcp | resolves names to the fake services in detonation mode, logs queries | core |
| N2 | `ssh` | 22 | key auth with the planted key, fake shell, command log (`golang.org/x/crypto/ssh`) | core |
| N3 | `sftp` | 22 (subsystem) | listing and download of planted files (`pkg/sftp`) | core |
| N4 | `ftp` | 21 | login with `ftp-credentials`, LIST, RETR | core |
| N5 | `telnet` | 23, 2323 | BusyBox-style login and shell | core |
| N6 | `http` | 80, 443, 8080 | API answering the bound `api-token`; Honeyquest httpheaders and networkrequests honeywires as responses | core |
| N7 | `postgres` | 5432 | startup and password auth with `db-credentials`, small seeded schema | |
| N8 | `smb` | 445 | generic only: SMB2 negotiate, session setup denied, credentials logged | |
| N9 | `rdp` | 3389 | generic only: X.224 connection confirm, NLA denied, user logged | |
| N10 | `ntp` | 123/udp | client time replies only; no `monlist` or other amplifying commands | |

## Phase W - Wear and tear

Every wear step fills `wear/medium/` or `wear/high/` for one kind, with the
planting code in `layers/environment/persona` and a test that no credential
appears (the rule in `wear/README.md`).

| Step | Wear | Level | |
|---|---|---|---|
| W1 | git hook samples in `.git/hooks` (known gap) | low | core |
| W2 | `~/Downloads` content per persona (`count`, `types` from the persona XML) | medium | core |
| W3 | package and tool caches: `~/.cache/pip`, `~/.npm/_logs`, `~/.cache/go-build` sizes | medium | |
| W4 | crontab and systemd user units | medium | |
| W5 | tmux config, vim swap and undo files | medium | |
| W6 | `~/.docker/config.json` (no auths) and image list in history | medium | |
| W7 | system logs: `/var/log/auth.log`, `apt/history.log`, `dpkg.log` aged by `age-days` | high | core |
| W8 | AI-tool artifacts: Ollama model manifests, assistant chat history files | high | |
| W9 | longer, timestamped shell history (`HISTTIMEFORMAT`), aging of all files by `age-days` | high | |
| W10 | trash and recently used files | high | |

## Sources

- Honeyquest query database (ODbL; queries mostly CC-BY-4.0). Imported
  content keeps its `payload/author` and `payload/license` fields; cite
  Kahlhofer et al., RAID 2024.
- Not in the paper's reference list [1]-[15]: protocol scan rankings
  (Franco et al., 2021). Cite only if added to the bibliography.
