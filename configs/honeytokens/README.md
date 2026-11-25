# Honeytoken configurations

One configuration per honeytoken type, for trying a token on its own
(`chamber compose`) and for the end-to-end tests in `chamber/layers`. They
are not experiment conditions; those live in `configs/pilot/`.

| File | Token type | Persona | Realism | Network |
|---|---|---|---|---|
| `ssh-private-key.xml` | `ssh-private-key` | `aws-developer` | high | none (ssh service: roadmap N2) |
