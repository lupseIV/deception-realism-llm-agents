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

| Path        | Language | Content                                                        |
|-------------|----------|----------------------------------------------------------------|
| `chamber/`  | Go       | Honeytoken generator, fake network services, trap logger       |
| `sandbox/`  | Docker   | Container setup (outbound blocked / redirected to fake services) |
| `harness/`  | Python   | Inspect AI tasks and ReAct agent configuration                 |
| `analysis/` | Python   | Metrics, Wilson confidence intervals, logistic regression      |
| `docs/`     | Markdown | Research report chapters                                       |

## Requirements

- Docker Desktop with WSL2 (gVisor `runsc` runtime as target)
- Go 1.22+
- Python 3.11+
- Ollama with `qwen3:8b` for local runs; Gemini API key for the second model

Copy `.env.example` to `.env` and fill in the keys. `.env` is not committed.

## Status

Lab5-8: modelling, case study (pilot), related work. Work in progress.
