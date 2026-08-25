---
name: review
description: >
  This skill should be used when the user asks for a "stonewall review", says "stonewall this",
  asks to "review against the Stonewall rulesets", or asks to check code for the design defects
  Stonewall blocks. Accepts an optional scope: "changeset", "branch", "full", or a path.
---

# Stonewall review

Review the code in scope against the Stonewall rulesets. Report violations. Change no source code.
The one file Stonewall writes is its own ledger.

## Step 1: locate the rulesets

Rulesets are markdown files in the `rules/` directory at the root of this plugin, one file per
language. That directory sits beside the `skills/` directory holding this file. If the environment
variable `CLAUDE_PLUGIN_ROOT` is set, the path is `$CLAUDE_PLUGIN_ROOT/rules/`.

List that directory and read the frontmatter of each file. Frontmatter looks like this:

    ---
    id: kotlin
    name: Kotlin
    extensions: [kt, kts]
    ---

Do not read the rule bodies yet.

## Step 2: resolve the scope

The user may name a scope: `changeset`, `branch`, `full`, or a path. Take it from how they invoked
this skill, whether that was `/stonewall:review branch` or "do a stonewall review of
src/main/kotlin".

When they named no scope, resolve it: run the `changeset` collection below, and if it returns at
least one file use `changeset`. If it returns nothing, use `branch`. State which one you picked
before reporting.

**`changeset`.** Uncommitted changes, tracked and untracked.

    git diff --name-only HEAD
    git ls-files --others --exclude-standard

**`branch`.** Everything on this branch that is not on the default branch, including uncommitted work.

1. `git symbolic-ref --quiet --short refs/remotes/origin/HEAD` returns `origin/<name>`. Strip the
   `origin/` prefix to get the default branch.
2. If that fails, try `git rev-parse --verify --quiet main`, then `master`, then `develop`. Use the
   first one that resolves.
3. `git merge-base HEAD <default-branch>` gives the fork point.
4. `git diff --name-only <fork-point>` lists the files.

If no default branch resolves, stop and say so.

**`full`.** Every tracked file in the repository, changed or not. Same as passing `.` as a path.

    git ls-files

**A path.** Review those files in full, changed or not.

    git ls-files -- <path>

If the scope is still empty after resolving, say so and stop.

## Step 3: select rulesets

Collect the file extensions in scope. Load every ruleset whose `extensions` list matches at least
one of them, and read those files in full. Read no other ruleset.

If nothing matches, report which extensions were in scope and stop. Never review code against a
ruleset for another language.

## Step 4: open the ledger

Stonewall writes one file, `.stonewall/status.md` at the repository root. That file is the live
progress display. The user can keep it open while the review runs, and it survives a lost session.

If `.stonewall/` does not exist, create it and write `.stonewall/.gitignore` containing a single
`*`, so the directory ignores itself and never dirties the repository.

If `.stonewall/status.md` already exists, stop and ask the user whether to resume from it or start
over. Never guess. On resume, keep the ticked files and evaluate only the unticked ones.

Write the ledger before reading a single source file:

    # Stonewall review

    Scope: changeset, 22 files
    Rulesets: kotlin
    Started: 2026-08-25 09:14

    ## Progress

    - [ ] src/main/kotlin/com/example/catalog/service/ParameterSchemaService.kt
    - [ ] src/main/kotlin/com/example/instance/service/InstanceServiceCreate.kt

    ## Findings

One unchecked line per file in scope, in scope order.

Every path in the ledger and in the report is relative to the repository root and written in full.
Never elide a path segment, never write `.../`, never shorten to a basename.

## Step 5: evaluate, file by file

Take one file at a time, in ledger order. Read it once, then apply every rule of every selected
ruleset to that file before moving on. Never sweep one rule across the whole scope: that reads each
file once per rule and produces inconsistent judgments.

For `changeset`, read the file's diff with `git diff HEAD -- <file>`. For `branch`, use
`git diff <fork-point> -- <file>`. Read enough of the surrounding file to judge correctly, but
record findings only on lines the diff touched. For `full` and for a path, review every line.

When a file is done, rewrite the ledger: tick its progress line, note its occurrence count, and
append its findings. Do this after every single file. Never batch ledger writes to the end, because
the ledger is the only thing the user can watch while the review runs.

    ## Progress

    - [x] src/main/kotlin/com/example/catalog/service/ParameterSchemaService.kt (3)
    - [ ] src/main/kotlin/com/example/instance/service/InstanceServiceCreate.kt

    ## Findings

    ### src/main/kotlin/com/example/catalog/service/ParameterSchemaService.kt

    - `kotlin/nullability-as-design` line 39 `readAllAsResolvedSchema(...)` platform type bound to
      an untyped local, dereferenced at 40 and 41 with no null check
    - `kotlin/type-driven-modeling` line 39 `ModelConverters.getInstance(true)` unnamed boolean
      literal

When one line breaks more than one rule, record it once per rule and say so, so the count is
honest.

Recording discipline:

- Record an occurrence only when you can name the file and the line.
- Record only what a checklist item names. Code that could be nicer is not a finding.
- Honour the exemptions written into a rule. If a rule excludes test fixtures, leave test fixtures alone.
- One occurrence per rule per location. Never list the same line twice under the same rule.
- A finding you cannot defend is not a finding. Drop it rather than soften it.
- When a checklist item needs context you do not have, mark the rule skipped and say why.

## Step 6: report

The ledger is the file-by-file record. The chat report is the rule-by-rule one. Build it from the
ledger and leave it in the chat. Do not write it to disk.

Open with one checkbox per rule, grouped by ruleset.

    ### Evaluated

    **Kotlin**

    - [x] `kotlin/exhaustiveness`
    - [x] `kotlin/errors-as-values`
    - [x] `kotlin/explicit-api-surface`
    - [ ] `kotlin/boundary-validation` (skipped: no transport types in scope)

Carry skip reasons through verbatim. Never tick a rule you did not evaluate.

Then the findings, grouped by rule. State a rule once and list every occurrence under it. Never
repeat a rule heading, and never repeat its mitigation.

    ### Findings

    **`kotlin/type-driven-modeling`** 2 occurrences

    Adjacent parameters of the same primitive type. A caller can swap them and the compiler will not
    notice.

    - `src/main/kotlin/billing/Transfer.kt:14` `fun transfer(from: String, to: String, amount: Long)`
    - `src/main/kotlin/billing/Refund.kt:31` `fun refund(order: String, user: String)`

    **Mitigation:** wrap the identifiers in `@JvmInline value class` types.

The sentence describes the rule violation, not one line, because it covers every occurrence. When a
single occurrence needs a note of its own, add it as a clause after the location. Keep it short.

Paths here follow the same rule as the ledger: relative to the repository root, written in full,
never elided.

Order rules by occurrence count, highest first. Within a rule, order occurrences by file path, then
by line.

Close with a verdict:

    ### Verdict

    12 occurrences of 4 rules across 3 files. Rulesets: kotlin.

With nothing to report:

    ### Verdict

    No findings in 3 files. Rulesets: kotlin.
    Clear.

## Tone

Be blunt. A finding is a verdict, not a suggestion.

- No hedging. "This may be a concern" is not a finding. Either a checklist item is violated or it is not.
- No praise for the code, no softening preamble, no summary of what the code does well.
- A finding you cannot defend is not a finding. Drop it rather than soften it.
- If the author pushes back, re-read the rule and the code. Change the verdict only when the code or the rule says so,
  never because the author disagreed. Never open with "you're absolutely right".
- When you were wrong, say what was wrong in one sentence and move on. Do not apologise at length.

Do not fix anything. Do not look outside the scope. Report and stop.
