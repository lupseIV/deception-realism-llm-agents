# deception-realism-llm-agents

Research project (Proiect de cercetare, UBB Cluj-Napoca, 2026-2027):
**Effects of Honeytoken and Network-Deception Realism on LLM Agent Behaviour.**

A tool-using LLM agent runs inside an isolated "detonation chamber" while the
realism of two deception layers is varied independently:

| Layer       | Low                    | Medium                 | High                                   |
|-------------|------------------------|------------------------|----------------------------------------|
| Honeytokens | Obviously fake         | Plausible, no context  | Statistically realistic, with history  |
| Network     | Connection refused     | Generic responses      | Protocol-faithful fake services        |

## Repository layout

| Path | Language | Content |
|---|---|---|
| `chamber/` | Go | Detonation chamber: environment, honeytoken and network layers, token sources, trap logger, gateway runtime (see `chamber/README.md`) |
| `configs/` | XML | Sandbox configurations: `pilot/` experiment conditions, `honeytokens/` one per token type |
| `personas/` | XML | Personas: who uses the machine, where tokens are placed |
| `schema/` | XSD | Schemas for personas and configurations, invalid examples, `check.sh` |
| `sandbox/` | Docker | Agent and gateway images, Moto compatibility check |
| `harness/` | Python | Inspect AI task, prompts, scoring, model profiles; `archive/` holds the dropped local-model benchmark |
| `analysis/` | Python | Metrics, Wilson confidence intervals, logistic regression (planned) |
| `docs/` | Markdown | Design (`architecture.md`, `roadmap.md`, `tools.md`) and report chapters (`report/`) |
| `runs/`, `logs/` | generated | Per-run compose files from `harness/prepare.py`; Inspect logs. Not committed |
| `tokens/` | JSON | Token pool files; may hold provider-issued tokens. Not committed |

## Requirements

- Docker Desktop with WSL2 (gVisor `runsc` runtime as target)
- Go 1.22+
- Python 3.11+
- Model access: Gemini free tier and OpenRouter (Qwen3), see `harness/models.yaml`.
  Local Qwen3 8B on Ollama was dropped on 2026-11-17 (`harness/archive/`).

Copy `.env.example` to `.env` and fill in the keys. `.env` is not committed.

## Status

Lab5-8: modelling, case study (pilot), related work. Work in progress.
