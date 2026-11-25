# Ollama benchmark (archived)

Archived on 2026-11-26. Local Qwen3 8B on Ollama was dropped on 2026-11-17
(see `harness/models.yaml`); the results here are the evidence for that
decision. `qwen3-8k.Modelfile` is the 8k-context model used in the runs.

Measures, for a local model and several context sizes: load time, VRAM used
and left after loading, how much of the model sits on the GPU, answer time and
output speed with Qwen3 thinking off and on, whether a tool call works (needed
by the Inspect `react()` agent), and prompt speed and peak VRAM with a prompt
that fills about 70% of the context.

## Setup (Windows)

1. Install Ollama for Windows from https://ollama.com/download and start it
   (tray icon; the API listens on http://localhost:11434).
2. Pull the model: `ollama pull qwen3:8b`
3. Check: `ollama run qwen3:8b "Reply with OK."` and `nvidia-smi`

## Run

```powershell
cd harness\archive\ollama-qwen3-8b
python ollama_bench.py --model qwen3:8b --ctx 4096 8192 16384 32768
```

Close other GPU programs first (browser video, games), they take VRAM.
Results go to `results/<model>-<time>.json` and `.md`; commit them.

Optional second run with a smaller KV cache: quit Ollama, set the user
environment variables `OLLAMA_FLASH_ATTENTION=1` and `OLLAMA_KV_CACHE_TYPE=q8_0`,
start Ollama again and repeat the run.

## Reading the table

- `on GPU %` below 100: the model plus KV cache no longer fits in VRAM and
  part runs on the CPU; that context size is too large for agent runs.
- `VRAM free MiB` after loading is the headroom left at that context size.
- `tool call` must be `yes`, otherwise the react() agent cannot use tools.
