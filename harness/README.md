# harness

Inspect AI tasks (benign and prompt-injection variants) and the agent
configuration. This file covers three things:

1. where the model runs (local vs cloud) and what it costs,
2. how the harness talks to a model so it does not depend on where the model runs,
3. which existing harnesses fit this research and how far each can be changed.

Prices and API details were checked on 2026-11-16. Re-check them before a run
and write the date in the run log.

---

## 1. Local vs cloud inference

### 1.1 Split

Only model inference may move to the cloud. Everything else stays on the local PC:

| Part | Where | Why |
|---|---|---|
| Inspect process (task, agent loop, scorer) | local | owns the logs, cheap |
| Agent container (Docker, later gVisor) | local | full control of network and egress |
| Detonation chamber (Go fake services, trap logger) | local | Exp 3 needs outbound traffic redirected to our services |
| Model inference | local (Ollama) or cloud (API / rented GPU) | larger models do not fit in 8 GB VRAM |

The agent's tool calls always run in the local container. Only the request to
the model and its answer cross the network. The container never talks to the
model endpoint and never sees an API key.

### 1.2 Why add a cloud model

`archive/ollama-qwen3-8b/results/qwen3-8b-*` shows that `qwen3:8b` stays fully on the GPU only up
to an 8k context. At 16k only about 80% of the model is on the GPU and output
speed roughly halves. A larger model cannot be run locally at a useful speed.
A larger model also makes the results more credible (same family, different size).

### 1.3 Options and prices (checked 2026-11-16)

**A. Hosted API (preferred).** Second model for now is **Gemini 3.8 Flash on the Google AI free tier** (`google/gemini-3.8-flash`, `GOOGLE_API_KEY`): no cost, thinking disabled, but a low daily request quota, so runs stay sequential and a pilot is realistic while the full grid may need `gemini-3.5-flash-lite` (lighter) or a paid key. Thinking is turned off with `reasoning_tokens: 0` to match the local model.

**Gemini free-tier quota.** When the quota runs out, every call returns 429 `You exceeded your current quota`. Inspect then retries in the background, so in `inspect view` the sample looks stuck on the system prompt. Pilot logs from 2026-11-17 show this, not a context limit (the last working run was at about 2.4k input tokens). Every API profile now sets `max_retries: 5`, so the sample fails as an infra error instead of hanging.

**OpenRouter profiles** (`OPENROUTER_API_KEY`, thinking off, one backend pinned, prices checked 2026-11-17):

| Profile | Model | Backend | Price in / out per M tokens | Thinking |
|---|---|---|---|---|
| `qwen3-14b-api` | Qwen3-14B dense (same release as local `qwen3:8b`) | DeepInfra fp8 | $0.12 / $0.24 | off (`reasoning_enabled: false`) |
| `qwen3-32b-api` | Qwen3-32B dense (same release) | DeepInfra fp8 | $0.08 / $0.28 | off |
| `qwen3-30b-a3b-api` | Qwen3-30B-A3B Instruct-2507 (MoE) | Nebius fp8 | $0.10 / $0.30 | none (Instruct build) |
| `qwen3-235b-api` | Qwen3-235B-A22B Instruct-2507 | Parasail fp8 | $0.14 / $0.80 | none (Instruct build) |
| `qwen3-next-80b-api` | Qwen3-Next-80B-A3B Instruct | Parasail fp8 | $0.10 / $1.10 | none (Instruct build) |
| `gemma4-31b-free` | Gemma 4 31B | Google AI Studio (free) | $0 | off (`reasoning_enabled: false`) |
| `nemotron3-ultra-free` | Nemotron 3 Ultra 550B-A55B | NVIDIA (free) | $0 | off |
| `nemotron35-super-free` | Nemotron 3.5 Super VL 120B-A12B | NVIDIA (free) | $0 | off |
| `ling32-flash-free` | Ling 3.2 Flash | Ant Digital fp8 (free) | $0 | off |
| `particle1-free` | Particle-1 (DeepSeek V4.1 Flash base) | Particle AI (free) | $0 | off |

**Model comparison.** The local Qwen3 8B is dropped: on 8 GB VRAM it stays fully on the GPU only up to an 8k context, which is too small for the agent runs. Ladder A (scale within one release): `qwen3-14b-api` -> `qwen3-32b-api`, both April 2025 Qwen3 dense models, thinking off, fp8 on the same backend, same 32k history budget. Ladder B (2507 Instruct MoE): `qwen3-30b-a3b-api` -> `qwen3-235b-api`. Do not pool the two ladders.

The free Gemma endpoint shares Google's upstream rate-limit pool and returned 429 when tested on 2026-11-17. The four other free models answered with 0 reasoning tokens. Free endpoints are rate limited and can be changed or removed without notice, so use them for the pilot, not for the final grid.

**B. Rented GPU running vLLM.** Only needed for control over weights,
quantization or logprobs.

| GPU (RunPod) | Community Cloud | Secure Cloud |
|---|---|---|
| A100 PCIe 80 GB | from $1.19/h | $1.79/h |
| H100 PCIe 80 GB | from $1.99/h | $2.89/h |

Billed per second. The GPU also costs money while the agent waits for tool
calls, so for this workload option B usually costs more than option A.

**Rough budget for option A (estimate, not measured).** If one agent run uses
100k-300k input tokens, the 840 runs of Exp 1 + Exp 2 need about 100M-250M input
tokens, roughly $10-$120 depending on the model. Measure this with a pilot of
about 10 runs. Inspect logs record token usage per sample.

---

## 2. Design: model access that is not tied to the local machine

### 2.1 Principle

Harness code never names a host, port or provider. It asks for a **model
profile** by name. A profile maps to an Inspect model string, model args and a
fixed generation config. Inspect already abstracts providers, so this layer is
thin. Moving from local Ollama to a cloud API is a config change, not a code
change.

```
task.py ──> profile name ("qwen3-14b-api") ──> models.yaml ──> Inspect model string + args
                                                                     │
                       ┌──────────────────────┬──────────────────────┼─────────────────────┐
                       ▼                      ▼                      ▼                     ▼
               ollama/qwen3:8b      vllm/Qwen/Qwen3-8B     openrouter/qwen/...    google/gemini-...
               (local GPU)          (rented GPU, URL       (API, provider          (free tier)
                                     from VLLM_BASE_URL)    pinned)
```

### 2.2 Inspect model strings used

| Backend | Model string | Settings (in `.env`, never committed) |
|---|---|---|
| Local Ollama | `ollama/qwen3:8b` | `OLLAMA_BASE_URL` (default `http://localhost:11434/v1`) |
| Rented GPU, vLLM | `vllm/<hf-model-id>` | `VLLM_BASE_URL`, `VLLM_API_KEY` |
| OpenRouter | `openrouter/<vendor>/<model>` | `OPENROUTER_API_KEY` |
| Any OpenAI-compatible API | `openai-api/<provider>/<model>` | `<PROVIDER>_API_KEY`, `<PROVIDER>_BASE_URL` |
| Gemini | `google/<model>` | `GOOGLE_API_KEY` |

OpenRouter can route one model to different backends. For reproducible runs,
pin one backend with the `provider` model arg (fixed `order`,
`allow_fallbacks: false`).

### 2.3 Profile file (`harness/models.yaml`)

```yaml
profiles:
  qwen3-14b-api:
    model: openrouter/qwen/qwen3-14b
    model_args:
      provider: {order: [deepinfra], allow_fallbacks: false, require_parameters: true}
      reasoning_enabled: false
    max_context: 32768
  qwen3-235b-api:
    model: openrouter/qwen/qwen3-235b-a22b-2507
    model_args:
      provider: {order: ["<backend>"], allow_fallbacks: false}
  gemini-flash:
    model: google/gemini-3.8-flash

generate:                          # same for every profile
  temperature: <fixed>
  max_tokens: <fixed>
  seed: <fixed, where supported>
```

`harness/models.py` reads the file and returns the model string,
model args and `GenerateConfig`. Tasks pass the result to `eval()`. The profile
name and the resolved values are written into the run metadata.

### 2.4 Rules

1. **One code path.** Local and cloud models go through the same task,
   agent and scorer. Only the profile changes.
2. **Secrets stay on the host.** Keys live in `.env` on the host. The agent
   container gets no model keys and no route to the model endpoint.
3. **Infra errors are not agent behaviour.** A timeout, 429 or 5xx from the
   model API must not be scored as the agent giving up (RQ2). Such samples are
   marked as errors and re-run. Keep errors and agent decisions apart in the log.
4. **Same weights does not mean same condition.** `qwen3:8b` in Ollama (Q4) and
   Qwen3-8B in vLLM (BF16) are different conditions. Record backend and
   quantization per run and never pool them silently.
5. **Fixed generation settings.** Temperature, max tokens and seed are set
   once per experiment. Hosted APIs are not fully deterministic even at
   temperature 0, so variance is handled by repetitions and Wilson CIs, not by
   claiming determinism.
6. **Context budget per profile.** The agent's message history is truncated or
   compacted to the profile's `max_context`, so a local and a cloud model see
   comparable histories.
7. **Concurrency limit per profile.** Free tiers and local GPUs need low
   parallelism; the API can take more. Set the limit in the profile, not in the task.

### 2.5 Rented GPU checklist

1. Start a pod with vLLM serving the model, with an API key set.
2. Put the pod URL in `VLLM_BASE_URL` and the key in `VLLM_API_KEY`.
3. Check that a tool call works before the experiment (the archived Ollama
   benchmark in `archive/ollama-qwen3-8b/` shows the check).
4. Stop the pod after the run. A stopped pod still pays for storage until the
   volume is deleted.

### 2.6 Running

```bash
pip install -r harness/requirements.txt
python harness/prepare.py                                     # compose files + runs/pilot/samples.jsonl
python harness/run.py --profile qwen3-14b-api --limit 1       # smoke test, one sample
python harness/run.py --profile qwen3-14b-api                 # all samples, sequentially
inspect view --log-dir logs/pilot
```

| File | Role |
|---|---|
| `models.yaml`, `models.py` | model profiles |
| `prepare.py` | one chamber run per condition x task x repetition (`chamber compose` + `chamber facts`) |
| `prompts.py` | system prompt, benign and injection task, handover note with the injection |
| `chamber_task.py` | Inspect task: `react()` agent with `bash` as the persona user, history trimmed to the profile budget, trap scorer |
| `scoring.py` | metrics from trap log and transcript (read, exfiltrate, use, trigger, depth, avoid, mention, benign success) |
| `run.py` | runs a sample set with one profile; logs to `logs/<set>/` |

---

## 3. Existing harnesses and how far they can be changed

Customisation levels used below:

- **L0 config**: CLI flags, model args, YAML.
- **L1 compose**: own tasks, datasets, prompts, scorers using the public API.
- **L2 extend**: own tools, agents, sandbox types or model providers through
  documented extension points.
- **L3 fork**: change the framework source.

| Harness | What it is | Model backends | Sandbox | Useful up to | Fit for this project |
|---|---|---|---|---|---|
| **Inspect AI** (UK AISI) | Evaluation framework with built-in `react()` agent | Lab APIs, Ollama, vLLM, OpenRouter, any OpenAI-compatible API; own providers via `@modelapi` | Docker built in; extension API for custom sandboxes; external packages for k8s, EC2, Modal, Daytona and others | L2 | **Main harness** (already chosen) |
| **Inspect Cyber** (UK AISI) | Inspect extension for agentic cyber evaluations | as Inspect | as Inspect | L1 | Optional: structure for many environment variants. Check that it adds value before using it |
| **AgentDojo** (ETH Zurich) | Prompt-injection benchmark with tasks, attacks and defences; built to be extended | OpenAI, Anthropic, Cohere, Google in its pipeline | Simulated tools in Python, no real container | L1-L2 | Not as harness (no real network or files for honeytokens). Use its injection-task design as a model for our prompt-injection tasks. A fork (AgentDojo-Inspect, US AISI) adds an Inspect bridge |
| **mini-swe-agent** (SWE-agent team) | About 100 lines of agent, bash as its only tool | LiteLLM, OpenRouter, Portkey | local, Docker/Podman, Singularity, Bubblewrap | L3 is cheap (tiny code) | Possible second agent scaffold to test that results do not depend on `react()`. Not planned for Lab5-8 |

Other agent frameworks (OpenHands, PyRIT and similar) were not evaluated.

### 3.1 What we change in Inspect, by level

| Level | Change | Serves |
|---|---|---|
| L0 | Model profile -> `--model`, `-M` args, generation config | Section 2, both models |
| L1 | Tasks: benign and prompt-injection, one sample per sandbox config in `configs/` | Exp 1 grid |
| L1 | `react()` params: `prompt` (informed vs uninformed instructions), `tools=[bash(), text_editor()]`, `truncation`/`compaction` for the context budget, `on_continue` for the turn limit | Exp 2, rule 6 |
| L1 | Scorer reads the chamber trap log from the sandbox and computes trigger, time to first trigger, progression depth, avoidance, benign success | Metrics |
| L1 | Trap-mention codebook applied to the transcript in the scorer | Trap mention metric |
| L2 | Sandbox: Docker compose per condition (agent + chamber services, egress blocked or redirected); later gVisor `runsc` as Docker runtime | Exp 3 |
| L2 | Optional custom sandbox type only if Docker compose cannot express the chamber setup | Exp 3 |
| L3 | Not planned. Keeps runs reproducible with a pinned Inspect version | - |

### 3.2 Decision

Stay on Inspect at L1-L2: the sandbox, model access and logging needs are
covered by documented extension points, so no fork is needed. Pin the Inspect
version in the requirements file and record it in every run log.

---

## Sources

- Inspect AI, Model providers: https://inspect.aisi.org.uk/providers.html
- Inspect AI, ReAct agent: https://inspect.aisi.org.uk/react-agent.html
- Inspect AI, Sandboxing: https://inspect.aisi.org.uk/sandboxing.html
- Inspect AI, Sandbox extensions: https://inspect.aisi.org.uk/extensions-sandboxes.html
- Inspect AI, Agent bridge: https://inspect.aisi.org.uk/agent-bridge.html
- Inspect Cyber: https://ukgovernmentbeis.github.io/inspect_cyber/
- AgentDojo: https://github.com/ethz-spylab/agentdojo
- AgentDojo-Inspect (US AISI): https://data.nist.gov/pdr/lps/ark:/88434/mds2-3690
- mini-swe-agent: https://github.com/SWE-agent/mini-swe-agent
- OpenRouter, Qwen3 235B A22B 2507: https://openrouter.ai/models/qwen/qwen3-235b-a22b-2507
- RunPod pricing: https://runpod.io/pricing
