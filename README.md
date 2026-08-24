<div align="center">

<img src="docs/logo.svg" alt="Stonewall" width="128" height="128">

# Stonewall

**Strict CI validation to prevent sloppy code written by AI (and humans)**

[![License: MIT](https://img.shields.io/badge/License-MIT-E4572E.svg?style=flat-square)](LICENSE)
[![Build](https://img.shields.io/github/actions/workflow/status/4TechTeams/stonewall/build.yml?branch=main&style=flat-square&logo=githubactions&logoColor=white&label=build)](https://github.com/4TechTeams/stonewall/actions/workflows/build.yml)

</div>


---

## What it is

Stonewall stops code slop before it gets merged. If a senior engineer would say “ *no*”, the build fails. Not
guardrails. **Stone walls.**

## Supported Platforms

Stonewall runs on **GitLab CI** today. **GitHub Actions is planned.**

| Platform                                                                                                                     | Status       |
|------------------------------------------------------------------------------------------------------------------------------|--------------|
| ![GitLab CI](https://img.shields.io/badge/GitLab%20CI-FC6D26?style=flat-square&logo=gitlab&logoColor=white)                  | ✅ Supported |
| ![GitHub Actions](https://img.shields.io/badge/GitHub%20Actions-2088FF?style=flat-square&logo=githubactions&logoColor=white) | 🚧 Planned   |

## Technologies & Tools

| Language                                                                                            | Tools                           |
|-----------------------------------------------------------------------------------------------------|---------------------------------|
| ![Java](https://img.shields.io/badge/Java-ED8B00?style=flat-square&logo=openjdk&logoColor=white)    | `Checkstyle`, `PMD`, `SpotBugs` |
| ![Kotlin](https://img.shields.io/badge/Kotlin-7F52FF?style=flat-square&logo=kotlin&logoColor=white) | `ktlint`, `detekt`              |
| ![PHP](https://img.shields.io/badge/PHP-777BB4?style=flat-square&logo=php&logoColor=white)          | `PHPStan`, `PHP-CS-Fixer`       |

More languages and CI environments may come later.

## Contributing

Issues and pull requests are welcome. New languages, tools, and CI platforms are the highest-value contributions.

## License

Released under the [MIT License](LICENSE) © 4TechTeams.

<div align="center">
<sub>Built by <a href="https://github.com/4TechTeams">4TechTeams</a></sub>
</div>
