"""Run a sample set with one model profile.

  python harness/run.py --profile qwen3-14b-api
  python harness/run.py --profile qwen3-14b-api --limit 1         # smoke test
  python harness/run.py --profile qwen3-235b-api --informed

Runs are sequential: all runs share the sandbox subnet and Moto's reset.
Logs go to logs/<set>/ and carry the resolved profile in their metadata.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

# Inspect's own signature has a partially unknown parameter type (scanner).
from inspect_ai import eval as inspect_eval  # pyright: ignore[reportUnknownVariableType]

sys.path.insert(0, str(Path(__file__).resolve().parent))
import models  # noqa: E402
from chamber_task import pilot  # noqa: E402

ROOT = Path(__file__).resolve().parents[1]


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--profile", required=True)
    ap.add_argument("--set", default="pilot")
    ap.add_argument("--informed", action="store_true")
    ap.add_argument("--limit", type=int, default=None)
    ap.add_argument("--message-limit", type=int, default=40)
    a = ap.parse_args()

    p = models.load(a.profile)
    t = pilot(samples=f"runs/{a.set}/samples.jsonl", informed=a.informed,
              context_budget=p.context_budget(), message_limit=a.message_limit)
    inspect_eval(
        t,
        model=p.model,
        model_args=p.model_args,
        limit=a.limit,
        max_samples=1,
        max_sandboxes=1,
        fail_on_error=False,   # an infra error marks the sample, it is not agent behaviour
        retry_on_error=2,
        log_dir=str(ROOT / "logs" / a.set),
        metadata={**p.metadata(), "set": a.set, "informed": a.informed},
        **p.config().model_dump(exclude_none=True),
    )


if __name__ == "__main__":
    main()
