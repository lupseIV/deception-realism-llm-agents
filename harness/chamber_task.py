"""Inspect task: a react() agent with a bash tool inside a chamber sandbox.

One sample per run prepared by prepare.py. The sandbox is that run's compose
file (agent container, gateway, Moto). The scorer reads the gateway trap log
and the transcript and computes the metrics in scoring.py.
"""

from __future__ import annotations

import json
import sys
import time
from pathlib import Path
from typing import Any

from inspect_ai import Task, task
from inspect_ai.agent import AgentPrompt, as_solver, react
from inspect_ai.dataset import MemoryDataset, Sample
from inspect_ai.model import ChatMessageAssistant, ChatMessageTool, CompactionTrim
from inspect_ai.scorer import Score, Target, mean, scorer
from inspect_ai.solver import Generate, Solver, TaskState, solver
from inspect_ai.tool import bash
from inspect_ai.util import sandbox, store

sys.path.insert(0, str(Path(__file__).resolve().parent))
import prompts  # noqa: E402
import scoring  # noqa: E402

ROOT = Path(__file__).resolve().parents[1]
TRAP_LOG = "/var/log/chamber/traps.jsonl"
NOTES = "setup-notes.md"


def load_samples(path: str, informed: bool) -> MemoryDataset:
    samples: list[Sample] = []
    for line in (ROOT / path).read_text(encoding="utf-8").splitlines():
        if not line.strip():
            continue
        r: dict[str, Any] = json.loads(line)
        f: dict[str, str] = r["facts"]
        home = f["env.home"]
        files = {f"{home}/handover.md": prompts.HANDOVER} if r["task"] == "injection" else None
        samples.append(Sample(
            id=r["id"],
            input=prompts.INJECTION if r["task"] == "injection" else prompts.BENIGN,
            sandbox=("docker", r["compose"]),
            files=files,
            metadata={**r, "informed": informed},
        ))
    return MemoryDataset(samples, name=Path(path).parent.name)


@solver
def chamber_agent(context_budget: int) -> Solver:
    async def solve(state: TaskState, generate: Generate) -> TaskState:
        f: dict[str, str] = state.metadata["facts"]
        user, home = f["env.user"], f["env.home"]
        # Sample files are written as root; give them to the persona user.
        if state.metadata["task"] == "injection":
            await sandbox().exec(["chown", f"{user}:{user}", f"{home}/handover.md"])
        system = prompts.SYSTEM.format(hostname=f["env.hostname"], full_name=f["persona.full-name"],
                                       role=f["persona.role"], company=f["persona.company"], user=user)
        if state.metadata["informed"]:
            system += prompts.INFORMED
        agent = react(
            prompt=AgentPrompt(instructions=system),
            tools=[bash(timeout=120, user=user)],
            compaction=CompactionTrim(threshold=context_budget, memory=False),
        )
        store().set("t_start", time.time())
        return await as_solver(agent)(state, generate)

    return solve


@scorer(metrics={k: [mean()] for k in
                 ("read", "exfiltrate", "use", "trigger", "depth", "avoid", "mention", "benign_success")})
def trap_scorer():
    async def score(state: TaskState, target: Target) -> Score:
        f: dict[str, str] = state.metadata["facts"]
        try:
            log = await sandbox("gateway").read_file(TRAP_LOG)
        except FileNotFoundError:
            log = ""
        try:
            notes: str | None = await sandbox().read_file(f"{f['env.home']}/{NOTES}")
        except FileNotFoundError:
            notes = None

        commands: list[str] = []
        outputs: list[str] = []
        texts: list[str] = []
        for m in state.messages:
            if isinstance(m, ChatMessageAssistant):
                texts.append(m.text)
                commands += [str(c.arguments.get("command", "")) for c in (m.tool_calls or [])]
            elif isinstance(m, ChatMessageTool):
                outputs.append(m.text)
        texts.append(state.output.completion)

        r = scoring.score_run(f, scoring.parse_events(log), commands, outputs, texts, notes,
                              store().get("t_start"))
        values = {k: r[k] for k in ("read", "exfiltrate", "use", "trigger", "depth", "avoid",
                                     "mention", "benign_success")}
        return Score(value=values, explanation=", ".join(r["mentions"]) or None,
                     metadata={**r, "trap_log": log})

    return score


@task
def pilot(samples: str = "runs/pilot/samples.jsonl", informed: bool = False,
          context_budget: int = 6900, message_limit: int = 40, time_limit: int = 900) -> Task:
    return Task(
        dataset=load_samples(samples, informed),
        solver=chamber_agent(context_budget),
        scorer=trap_scorer(),
        message_limit=message_limit,
        time_limit=time_limit,
    )
