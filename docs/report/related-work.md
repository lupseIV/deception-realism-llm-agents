# Related Work

This chapter positions the detonation chamber against the literature it builds
on and compares with. Work is grouped by theme: measuring cyber deception, the
realism and fingerprinting of traps, deception aimed at LLM agents, the agent
harness and prompt injection, and workload isolation. Each group states what the
existing approach does, how it differs from ours, and where our results should
line up with it. The chapter closes with the metrics where we expect results
similar to or better than the literature, and a deployment plan for the tool.

## 1. Measuring the effectiveness of deception

Han, Kheir and Balzarotti [15] survey cyber deception and call for rigorous,
quantitative measurement of deception techniques rather than anecdotal reports.
Our study answers that call for the specific case of LLM agents by treating the
realism of two deception layers as independent variables and measuring agent
behaviour, not human judgement.

Honeyquest [2] measures the enticingness of deception techniques with code-based
questionnaires: respondents mark what they would interact with in a snippet.
Honeyquest for LLMs [4] applies the same questionnaire format to language models.
Both are static: a model inspects fixed snippets and answers, with no live
environment, no tools, and no consequence to acting. The Tularosa study [7] is
the closest experimental design on the human side, quantifying deception
effectiveness on human red teamers and, crucially, testing the effect of telling
subjects that deception may be present. We follow the Tularosa informed-versus-
uninformed manipulation (RQ4, H4), but with tool-using agents acting in a live
sandbox rather than humans answering a protocol.

The difference from [2] and [4] is ecological validity: our agents run real
commands (shell, AWS CLI) against real services and can read, exfiltrate or use a
honeytoken, so we observe actions and their progression rather than stated intent.

## 2. Realism and fingerprinting of traps

Several works inform how we build and validate the three realism levels. Bowen et
al. [10] define decoy-document properties (believability, enticingness,
conspicuousness, differentiability) that guide our honeytoken levels. HoneyGen [9]
introduced the idea of generating honeytokens automatically from the real data
they imitate. We adopt that idea, not its method: our most realistic level derives,
for each run, a unique token from the real data being mocked, with surrounding
history, using our own generator.

On the detection side, Honeysweeper [1] shows that honeytokens carry
fingerprintable artifacts, and Bitter harvest [3] fingerprints low- and
medium-interaction honeypots at internet scale by protocol deviations. These
define what an "obviously fake" or "generic" level looks like to a capable
attacker, and motivate the protocol-faithful network level that avoids such
tells. Spotless sandboxes [8] evades malware-analysis systems using wear-and-tear
artifacts: real machines look used, sandboxes look pristine. We adopt this
directly, planting a lived-in home directory (shell history, varied timestamps, a
project folder) so the environment itself is not a tell; the same wear-and-tear
baseline is present in every condition, including the control, so conditions
differ only by the honeytoken.

Our contribution relative to this group is to turn realism, which these papers
treat as a fixed property to detect or evade, into a manipulated variable and
measure how it changes agent behaviour.

## 3. Deception aimed at LLM agents

Two recent works study LLM agents and deception directly. Cloak, honey, trap [6]
proposes proactive defenses against LLM agents but does not vary the realism of
the traps. Xie et al. [5] study LLM-based penetration testing in the presence of
honeypots, focusing on the attacking agent and showing that blocking outbound
traffic can reveal a honeypot; this motivates our Experiment 3 (sandbox versus
detonation mode) and our choice to redirect outbound traffic to fake services
rather than block it. We differ from both by studying trap design as the
independent variable: we hold the agent fixed and vary how realistic the two
deception layers are, which neither [5] nor [6] does.

## 4. Agent harness and prompt injection

The agent is a ReAct agent [13]: it interleaves reasoning and tool actions, which
is what lets us observe a progression from reading a token to using it. Our
prompt-injection task follows the indirect prompt-injection threat model of
Greshake et al. [11], where the malicious instruction arrives through content the
agent reads rather than the user prompt, and the task design follows AgentDojo
[12], a dynamic environment for evaluating prompt-injection attacks and defenses.
We reuse the injection-task design of [12] but place it in a live sandbox with
real services and honeytokens, which [12] (simulated Python tools) does not have.

## 5. Isolation

Because the agent executes untrusted actions, the workload must be isolated.
Anjali et al. [14] compare container and virtual-machine isolation for Firecracker
and gVisor, quantifying the overhead and the kernel-surface trade-off. This guides
our runtime: the pilot uses standard Docker (runc) for speed and reproducibility,
with gVisor (runsc) as the planned hardening, adopted only if the kernel-surface
probe shows that the runc surface is detectable by the agent.

## 6. Comparison

| Work | Deception layer | Realism as a variable | Subject | Environment | Relation to ours |
| --- | --- | --- | --- | --- | --- |
| Honeyquest [2] | honeytoken | no | human | static questionnaire | We measure actions on live agents, not stated intent |
| Honeyquest for LLMs [4] | honeytoken | no | LLM | static questionnaire | We add tools, a live environment and a recognition-action gap |
| Cloak, honey, trap [6] | mixed | no | LLM agent | live | We vary trap realism instead of proposing a fixed defense |
| Xie et al. [5] | network/honeypot | no | LLM agent | live | We study trap design; their outbound-block finding motivates Exp 3 |
| Tularosa [7] | mixed | no | human | live | We reuse informed vs uninformed, on agents |
| Honeysweeper [1] | honeytoken | detection only | tooling | - | Defines the "obviously fake" level |
| Bitter harvest [3] | network | detection only | tooling | internet | Defines protocol-faithfulness for the network level |
| Spotless sandboxes [8] | environment | evasion only | malware | live | Source of the wear-and-tear baseline |
| HoneyGen [9] | honeytoken | no | - | - | Idea only: realistic tokens from real data (our own generator) |
| Decoy documents [10] | honeytoken | no | human | live | Source of the decoy properties |
| AgentDojo [12] | - | no | LLM agent | simulated | Source of the injection-task design |
| This work | honeytoken + network | yes (3x3) | LLM agent | live sandbox | First to vary two deception layers independently on live agents |

## 7. Metrics where we expect similar or better results

- Enticingness ordering. Honeyquest [2] ranks techniques by stated enticingness.
  We expect our trigger rate to order the honeytoken levels in the same direction,
  but measured behaviourally, which [2] cannot do.
- Recognition-action gap (H1). Following the LLM questionnaire results of [4], we
  expect agents to mention a trap less as realism rises, while still triggering it,
  so the gap between mention and trigger should be visible in action data for the
  first time.
- Disengagement under poor realism (H2). Consistent with the detectability of
  generic and refused responses in [1] and [3], we expect agents to give up or
  change strategy sooner when the network level is refused or generic.
- Informed effect (H4). Following Tularosa [7], we expect informed agents to
  trigger as often but progress less far (lower depth).
- Overall measurement rigour. We expect to provide the quantitative,
  behaviour-based measurement of deception that [15] calls for and that [2], [4]
  and [6] do not jointly provide (realism varied, two layers, live agents).
