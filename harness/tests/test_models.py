import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import models  # noqa: E402


def test_profiles_resolve():
    for name in ("qwen3-8b-local", "gemini-flash"):
        p = models.load(name)
        cfg = p.config()
        assert cfg.temperature == 0.7 and cfg.seed == 1234  # shared settings
        assert p.context_budget() < p.max_context


def test_local_profile_turns_thinking_off():
    assert models.load("qwen3-8b-local").config().extra_body == {"reasoning": {"effort": "none"}}


def test_gemini_profile_turns_thinking_off():
    assert models.load("gemini-flash").config().reasoning_tokens == 0


def test_unknown_profile():
    with pytest.raises(KeyError):
        models.load("nope")
