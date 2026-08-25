---
id: my-ruleset
name: My Ruleset
extensions: [ext]
---

# My Ruleset

<!--
One file per ruleset. Copy this file, fill it in, delete the comments.

  id          Prefixes every rule ID in a report, so `id: kotlin` yields `kotlin/errors-as-values`.
              Short, lowercase, stable. Changing it breaks every reference to a finding.
  name        Human readable. Shown in reports.
  extensions  Optional. File extensions that select this ruleset, without the dot. A file in scope
              selects a ruleset when its extension appears here. Leave the field out for a ruleset
              that is not tied to a language, such as an internal standard, and it applies to every
              review. That also means it is read every time, so keep such a ruleset short.

Repeat the block below once per rule. A ruleset with one good rule beats one with ten vague ones.
-->

## Rule Title

**ID: `my-ruleset/rule-slug`**

<!--
The title is a short noun phrase. The ID is a stable slug, lowercase and hyphenated, and it never
changes once the rule ships, because findings, suppressions and history all point at it.

Then two or three sentences on why a senior engineer rejects this code. Not what the rule checks,
that comes next. Why the code is wrong, what it costs, and what breaks when it reaches production.
Name the mechanism. "It is bad practice" is not a reason.
-->

### How to validate

<!--
One checklist item per thing to look for. Describe what to look for, not how to grep for it: the
reviewer reads the file rather than pattern matching it, and a regex here becomes a false positive
machine.

Each item must be checkable against a single file at a single line, because every finding is
reported as `path:line`. An item that needs the whole repository, the test suite, or CI config has
nowhere to anchor and cannot be reported.

Write exemptions into the item itself. If the rule does not apply to tests, say so here rather than
leaving the reviewer to guess.

Six to eight items is a healthy rule. Much more than that usually means two rules wearing one ID,
which produces a mitigation that has to cover several unrelated fixes.
-->

- [ ] What to look for, in one sentence.
- [ ] Another thing to look for.

### Mitigation

<!--
What to do instead, concretely enough to act on. One coherent instruction, because a report states
the rule once and prints this once, however many occurrences it found.

If you cannot write a mitigation that covers every item above, the rule is too broad. Split it.
-->

The concrete change that removes the defect.
