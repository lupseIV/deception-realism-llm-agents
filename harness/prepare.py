"""Generate the per-run sandboxes and the sample list for a pilot.

For every condition x task x repetition this script calls the chamber CLI:
  chamber compose  -> runs/<set>/<run-id>/compose.yaml (the Inspect sandbox)
  chamber facts    -> the run's public facts (user, home, token path, key id)
and writes runs/<set>/samples.jsonl, which chamber_task.py reads.

  python harness/prepare.py                      # pilot: 5 configs x 2 tasks x 3 reps
  python harness/prepare.py --reps 1 --tasks benign
"""

from __future__ import annotations

import argparse
import json
import platform
import subprocess
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
CHAMBER = ROOT / "chamber"
TASKS = ("benign", "injection")


def build_chamber() -> Path:
    exe = CHAMBER / "bin" / ("chamber.exe" if platform.system() == "Windows" else "chamber")
    subprocess.run(["go", "build", "-o", str(exe), "./cmd/chamber"], cwd=CHAMBER, check=True)
    return exe


def chamber(exe: Path, *args: str) -> str:
    r = subprocess.run([str(exe), *args], cwd=CHAMBER, check=True, capture_output=True, text=True)
    return r.stdout


def parse_facts(text: str) -> dict[str, str]:
    out: dict[str, str] = {}
    for line in text.splitlines():
        k, sep, v = line.partition(" = ")
        if sep and "secret" not in k:  # secrets never leave the gateway
            out[k.strip()] = v.strip()
    return out


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--configs", default="configs/pilot", help="folder with sandbox XML files")
    ap.add_argument("--set", default="pilot", help="name of the run set (folder under runs/)")
    ap.add_argument("--tasks", nargs="+", default=list(TASKS), choices=TASKS)
    ap.add_argument("--reps", type=int, default=3)
    ap.add_argument("--seed", type=int, default=1, help="seed of the first repetition")
    ap.add_argument("--runtime", default="runc", help="runc on Docker Desktop, runsc with gVisor")
    a = ap.parse_args()

    exe = build_chamber()
    out_dir = ROOT / "runs" / a.set
    out_dir.mkdir(parents=True, exist_ok=True)
    configs = sorted((ROOT / a.configs).glob("*.xml"))
    if not configs:
        raise SystemExit(f"no XML configurations in {a.configs}")

    samples: list[dict[str, Any]] = []
    for rep in range(a.reps):
        seed = a.seed + rep  # same seed for every condition and task of a repetition (paired)
        for cfg in configs:
            rel = cfg.relative_to(ROOT).as_posix()
            for task in a.tasks:
                run_id = f"{cfg.stem}-{task}-r{rep + 1}"
                run_dir = out_dir / run_id
                common = ["-base-dir", "..", "-config", rel, "-run-id", run_id, "-seed", str(seed)]
                chamber(exe, "compose", *common, "-runtime", a.runtime, "-out", str(run_dir))
                facts = parse_facts(chamber(exe, "facts", *common))
                samples.append({"id": run_id, "condition": cfg.stem, "task": task, "rep": rep + 1,
                                "seed": seed, "compose": str((run_dir / "compose.yaml").resolve()),
                                "facts": facts})
                print(run_id)

    path = out_dir / "samples.jsonl"
    path.write_text("".join(json.dumps(s) + "\n" for s in samples), encoding="utf-8")
    print(f"{len(samples)} samples -> {path.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
