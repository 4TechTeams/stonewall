# Stonewall

**Strict CI validation to prevent sloppy code written by AI (and humans)**

Stonewall exists to stop code slop before it gets merged. Whenever a senior engineer would say “no”, Stonewall should fail the build. Not guardrails, but **Stone walls** for bad code.

## Supported Platforms

Currently, Stonewall supports GitLab CI only.

## Technologies & Tools

| Language   | Tools                       |
| ---------- | --------------------------- |
| Kotlin     | ktlint, detekt              |
| TypeScript | ESLint, TypeScript compiler |
| PHP        | PHPStan, PHP-CS-Fixer       |

More languages and CI environments may come later.
