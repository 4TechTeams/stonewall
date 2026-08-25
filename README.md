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

Stonewall stops **code slop** before it gets merged. If a senior engineer would say "*no*", Stonewall says "*no*". It's
not another fluffy review skill that tells you how right you are, but a stone wall in front of bad code.

Imagine your linter passed, code compiles, tests are green. Still it models a date as a string, swallows errors, adds
empty branch defaults, etc. AI agents can generate such issues way faster than your human team can review.

As a coding agent skill, it gives you **honest**, **direct** and **ruthless** feedback about your solutions. That's it.

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

Stonewall keeps a live ledger at `.stonewall/status.md`, recording which files it has reviewed and what it found. It
holds the same findings the other way round, by file rather than by rule. Keep it open to watch a long review.

Find an existing ledger and Stonewall asks whether to resume from it or start over. A `.gitignore` beside it ignores the
ledger and nothing else, so `.stonewall/` stays free for your own rulesets. No source file is ever touched.

## 📜 Rulesets

Stonewall is based on specific rulesets per language. Some are rather generic, others are opinionated. Use, extend or
change them:

| Language                                                                                            | Rules | Status     |
|-----------------------------------------------------------------------------------------------------|-------|------------|
| ![Kotlin](https://img.shields.io/badge/Kotlin-7F52FF?style=flat-square&logo=kotlin&logoColor=white) | 10    | ✅ Ready   |
| ![Java](https://img.shields.io/badge/Java-ED8B00?style=flat-square&logo=openjdk&logoColor=white)    | –     | 🚧 Planned |
| ![PHP](https://img.shields.io/badge/PHP-777BB4?style=flat-square&logo=php&logoColor=white)          | 13    | ✅ Ready   |

Ruleset pull requests are highly welcome.

### Custom Rules

Need rules for internal standards, architecture decisions, or languages we don't cover yet?

Just drop a ruleset file in `.stonewall/rulesets/` and Stonewall loads it alongside its own. Give it an `id` of your
own, since that prefixes every rule ID it reports and a clash with a built-in ruleset stops the review. Start from
[the template](docs/templates/ruleset.md).

## 🛠️ Development

Load the plugin straight from the working tree, no install:

```
claude --plugin-dir .
```

Then use it by stating `/stonewall:review` or similar.

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

A new language just needs a file in `rulesets/` with `id`, `name`, and `extensions` in its frontmatter.

## 🤝 Contributing

Issues and pull requests are welcome. New rulesets and new rules are the highest-value contributions. A rule needs a
stable ID, a reason a senior engineer would reject the code, a checklist an agent can follow, and a concrete mitigation.

## ⚖️ License

Released under the [MIT License](LICENSE) © 4TechTeams.

<div align="center">
<sub>Built by <a href="https://github.com/4TechTeams">4TechTeams</a></sub>
</div>
