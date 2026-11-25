# chamber

Go components of the detonation chamber. See `docs/architecture.md` for the
design and `docs/roadmap.md` for the planned layers.

| Path | Role |
|---|---|
| `cmd/chamber` | CLI, flags only: `compose` (per-run compose project), `serve` (gateway), `facts` (read by the harness) |
| `internal/app` | Use cases: prepare a run, write its compose project, serve it |
| `internal/config` | Sandbox and persona XML: types, loading, validation |
| `internal/assemble` | Builds the master tree of one run from a configuration |
| `internal/master` | Master tree: files go down as data, events go up through `emit` |
| `internal/helpers` | Seeded randomness, timestamps, `~` paths shared by the layers |
| `internal/facts` | Facts passed between layers, and their key names |
| `internal/gateway` | Runtime: planted files, file modes, listener, setup bundle |
| `internal/tlsca` | CA and certificates for the fake TLS services |
| `internal/openssh` | OpenSSH key files, fingerprints, hashed known_hosts lines |
| `internal/tokensource` | Token values: generator, pool, credentials importer |
| `internal/tokensource/awskey` | AWS key pair generation used by the generator |
| `layers/environment/persona` | Baseline home of the persona (every condition, control included) |
| `layers/honeytoken/awscreds` | `aws-credentials` honeytoken layer |
| `layers/honeytoken/sshkey` | `ssh-private-key` honeytoken layer |
| `layers/network/aws` | AWS network layer: refused, generic, or proxy in front of Moto |
| `wear` | Embedded text store of the planted home, per realism level |
| `internal/traplog` | Trap event logger (JSONL); counts events it could not write |

Layer packages are named after the token or service type without hyphens
(`sshkey` is type `ssh-private-key`); each exports `Kind`, the factory key,
and is registered in `layers/factory.go`.

Tests: `go test ./...`. The test against a real Moto runs when `MOTO_URL`
is set, e.g. `docker run -d -p 5000:5000 motoserver/moto` and
`MOTO_URL=http://localhost:5000`. Moto needs no account setting: the AWS
layer sends the run's account with every request.
