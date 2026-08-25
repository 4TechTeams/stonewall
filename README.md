<div align="center">

<img src="docs/logo.svg" alt="Stonewall" width="128" height="128">

# Stonewall

**Your linter says pass. Stonewall says no.**

[![License: MIT](https://img.shields.io/badge/License-MIT-E4572E.svg?style=flat-square)](LICENSE)
[![Build](https://img.shields.io/github/actions/workflow/status/4TechTeams/stonewall/build.yml?branch=main&style=flat-square&logo=githubactions&logoColor=white&label=build)](https://github.com/4TechTeams/stonewall/actions/workflows/build.yml)

</div>

---

## What it is

Stonewall stops code slop before it gets merged. If a senior engineer would say "*no*", Stonewall says "*no*". Not
guardrails. **Stone walls.**

Your linter already passed this file. It compiles, the tests are green, and it still models a user ID as a `String`,
hides a missing branch behind `else -> null`, and launches a coroutine that outlives the request that started it. None
of that is a style violation. All of it should block the merge. AI writes this code faster than any team can review it.

Stonewall reviews against rulesets that encode the arguments senior engineers actually have. Exhaustiveness.
Nullability as a contract. Errors in the return type. Effects injected, never reached for. Ten rules for Kotlin today,
each one written down with the reasoning, a checklist an agent can follow, and the fix.

No severity levels. No "consider refactoring". A finding is a wall.

## Install

Claude Code:

```
/plugin marketplace add 4TechTeams/stonewall
/plugin install stonewall@stonewall
```

Rulesets are plain markdown with no Claude Code specifics. Wrappers for other agents such as Cursor and Codex are
planned.

## Usage

```
/stonewall:review
```

Stonewall loads only the rulesets that match the files in scope.

| Argument    | Scope                                                          |
|-------------|----------------------------------------------------------------|
| *omitted*   | Uncommitted changes. Same as `changeset`.                      |
| `changeset` | Uncommitted changes.                                           |
| `branch`    | Diff against the default branch (`main`, `master`, `develop`). |
| *path*      | A directory or a file, reviewed in full.                       |

```
/stonewall:review changeset
/stonewall:review branch
/stonewall:review src/main/kotlin
```

`changeset` and `branch` review a diff. A path reviews the files as they stand, whether you touched them or not.

Findings are reported per rule ID, with the location and the mitigation from the ruleset.

## Rulesets

| Language                                                                                            | Rules | Status  |
|-----------------------------------------------------------------------------------------------------|-------|---------|
| ![Kotlin](https://img.shields.io/badge/Kotlin-7F52FF?style=flat-square&logo=kotlin&logoColor=white) | 10    | Ready   |
| ![Java](https://img.shields.io/badge/Java-ED8B00?style=flat-square&logo=openjdk&logoColor=white)    | –     | Planned |
| ![PHP](https://img.shields.io/badge/PHP-777BB4?style=flat-square&logo=php&logoColor=white)          | –     | Planned |

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

The ID is stable. Use it to reference a finding, suppress a rule, or track it over time.

## Contributing

Issues and pull requests are welcome. New rulesets and new rules are the highest-value contributions. A rule needs a
stable ID, a reason a senior engineer would reject the code, a checklist an agent can follow, and a concrete mitigation.

## License

Released under the [MIT License](LICENSE) © 4TechTeams.

<div align="center">
<sub>Built by <a href="https://github.com/4TechTeams">4TechTeams</a></sub>
</div>
