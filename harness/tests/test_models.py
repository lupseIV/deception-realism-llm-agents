import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import models  # noqa: E402


def test_profile_resolves():
    p = models.load("gemini-flash")
    cfg = p.config()
    assert cfg.temperature == 0.7 and cfg.seed == 1234  # shared settings
    assert cfg.max_tool_output == 200000                # no harness truncation banner
    assert p.context_budget() < p.max_context


def test_gemini_profile_uses_low_thinking():
    cfg = models.load("gemini-flash").config()
    assert cfg.reasoning_effort == "low" and cfg.max_tokens is None  # no output cap


def test_unknown_profile():
    with pytest.raises(KeyError):
        models.load("nope")


OPENROUTER = ["qwen3-14b-api", "qwen3-32b-api", "qwen3-30b-a3b-api", "qwen3-235b-api", "qwen3-next-80b-api", "gemma4-31b-free", "nemotron3-ultra-free",
              "nemotron35-super-free", "ling32-flash-free", "particle1-free"]


@pytest.mark.parametrize("name", OPENROUTER)
def test_openrouter_profiles_pinned_and_no_thinking(name: str):
    p = models.load(name)
    assert p.model.startswith("openrouter/")
    prov = p.model_args["provider"]
    assert len(prov["order"]) == 1 and prov["allow_fallbacks"] is False  # one backend = one condition
    cfg = p.config()
    assert cfg.reasoning_effort is None and cfg.reasoning_tokens is None  # never asks for thinking
    assert cfg.max_retries == 5                                           # 429s fail, not hang
    assert p.context_budget() < p.max_context


@pytest.mark.parametrize("name", [n for n in OPENROUTER if n.endswith("-free")])
def test_free_hybrid_models_reasoning_disabled(name: str):
    p = models.load(name)
    assert p.model.endswith(":free") and p.model_args["reasoning_enabled"] is False


@pytest.mark.parametrize("name", ["qwen3-14b-api", "qwen3-32b-api"])
def test_ladder_a_same_budget_thinking_off(name: str):
    p = models.load(name)
    assert p.max_context == 32768 and p.model_args["reasoning_enabled"] is False


def test_local_profile_removed():
    with pytest.raises(KeyError):
        models.load("qwen3-8b-local")
