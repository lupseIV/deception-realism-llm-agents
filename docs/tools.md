# Third-party tools

Software and datasets used or considered by the chamber. These are not
literature references; in the report they are named as software with a link.
"Verified" means the project page was checked on the given date; other entries
come from a tool survey and must be checked before use.

## In use or planned for Lab5-8

| Tool | Link | Role | License | Status |
|---|---|---|---|---|
| Moto | https://github.com/getmoto/moto | AWS emulator behind the chamber proxy | Apache-2.0 | Verified 2026-11-14: `MOTO_ACCOUNT_ID`, `INITIAL_NO_AUTH_ACTION_COUNT`, compat check passed |
| Thinkst Canarytokens | https://github.com/thinkst/canarytokens | Provider-issued tokens imported into the `pool` source (low realism tier) | BSD-3-Clause | Verified 2026-11-14 |
| Inspect AI | https://github.com/UKGovernmentBEIS/inspect_ai | Agent harness, `react()` agent | MIT | Project decision |
| gVisor | https://github.com/google/gvisor | Isolation runtime (`runsc`); runtime monitoring to be checked for file-read detection | Apache-2.0 | Runtime monitoring not verified yet |

## Candidates for environment medium/high realism and further layers

| Tool | Link | Role | Status |
|---|---|---|---|
| TeamShift fake-business | https://teamshift.io/tools/fake-business | Seeded synthetic small business (CRM, invoices, emails, timeline); Apache-2.0, `npx @teamshift/fake-business --seed N` | Verified 2026-11-14; 3 industries only |
| Sinabis 10k synthetic emails | https://huggingface.co/datasets/sinabis-group/sinabis-10k-synthetic-emails | `.eml`/`.ics` corpus of a fictional energy company; CC BY 4.0; domain `sinabis-oil.com` must be rewritten | Verified 2026-11-14 (10k edition only) |
| NexaSoft email corpus | https://www.mdpi.com/2076-3417/16/22/11224 | Synthetic enterprise email corpus (paper, Nov 2026) | Paper verified; data location not confirmed |
| libfaketime | https://github.com/wolfcw/libfaketime | Build-time aging of timestamps; must be removed before the agent starts | Not verified |
| Loghub | https://github.com/logpai/loghub | Real system logs for `/var/log` | Not verified |
| Flog | https://github.com/mingrammer/flog | Generated web/syslog logs | Not verified |
| faker-file | https://github.com/barseghyanartur/faker-file | Office documents with embedded tokens | Not verified |
| Pagila / test_db | https://github.com/devrimgunduz/pagila, https://github.com/datacharmer/test_db | Database content for the server-host persona | Not verified |
| Mailpit | https://github.com/axllent/mailpit | Mail network layer | Not verified |
| GLAuth | https://github.com/glauth/glauth | LDAP network layer | Not verified |
| KWOK | https://github.com/kubernetes-sigs/kwok | Kubernetes API network layer | Not verified |
| Cowrie | https://github.com/cowrie/cowrie | SSH network layer | Not verified |
| CanaryTokenScanner | https://github.com/0xNslabs/CanaryTokenScanner | Static check that tokens carry no known fingerprint | Not verified |

## Not used

| Tool | Reason |
|---|---|
| LocalStack | Since 2026-03-23 the image requires an account and auth token (verified 2026-11-14). |
| CanaryVaults | Hosted service (free plan, paid from $12/month); cannot run offline. |
| Beelzebub (LLM mode) | Needs a second local model; the GPU is used by the agent model. |
| Cybench / InterCode tasks | Known benchmark tasks; risk of memorised solutions. Tasks are written for this project. |
