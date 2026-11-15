"""Benchmark a local Ollama model: load time, speed and VRAM per context size.

For every context size the model is unloaded and loaded again, then:
  - VRAM used after loading (nvidia-smi) and how much of the model is on the GPU (ollama /api/ps)
  - short answer and agent-style answer, with Qwen3 thinking off and on
  - one tool call through /api/chat (what the Inspect react() agent needs)
  - a prompt that fills about 70% of the context (prompt speed and peak VRAM)

Only the Python standard library is used.

    python ollama_bench.py --model qwen3:8b --ctx 4096 8192 16384 32768

Results: results/<model>-<timestamp>.json and .md
"""

import argparse
import datetime as dt
import json
import pathlib
import subprocess
import sys
import time
import urllib.error
import urllib.request

PROMPTS = {
    "short": "In two sentences, what is an AWS access key?",
    "agent": (
        "You are working in a Linux shell as a developer. List, as numbered shell commands with "
        "one line of explanation each, how you would find out why a nightly backup script that "
        "uploads to S3 failed. Do not run anything."
    ),
}

TOOL = {
    "type": "function",
    "function": {
        "name": "bash",
        "description": "Run a bash command in the sandbox and return its output.",
        "parameters": {
            "type": "object",
            "properties": {"cmd": {"type": "string", "description": "The command to run."}},
            "required": ["cmd"],
        },
    },
}


def call(host, path, body=None, timeout=900):
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(host + path, data=data, headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read() or b"{}")


def vram():
    """Used and total VRAM in MiB of the first GPU, or (None, None)."""
    try:
        out = subprocess.run(
            ["nvidia-smi", "--query-gpu=memory.used,memory.total", "--format=csv,noheader,nounits"],
            capture_output=True, text=True, check=True,
        ).stdout.splitlines()[0]
        used, total = (int(x) for x in out.split(","))
        return used, total
    except (OSError, subprocess.CalledProcessError, ValueError, IndexError):
        return None, None


def unload(host, model):
    try:
        call(host, "/api/generate", {"model": model, "keep_alive": 0})
    except urllib.error.URLError:
        pass
    time.sleep(3)


def loaded(host, model):
    for m in call(host, "/api/ps").get("models", []):
        if m.get("name", "").startswith(model) or m.get("model", "").startswith(model):
            return m
    return {}


def rates(r):
    """Timings of an Ollama response in seconds and tokens per second."""
    ns = 1e9
    pe, pd = r.get("prompt_eval_count", 0), r.get("prompt_eval_duration", 0)
    ec, ed = r.get("eval_count", 0), r.get("eval_duration", 0)
    return {
        "total_s": round(r.get("total_duration", 0) / ns, 2),
        "load_s": round(r.get("load_duration", 0) / ns, 2),
        "prompt_tokens": pe,
        "prompt_tok_s": round(pe / (pd / ns), 1) if pd else None,
        "output_tokens": ec,
        "output_tok_s": round(ec / (ed / ns), 1) if ed else None,
        "truncated": r.get("done_reason") == "length",
    }


def generate(host, model, prompt, ctx, think, num_predict):
    body = {
        "model": model, "prompt": prompt, "stream": False, "keep_alive": "10m",
        "options": {"num_ctx": ctx, "num_predict": num_predict, "temperature": 0.6},
    }
    if think is not None:
        body["think"] = think
    try:
        return call(host, "/api/generate", body)
    except urllib.error.HTTPError as e:
        if think is not None and e.code == 400:  # Ollama without the think option
            body.pop("think")
            return call(host, "/api/generate", body)
        raise


def tool_call(host, model, ctx):
    body = {
        "model": model, "stream": False, "keep_alive": "10m", "think": False,
        "options": {"num_ctx": ctx, "num_predict": 256},
        "tools": [TOOL],
        "messages": [{"role": "user", "content": "Use the bash tool to list the files in the home directory."}],
    }
    r = call(host, "/api/chat", body)
    calls = r.get("message", {}).get("tool_calls") or []
    out = rates(r)
    out["tool_call"] = calls[0]["function"] if calls else None
    return out


def filler(tokens):
    """Varied log-like text of roughly the given number of tokens. Dates and
    numbers cost more tokens than words, so about 2.2 tokens per word."""
    words_per_line, lines = 18, []
    for i in range(int(tokens / 2.2 / words_per_line) + 1):
        lines.append(f"2026-11-{1 + i % 28:02d}T{i % 24:02d}:{i % 60:02d}:00Z backup job {i} "
                     f"uploaded part {i * 7 % 997} of archive to bucket in region eu-central-1 status ok")
    return "\n".join(lines)


def bench_ctx(host, model, ctx, log):
    log(f"\n=== num_ctx {ctx} ===")
    unload(host, model)
    base_used, total = vram()

    warm = generate(host, model, "Reply with OK.", ctx, False, 8)
    used, _ = vram()
    ps = loaded(host, model)
    size, size_vram = ps.get("size", 0), ps.get("size_vram", 0)
    row = {
        "ctx": ctx,
        "load_s": round(warm.get("load_duration", 0) / 1e9, 2),
        "vram_total_mib": total,
        "vram_before_mib": base_used,
        "vram_loaded_mib": used,
        "vram_free_mib": (total - used) if total and used else None,
        "model_size_mib": round(size / 2**20),
        "on_gpu_pct": round(100 * size_vram / size) if size else None,
        "runs": [],
    }
    log(f"loaded in {row['load_s']} s, VRAM {used}/{total} MiB, {row['on_gpu_pct']}% of the model on the GPU")

    for name, prompt in PROMPTS.items():
        for think in (False, True):
            r = rates(generate(host, model, prompt, ctx, think, 1024 if think else 512))
            r.update(prompt=name, think=think)
            row["runs"].append(r)
            log(f"{name:6} think={think!s:5} {r['total_s']:6} s  out {r['output_tokens']:4} tok "
                f"@ {r['output_tok_s']} tok/s{'  (truncated)' if r['truncated'] else ''}")

    t = tool_call(host, model, ctx)
    t.update(prompt="tool_call", think=False)
    row["runs"].append(t)
    log(f"tool   think=False {t['total_s']:6} s  call: {t['tool_call']}")

    f = rates(generate(host, model, filler(int(ctx * 0.7)) + "\n\nHow many backup jobs are listed? Answer with a number.",
                       ctx, False, 32))
    peak, _ = vram()
    f.update(prompt="fill_70pct", think=False, vram_peak_mib=peak)
    row["runs"].append(f)
    log(f"fill   {f['prompt_tokens']} prompt tok @ {f['prompt_tok_s']} tok/s, {f['total_s']} s, VRAM peak {peak} MiB")
    return row


def markdown(meta, rows):
    out = [f"# Ollama benchmark: {meta['model']}", "",
           f"{meta['time']} · Ollama {meta['ollama']} · GPU VRAM {rows[0]['vram_total_mib']} MiB", "",
           "| num_ctx | load s | VRAM loaded MiB | VRAM free MiB | on GPU % | short s (think off/on) | agent s (off/on) | out tok/s | tool call | fill prompt tok/s | fill VRAM peak MiB |",
           "|---|---|---|---|---|---|---|---|---|---|---|"]
    for r in rows:
        runs = {(x["prompt"], x["think"]): x for x in r["runs"]}
        s0, s1 = runs[("short", False)], runs[("short", True)]
        a0, a1 = runs[("agent", False)], runs[("agent", True)]
        tool, fill = runs[("tool_call", False)], runs[("fill_70pct", False)]
        out.append(f"| {r['ctx']} | {r['load_s']} | {r['vram_loaded_mib']} | {r['vram_free_mib']} | {r['on_gpu_pct']} "
                   f"| {s0['total_s']} / {s1['total_s']} | {a0['total_s']} / {a1['total_s']} | {a0['output_tok_s']} "
                   f"| {'yes' if tool['tool_call'] else 'no'} | {fill['prompt_tok_s']} | {fill['vram_peak_mib']} |")
    return "\n".join(out) + "\n"


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--model", default="qwen3:8b")
    ap.add_argument("--ctx", type=int, nargs="+", default=[4096, 8192, 16384, 32768])
    ap.add_argument("--host", default="http://localhost:11434")
    ap.add_argument("--out", default=str(pathlib.Path(__file__).parent / "results"))
    a = ap.parse_args()

    try:
        version = call(a.host, "/api/version").get("version", "?")
    except urllib.error.URLError as e:
        sys.exit(f"Ollama not reachable at {a.host}: {e}")
    if vram() == (None, None):
        print("warning: nvidia-smi not found, VRAM columns stay empty")

    meta = {"model": a.model, "ollama": version, "time": dt.datetime.now().isoformat(timespec="seconds")}
    rows = []
    for ctx in a.ctx:
        try:
            rows.append(bench_ctx(a.host, a.model, ctx, print))
        except urllib.error.URLError as e:
            print(f"num_ctx {ctx} failed: {e}")
    unload(a.host, a.model)
    if not rows:
        sys.exit("no results")

    out = pathlib.Path(a.out)
    out.mkdir(parents=True, exist_ok=True)
    stem = f"{a.model.replace(':', '-').replace('/', '-')}-{dt.datetime.now():%Y%m%d-%H%M}"
    (out / f"{stem}.json").write_text(json.dumps({"meta": meta, "rows": rows}, indent=2), encoding="utf-8")
    (out / f"{stem}.md").write_text(markdown(meta, rows), encoding="utf-8")
    print(f"\nwritten {out / stem}.json and .md")


if __name__ == "__main__":
    main()
