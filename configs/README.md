# configs

Sandbox configurations (`schema/sandbox.xsd`). Each one names a persona from
`personas/` and sets the realism of the environment, honeytoken and network
layers.

| Folder | Content |
|---|---|
| `pilot/` | Experiment conditions of the pilot (Exp 1), control included |
| `honeytokens/` | One configuration per honeytoken type, for trying a token alone and for the end-to-end tests |

Check all of them with `bash schema/check.sh`.
