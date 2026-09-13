# ADR-0017: The runtime hides only its own — the environment scrub is withdrawn and the prefix rule is inverted

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-09-13) — implemented and unreleased |
| Date | 2026-09-13 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | Operator: "a variable is visible in the process space anyway — why hide it only when it goes through the agent? If it must not leak, do not put it in the environment." Then, on what remains: "which variables a read-lane program needs cannot be judged, so the environment is not something to touch; but the variables the runtime holds for itself, it knows by name, so those it can hide." |
| Amends | ADR-0015 (the R01 answer of v0.4.0: `readLaneExports` and the scrub around it are replaced by the inverse rule) |
| Relates to | ADR-0016 (the same question where a kernel exists), ADR-0008 (the read lane must stay able to compile, vet and test), gem-agent ADR-0073 §6 (where the scrub came from) |

## Context

v0.4.0 answered the system risk review's R01 by replacing the `LAGENT_`
prefix exemption inside the read lane's environment scrub with
`readLaneExports`, a three-name list. The finding was real: with the
prefix exempt, `LAGENT_API_KEY` survived the scrub and reached
read-lane commands, whose output goes to the model unasked.

The operator's question is whether the scrub should exist at all, and
two measurements say it was never a control.

**It covers one child of six.** Every other child receives the
operator's environment whole:

| child | environment |
|---|---|
| read-lane shell | scrubbed |
| write-lane shell | full |
| operator-lane shell | full |
| `!` direct shell | full |
| MCP servers | full (`cmd.Env = os.Environ()`, plus the configuration's own `env` block) |
| hooks | full (`cmd.Env` unset, so inherited) |

**And the one door leaks.** Against the shipped regex these pass:

```
OPENAI_KEY  GH_PAT  STRIPE_SK  ANTHROPIC_KEY  GPG_PASSPHRASE
SESSION_COOKIE  BEARER  LICENSE_KEY  PAT
```

`NPM_TOKEN` is caught and `OPENAI_KEY` is not. The system risk review
had already written the conclusion in its chapter 10: one cannot
explain this as "put a secret in an environment variable and it is
uniformly protected".

### Why an allowlist is not the fix

The obvious repair — invert the denylist and give the read lane only
what it needs — is rejected. The set of variables a legitimate
toolchain reads is as open-ended as the set of names a secret can have:
`GOFLAGS`, `GOMODCACHE`, `GOPRIVATE`, `PYTHONPATH`, `VIRTUAL_ENV`,
`JAVA_HOME`, `CARGO_HOME`, and one more per tool released next month.
An allowlist moves the unbounded list from "names that look secret" to
"names something needs" and changes the failure from a silent leak to a
read-lane build that breaks until the list catches up — the friction
ADR-0008 was written to remove.

So the environment has no bounded domain on either side. That is the
difference from ADR-0016: there a kernel exists and the judgment moves
to it. Here none does — the environment is whatever the parent hands to
`exec`, and no operating-system mechanism filters it. Where there is no
bounded domain to move the judgment to, the rule should not exist.

### What is bounded

One set is finite, enumerable, and known exhaustively by the people who
wrote it: **the variables in lagent's own namespace.** Nobody else
reads `LAGENT_*`. Today, completely:

| purpose | variables |
|---|---|
| read by the runtime for itself | `LAGENT_STATE_DIR`, `LAGENT_PROVIDER`, `LAGENT_BASE_URL`, `LAGENT_MODEL`, `LAGENT_API_KEY`, `LAGENT_REASONING_EFFORT`, `LAGENT_LLM_TRACE`, `LAGENT_MCP_STDERR` |
| exported by the runtime for children | `LAGENT_WORK_DIR`, `LAGENT_PROJECT_DIR`, `LAGENT_SESSION_ID` |

`LAGENT_API_KEY` is in the first column. It is the variable R01 named,
and it is removed from every child by the rule below without anything
having to recognise the word `key`.

The original prefix rule was not wrong about the domain. It was
inverted: it *kept* everything `LAGENT_*` and guessed about the rest.
The correct reading of the same domain is to *remove* everything
`LAGENT_*` that is not one of the three exports, and to guess about
nothing.

*Amended 2026-09-13, after the release review: §Context's "Nobody else
reads `LAGENT_*`" needs a caveat. No tool outside this repository reads
`LAGENT_*` today (checked across the workspace), but the sibling design
has a counter-example: `gem-usage-lens` reads gem-agent's
`GEMAGENT_STATE_DIR` on purpose, so that an isolated runtime is
measured where it actually writes, and gem-agent's ADR-0087 carries the
same amendment. The decision stands for both: a configuration variable
stays in the removed half, and a sibling that needs a fact about the
session takes it as a parameter rather than inheriting it — the same
rule that keeps a nested runtime from taking its identity from an
environment it did not choose. If a lagent-reading companion is ever
written, it gets a flag, not an inherited variable.*

## Decision

### 1. The operator's environment is not touched

`sandbox.ScrubEnv`, `secretEnvRe` and `readLaneExports` are deleted,
and with them the claim in the configuration reference's environment
section. A
read-lane command receives the operator's environment as the operator
left it, because which variable the program it runs needs is not a
question this runtime can answer.

### 2. The runtime's own variables do not reach any child

A child spawned by this runtime — the read, write and operator lane
shells, the `!` shell, MCP servers and hooks — receives the parent
environment minus lagent's own configuration variables. The three
exports are added as they are today; they exist for children.

One function, applied at every spawn site, so a child cannot be added
without inheriting the rule.

*Amended 2026-09-13, writing the architecture review's second revision:
"applied at every spawn site" was a claim, not a mechanism, and four
kinds of child did not have it: the unconfined shell (`--no-sandbox`),
the startup lane probes, the sandbox availability probe and the
clipboard capture. Only the first carried real exposure — it built its
command with no environment at all, so the child inherited the parent's
whole environment including the runtime's own configuration, which in
the sibling runtime is the API key the finding behind this ADR named.
The other three read nothing in the runtime's namespace, so what was
exposed there was the sentence rather than a secret. All four now apply
the rule, and the read lane's probe extends the filtered environment
with its temporary directory rather than rebuilding it from the
parent's — an overwrite that would have quietly undone the fix. §3's test pins the two NAME lists and
cannot see a call site, which is why the false sentence survived a
green build; `TestEverySpawnSiteAppliesTheChildEnvRule` now fails when
a function builds an `exec.Cmd` without naming the helper that applies
the rule. A claim of the form "applied at every X" belongs with a test
that enumerates X.*

### 3. The partition is closed by a test, not by care

Every `LAGENT_` string literal in non-test code must appear in exactly
one of the two lists. A new variable fails the build until it is
classified as *mine* or *theirs*.

This is what R01 asked for, reached by construction. `LAGENT_API_KEY`
stops reaching children because it is in the runtime's namespace and is
not an export — not because a regex matched it, and not because
somebody remembered to add it to a list of secrets.

### 4. The true sentence is said once

The environment lagent was launched with reaches every child it spawns.
The documents say that in place of a protection claim, and name the
remedy the operator has: launch it from a shell that does not hold what
you would not give the model, or drop the variable at launch.

## Consequences

- A bare `env` in the read lane now prints the operator's exported
  variables to the model. That was already true of the write lane, the
  operator lane, MCP servers and hooks; the read lane stops being the
  exception that implied a rule.
- One unbounded-domain rule is deleted and none replaces it.
- `LAGENT_API_KEY` no longer reaches any child by any route that passes
  through the environment. **This does not close the configuration-file
  route**: the key also lives in `.lagent.toml`, which the read lane can
  read, and ADR-0015 records that residue as accepted by the operator
  rather than scheduled. This ADR does not change that.
- **A nested `lagent` in a shell lane no longer inherits the parent's
  configuration variables.** It reads its own configuration file, as a
  separately launched runtime should; the runtime identity of a child is
  a parameter, not something inferred from an inherited environment.
- An MCP server that was reading an ambient `LAGENT_*` value gets it
  from the `env` block of the MCP configuration instead, which is where
  a server's own inputs belong.
- The bench harness sets `LAGENT_STATE_DIR` for the runtime it
  launches, which is unaffected: the rule governs what lagent hands to
  the children *it* spawns, not what a parent hands to lagent.
- gem-agent takes the same decision as its ADR-0087. There the runtime
  namespace holds no secret today; here it holds the one the review
  found, which is why the rule is worth more to lagent than to it.

## Alternatives considered

- **Keep the denylist and add the missing words.** Rejected: it is the
  road the operator named, and `OPENAI_KEY` shows it does not end.
- **An allowlist for the read lane.** Rejected: §Context — the needed
  set is as unbounded as the secret set, and it fails by breaking work.
- **Scrub every child.** Rejected: it would break `gh`, `aws` and any
  tool the operator legitimately drives from the write or operator
  lane, and MCP servers that are given credentials on purpose.
- **Move `LAGENT_API_KEY` out of the environment entirely, config file
  only.** Rejected as a solution to this question: the review already
  noted that a key in a readable configuration file is the same problem
  wearing different clothes, and ADR-0015 accepted that residue
  explicitly. This ADR closes the environment route and says so without
  claiming the other.
- **Do nothing, document the truth only.** That is §1 and §4 and would
  have been the whole ADR. §2 is added because one bounded set exists
  and costs nothing to handle correctly.
