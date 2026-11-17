"""Resolve a model profile from models.yaml into Inspect eval arguments."""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import yaml
from inspect_ai.model import GenerateConfig

PROFILES = Path(__file__).with_name("models.yaml")


@dataclass
class Profile:
    name: str
    model: str
    model_args: dict[str, Any] = field(default_factory=dict[str, Any])
    max_context: int = 8192
    max_connections: int = 1
    generate: dict[str, Any] = field(default_factory=dict[str, Any])
    notes: str = ""

    def config(self) -> GenerateConfig:
        return GenerateConfig(max_connections=self.max_connections, **self.generate)

    def context_budget(self) -> int:
        """History-token budget for compaction: most of the context window.
        Independent of the output cap, so it can never go non-positive."""
        return max(4096, int(self.max_context * 0.9))

    def metadata(self) -> dict[str, Any]:
        """What is written into every run log."""
        return {"profile": self.name, "model": self.model, "model_args": self.model_args,
                "max_context": self.max_context, "generate": self.generate, "notes": self.notes}


def load(name: str, path: Path = PROFILES) -> Profile:
    data: dict[str, Any] = yaml.safe_load(path.read_text(encoding="utf-8"))
    profiles: dict[str, dict[str, Any]] = data.get("profiles", {})
    if name not in profiles:
        raise KeyError(f"unknown model profile {name!r}; known: {', '.join(sorted(profiles))}")
    p = dict(profiles[name])
    generate: dict[str, Any] = {**data.get("generate", {}), **p.pop("generate", {})}
    return Profile(name=name, generate=generate, **p)
