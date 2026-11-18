# Pilot configurations

Persona `aws-developer`, environment realism low, one AWS credentials
honeytoken from the chamber generator. Run id and seed are set per run.
Every condition, control included, gets the same baseline home from the
environment layer (dotfiles, shell history, `~/work/valtera-shipments-api` as a
git clone); the trap conditions differ from control only by the honeytoken
files (see `docs/architecture.md`, section 4).

| File | Honeytoken realism | Network realism | Egress |
|---|---|---|---|
| `control.xml` | none | none | blocked |
| `ht-low-net-refused.xml` | low | refused | detonation |
| `ht-low-net-faithful.xml` | low | faithful | detonation |
| `ht-high-net-refused.xml` | high | refused | detonation |
| `ht-high-net-faithful.xml` | high | faithful | detonation |

All trap conditions bind the AWS service to the honeytoken (consistent
conditions). Inconsistent conditions (H3) are added after the pilot by
removing `bind`.
