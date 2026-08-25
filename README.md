<div align="center">

<img src="docs/logo.svg" alt="Stonewall" width="128" height="128">

# Stonewall

**Ruleset-driven code review that stops AI slop before it merges**

[![License: MIT](https://img.shields.io/badge/License-MIT-E4572E.svg?style=flat-square)](LICENSE)
[![Build](https://img.shields.io/github/actions/workflow/status/4TechTeams/stonewall/build.yml?branch=main&style=flat-square&logo=githubactions&logoColor=white&label=build)](https://github.com/4TechTeams/stonewall/actions/workflows/build.yml)

</div>


---

## What it is

Stonewall is a plugin for coding agents. It reviews your code against explicit rulesets that encode what a senior
engineer would reject.

Linters catch formatting and known bug patterns. They do not catch a `String` standing in for a domain identifier, a
`when` whose `else` branch hides a missing case, or a coroutine launched outside any lifecycle. Stonewall does. If a
senior engineer would say "*no*", the review says
"*no*". Not guardrails. **Stone walls.**

Rulesets are plain markdown. Every rule carries a stable ID, the reasoning behind it, a validation checklist, and a
mitigation. Read them, fork them, argue with them.

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

Every rule follows the same shape:

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
