<div align="center">

<img src="docs/logo.svg" alt="Stonewall" width="128" height="128">

# Stonewall

**Your linter says pass. Stonewall says no.**

[![License: MIT](https://img.shields.io/badge/License-MIT-E4572E.svg?style=flat-square)](LICENSE)
[![Build](https://img.shields.io/github/actions/workflow/status/4TechTeams/stonewall/build.yml?branch=main&style=flat-square&logo=githubactions&logoColor=white&label=build)](https://github.com/4TechTeams/stonewall/actions/workflows/build.yml)
[![Claude Code](https://img.shields.io/badge/Claude%20Code-D97757?style=flat-square&logo=claude&logoColor=white)](#claude-code)

</div>

---

## 🧱 What it is

Stonewall stops **code slop** before it gets merged. If a senior engineer would say "*no*", Stonewall says "*no*". Not
guardrails. **Stone walls.**

Imagine your linter passed, code compiles, tests are green. Still it models a date as a string, swallows errors, adds
empty branch defaults, etc. AI agents can generate such issues way faster than your human team can review. This is where
Stonewall holds:

It runs inside your coding agent and it is **honest**, **direct** and **ruthless**. No severity levels, no "consider
refactoring", no "you're absolutely right". A finding is a wall.

## 📦 Install

### Claude Code

```
/plugin marketplace add 4TechTeams/stonewall
/plugin install stonewall@stonewall
```

Rulesets are plain markdown with no Claude Code specifics. Wrappers for other agents such as Cursor and Codex are
planned.

## ⚡ Usage

```
/stonewall:review
```

Or just ask for it:

> Do a stonewall review of src/main/kotlin

Stonewall loads only the rulesets that match the files in scope.

| Scope       | What it covers                                                                     |
|-------------|------------------------------------------------------------------------------------|
| *nothing*   | `changeset` if anything is uncommitted, otherwise `branch`.                        |
| `changeset` | Uncommitted changes, tracked and untracked.                                        |
| `branch`    | Everything on this branch that is not on the default branch, uncommitted included. |
| `full`      | Every tracked file in the repository. Same as a `.` path.                          |
| *path*      | A directory or a file, reviewed in full.                                           |

```
/stonewall:review changeset
/stonewall:review branch
/stonewall:review full
/stonewall:review src/main/kotlin
```

`branch` resolves the default branch from `origin/HEAD`, falling back to `main`, `master`, then `develop`, and diffs
from the merge base. So it covers your committed branch work *and* anything still uncommitted.

`changeset` and `branch` review a diff. `full` and a path review files as they stand, whether you touched them or not.

The report opens with a checklist of every rule evaluated, then findings grouped by rule: each rule stated once, every
occurrence listed under it, one mitigation.

Stonewall writes one file, `.stonewall/status.md`, a live ledger of which files it has reviewed and what it found. It
holds the same findings the other way round, by file rather than by rule. Keep it open to watch a long review.

Find an existing ledger and Stonewall asks whether to resume from it or start over. The directory ignores itself, so it
never dirties your repository. No source file is ever touched.

## 📜 Rulesets

| Language                                                                                            | Rules | Status     |
|-----------------------------------------------------------------------------------------------------|-------|------------|
| ![Kotlin](https://img.shields.io/badge/Kotlin-7F52FF?style=flat-square&logo=kotlin&logoColor=white) | 10    | ✅ Ready   |
| ![Java](https://img.shields.io/badge/Java-ED8B00?style=flat-square&logo=openjdk&logoColor=white)    | –     | 🚧 Planned |
| ![PHP](https://img.shields.io/badge/PHP-777BB4?style=flat-square&logo=php&logoColor=white)          | –     | 🚧 Planned |

What Kotlin blocks today:

`exhaustiveness` · `nullability-as-design` · `immutability-by-default` · `errors-as-values` · `type-driven-modeling` ·
`structured-concurrency` · `explicit-api-surface` · `injected-effects` · `collection-discipline` ·
`boundary-validation`

Rulesets are plain markdown. Every rule carries a stable ID, the reasoning behind it, a validation checklist, and a
mitigation. Read them, fork them, argue with them.

```markdown
## Errors as Values

**ID: `kotlin/errors-as-values`**

Why a senior engineer cares.

### How to validate

- [ ] What to look for in the code.

### Mitigation

What to do instead.
```

The ID is stable. Use it to reference a finding or track it over time.

## 🛠️ Development

Load the plugin straight from the working tree, no install:

```
claude --plugin-dir .
```

`/stonewall:review` then runs your local `skills/` and `rules/`.

Validate before pushing. CI runs the same three:

```
claude plugin validate .claude-plugin/plugin.json --strict
claude plugin validate .claude-plugin/marketplace.json --strict
claude plugin validate skills --strict
```

To exercise the real install path:

```
claude plugin marketplace add ./
claude plugin install stonewall@stonewall
```

A new language is one file in `rules/` with `id`, `name`, and `extensions` in its frontmatter. Nothing else changes.

## 🤝 Contributing

Issues and pull requests are welcome. New rulesets and new rules are the highest-value contributions. A rule needs a
stable ID, a reason a senior engineer would reject the code, a checklist an agent can follow, and a concrete mitigation.

## ⚖️ License

Released under the [MIT License](LICENSE) © 4TechTeams.

<div align="center">
<sub>Built by <a href="https://github.com/4TechTeams">4TechTeams</a></sub>
</div>
