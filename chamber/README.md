# chamber

Go components of the detonation chamber. See `docs/architecture.md`.

| Package | Role |
|---|---|
| `internal/config` | Sandbox and persona XML: types, loading, validation |
| `internal/assemble` | Builds the master tree of one run from a configuration |
| `internal/master` | Master tree: layer and group masters, event paths |
| `internal/facts` | Facts passed between layers, and their key names |
| `internal/tokensource` | Token values: generator, pool, credentials importer |
| `layers/honeytoken/awscreds` | AWS credentials honeytoken layer |
| `layers/network/aws` | AWS network layer: refused, generic, or proxy in front of Moto |
| `honeytoken` | Key generation used by the generator token source |
| `traplog` | Trap event logger (JSONL) |

Tests: `go test ./...`. The test against a real Moto runs when `MOTO_URL`
is set, e.g. `docker run -d -p 5000:5000 motoserver/moto` and
`MOTO_URL=http://localhost:5000`. Moto needs no account setting: the AWS
layer sends the run's account with every request.
