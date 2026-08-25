---
description: Review code against the Stonewall rulesets and report every violation
argument-hint: "[changeset | branch | <path>]"
allowed-tools: Bash(git:*), Read, Grep, Glob
---

# Stonewall review

Review the code in scope against the Stonewall rulesets. Report violations. Change nothing.

## Step 1: locate the rulesets

Rulesets are markdown files in the `rules/` directory at the root of this plugin, one file per
language. That directory is a sibling of the `commands/` directory holding this file. If the
environment variable `CLAUDE_PLUGIN_ROOT` is set, the path is `$CLAUDE_PLUGIN_ROOT/rules/`.

List that directory and read the frontmatter of each file. Frontmatter looks like this:

    ---
    id: kotlin
    name: Kotlin
    extensions: [kt, kts]
    ---

Do not read the rule bodies yet.

## Step 2: resolve the scope

The argument is: `$ARGUMENTS`

An empty argument means `changeset`.

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

**Anything else.** Treat the argument as a directory or file path. Review those files in full,
changed or not.

    git ls-files -- "$ARGUMENTS"

If the scope is empty, say so and stop.

## Step 3: select rulesets

Collect the file extensions in scope. Load every ruleset whose `extensions` list matches at least
one of them, and read those files in full. Read no other ruleset.

If nothing matches, report which extensions were in scope and stop. Never review code against a
ruleset for another language.

## Step 4: review

Work through every rule of every loaded ruleset. Each rule carries a `How to validate` checklist.
Apply each checklist item to the files in scope.

For `changeset`, read the diff with `git diff HEAD -- <file>`. For `branch`, use
`git diff <fork-point> -- <file>`. Read enough of the surrounding file to judge correctly, but
report findings only on lines the diff touched. For a path argument, review every line.

Reporting discipline:

- Report a finding only when you can name the file and the line.
- Report only what a checklist item names. Code that could be nicer is not a finding.
- Honour the exemptions written into a rule. If a rule excludes test fixtures, leave test fixtures alone.
- One finding per rule per location. Never repeat the same rule for the same line.
- When a checklist item needs context you do not have, say so instead of guessing.

## Step 5: report

Group findings by file, in scope order. Use this shape:

    ### src/main/kotlin/billing/Transfer.kt

    **`kotlin/type-driven-modeling`** line 14

    ```kotlin
    fun transfer(from: String, to: String, amount: Long)
    ```

    Two adjacent `String` parameters. A caller can swap them and the compiler will not notice.

    **Mitigation:** wrap both identifiers in `@JvmInline value class` types.

The rule ID, the location, the offending code, one sentence on what is wrong, and the mitigation
from the ruleset. Nothing else.

Close with a verdict:

    ### Verdict

    7 findings in 3 files. Rulesets: kotlin.
    Stonewall says no.

With nothing to report:

    ### Verdict

    No findings in 3 files. Rulesets: kotlin.
    Clear.

Do not fix anything. Do not look outside the scope. Report and stop.
