# ADR-0040: a skill's description is read as Claude Code reads it — up to 1,024 characters, multi-line YAML included

| Field | Value |
|-------|-------|
| Status | **Proposed** (2026-11-04) |
| Date | 2026-11-04 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | gem-agent Issue #3 (2026-11-04): skill descriptions cut at 400 runes lose the triggers of skills written to the 1,024-character limit. lagent's `internal/skills` is the same code |
| Ports | gem-agent ADR-0106 (Accepted 2026-11-04, released in gem-agent v0.98.0), at gem-agent `ac5d03e` |
| Amends | ADR-0011 (the description line it left without a length) |
| Relates to | ADR-0003 (the catalog rides the runtime-facts message), ADR-0034 (one round is bounded by the window) |

## Context

ADR-0011 reads skills in Claude Code's format, as-is. One description line per skill rides the runtime-facts message (ADR-0003), and that line alone decides whether the skill is loaded. `internal/skills` is ported from gem-agent and, checked against `f8bdb7e`, its code differs from gem-agent's before ADR-0106 only in how the catalog is rendered. It therefore has both of the defects gem-agent ADR-0106 found:

1. **The description is cut at 400 runes, silently** (`DefaultLimits().MaxDescription`). Claude Code and the organization's skill validator accept 1,024 characters. Six of the nine skills in skills-series are longer than 400 (mcp-tactics 1,020, news-digest 644, compliance-review 594, weather-forecast 580, incident-review 578, incident-research 509); past the cut go, for example, mcp-tactics' BigQuery, browser, audio and every Japanese trigger.
2. **A multi-line description is read as its first line.** `description: >` followed by indented lines is read as the description `>`.

## Decision

lagent adopts gem-agent ADR-0106 as implemented at `ac5d03e`:

1. **`MaxDescription` becomes 1,024.** A longer description is still cut, and the cut is a discovery note: `skill "<name>": description is <n> characters; only the first 1024 are listed`.
2. **`parseFrontmatter` reads multi-line values**: block scalars (`>` folds lines with spaces and a blank line becomes a line break; `|` keeps line breaks; chomping indicators and an indentation digit are accepted) and plain scalars continued on more-indented lines (folded with spaces). The common indentation is removed, and the block ends at the first line that is neither indented nor blank. Quoted values spanning lines, anchors and flow collections stay unread; a full YAML parser is a dependency the organization does not take for one field.
3. **The catalog line stays one line.** A description that keeps line breaks (`|`, or a blank line inside `>`) is joined with spaces before it is listed, so no line of the facts message is the skill author's (independent review, ADR-0039 §6).

**Under ADR-0034.** The catalog is part of the facts message, which the round budget counts as fixed cost. For the nine skills of skills-series it grows by about 1,700 characters, and `MaxSkills` (100) still bounds the number of lines. On a small window that is real cost, but a skill whose triggers are hidden costs more: it never loads when it should.

## Consequences

- Skills written to the platform's limit trigger in lagent on everything they declare.
- The facts message grows with long descriptions, within the bounds above.
- A description longer than 1,024 characters is cut with a note, not silently.

## Alternatives considered

As in gem-agent ADR-0106: keep 400 and add a note (six of the nine shipped skills would carry it, their triggers unseen); no limit (an unbounded line in every session); a YAML library (a third-party dependency for one field).

## References

- gem-agent ADR-0106, Issue #3 and `ac5d03e` (the porting source)
- `internal/skills/skills.go` (`DefaultLimits`, `readSkill`, `parseFrontmatter`)
