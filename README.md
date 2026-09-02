<div align="center">

<img src="docs/logo.svg" alt="Stonewall" width="128" height="128">

# Stonewall

**Force applied security guardrails for coding agents**

[![License: MIT](https://img.shields.io/badge/License-MIT-E4572E.svg?style=flat-square)](LICENSE)
[![Build](https://img.shields.io/github/actions/workflow/status/4TechTeams/stonewall/build.yml?branch=main&style=flat-square&logo=githubactions&logoColor=white&label=build)](https://github.com/4TechTeams/stonewall/actions/workflows/build.yml)
[![Linux](https://img.shields.io/badge/Linux-bubblewrap-FCC624?style=flat-square&logo=linux&logoColor=black)](#-install)
[![macOS](https://img.shields.io/badge/macOS-sandbox--exec-000000?style=flat-square&logo=apple&logoColor=white)](#-install)

</div>

---

## 🧱 What it is

Stonewall is a lightweight local sandbox for AI coding agents, drastically limiting access to tools, paths and project
files.

```
stonewall claude
```

Instead of running coding agents as the current user with all its power, stonewall runs the agent in a sandbox with
kernel-enforced rules. Coding agents today rely on permission prompts and their own restraint. This is not another
prompt, skill or plugin: It's a literal stone wall between the agent and everything that should not be accessed by it.

In highly configurable policies, human maintainers can configure what is accessible by the AI agent beyond blindly
trusting an `AGENTS.md` file or system prompts:

### Tools and Binaries

Stonewall restricts access to available binaries (including `$PATH`) to whitelisted tools. Stop worrying about your
agent workaround-calling a delicate tool like `make`, `git`, `python`, `rm`, etc. It virtually doesn't exist within the
sandbox.

### Project Directories and Files

Inside the project, the agent works as usual, except where the policy says otherwise. Mark `.git` read-only and the
agent can read history but never commit, reset or rewrite it. Mark `.env` or `secrets/` hidden and the agent cannot
read them at all, no matter how politely it asks. Stop worrying about a stray `git push --force` or a leaked API key:
the kernel refuses, not the agent.

### Files outside the Project

Outside the project there is nothing to see. Your home directory does not exist for the agent: `~/.ssh`, `~/.aws`,
your shell history, your other projects, all gone. Only what the policy explicitly exposes comes back, read-only or
read-write, such as the agent's own settings under `~/.claude`. System directories stay readable so the allowed
tools keep working.

## 📦 Install

| Platform | Runtime requirement                                                                       |
|----------|-------------------------------------------------------------------------------------------|
| macOS    | Nothing. `sandbox-exec` ships with the OS.                                                |
| Linux    | `bubblewrap`: `apt install bubblewrap`, `dnf install bubblewrap`, `pacman -S bubblewrap`. |

Build from source with Go 1.27 or newer:

```
go install github.com/4TechTeams/stonewall@latest
```

Prebuilt binaries are planned.

## ⚡ Usage

```
cd my-project
stonewall claude
```

The first run shows the policy it proposes to write and asks before writing it. Edit it, run again. Any agent works once
it is allowed: `stonewall codex` after adding `codex` to `bin.allowed`, or after including a policy that lists it.
Stonewall refuses an agent the policy does not allow.

| Flag                | What it does                                                                                                                |
|---------------------|-----------------------------------------------------------------------------------------------------------------------------|
| `-p, --policy FILE` | Use `FILE` instead of the discovered `.stonewall.yml`.                                                                      |
| `-n, --dry-run`     | Print the effective policy and the exact `bwrap` or `sandbox-exec` command for inspection, launch nothing.                  |
| `--plain`           | No colour or formatting in Stonewall's own messages. Automatic when stderr is not a terminal; `NO_COLOR` drops colour only. |
| `-v, --version`     | Print the version.                                                                                                          |
| `-h, --help`        | Show the help page. Also shown when run without arguments.                                                                  |

Options go before the agent; everything after the agent name is passed to it untouched. `stonewall policy update`
refreshes the cached remote policies (see below); because `policy` is a subcommand, an agent literally named
`policy` needs `stonewall -- policy`.

The project root is the nearest directory upwards holding `.stonewall.yml` or `.git`, else the current directory. You
can launch from a subdirectory; the agent starts there. Stonewall refuses to run when the root is your home directory or
`/`, and prints the root and policy file it uses on every launch.

Inside the sandbox:

| Path                         | Linux (bubblewrap)                       | macOS (sandbox-exec)            |
|------------------------------|------------------------------------------|---------------------------------|
| Project                      | read-write                               | read-write                      |
| `project.readonly` entries   | mounted read-only                        | writes denied                   |
| `project.hidden` directories | appear empty                             | reads and writes denied         |
| `project.hidden` files       | read as empty                            | reads and writes denied         |
| `$HOME`                      | empty, except `expose` entries           | denied, except `expose` entries |
| `expose.read` entries        | read-only bind                           | reads allowed, writes denied    |
| `/usr`, `/etc`, `/opt`, …    | read-only                                | unchanged, read-write           |
| `PATH`                       | a directory of allowlisted programs only | the same directory              |
| Network                      | shared with the host                     | shared with the host            |

Linux makes hidden content vanish. macOS still lists the names but denies every access. Both stop the agent from reading
the secret.

Linux builds the sandbox from nothing and mounts in only what is listed. macOS starts from the full system and denies
your home directory, so locations such as `/opt/homebrew` stay writable there.

`PATH` restriction is defence in depth, not isolation. Anything reachable by absolute path still runs, and an
allowlisted interpreter such as `bash` or `python` can run anything. No shipped policy grants a shell, an interpreter or
`git`; if your agent needs one, that is a line you add to your own policy, knowingly.

**Known limitations**

- Environment variables pass through unchanged, except `SSH_AUTH_SOCK`. Secrets in your shell environment are visible to
  the agent.
- Linux: `expose.write` single files such as `~/.claude.json` are mount points. Tools that rewrite them by rename fail
  with `EBUSY`.
- Linux: the agent keeps the controlling terminal so job control works. On kernels before 6.2 that leaves `TIOCSTI`
  input injection open.
- Tools that rewrite an `expose.read` file, such as `git config --global`, fail.
- Not yet: network isolation, seccomp, process-exec allowlisting on macOS, hiding file names on macOS.

## 📜 Policy

`.stonewall.yml` at the project root contains all rules applied. Policies can be included locally and from remote 
sources.

Example ploicy of a Node project that lets Claude Code build, test and commit:

```yaml
include:
  - https://stonewall.sh/policy/base.yml    # .git read-only, .env and secrets hidden, read-only tools
  - https://stonewall.sh/policy/claude.yml  # claude, node, the keychain login, ~/.claude
project:
  hidden:
    - .env.local        # base.yml hides .env, this project also has local overrides
  writable:
    - .git              # base.yml makes .git read-only, this project wants commits from the agent
bin:
  allowed:
    - sh                # Claude Code's Bash tool needs a shell. This widens the sandbox considerably.
    - git
    - npm
    - npx
expose:
  write:
    - ~/.npm            # npm's cache
```

Included policies apply first, your own rules last. Remote policies are reviewed once, then cached in
`.stonewall/policies/`. Run `stonewall policy update` to fetch new versions.

See the [available official policies](./policies) you can include in your project.

## 🛠️ Development

```
make build       # ./stonewall
make test        # unit tests
make e2e         # end-to-end on this machine, macOS or Linux
make e2e-linux   # same, inside Docker with bubblewrap (runs --privileged)
```

`main.go` is the CLI. `internal/policy` finds the root, loads and merges a policy with its includes, and scaffolds one.
`internal/sandbox` resolves a policy into a `Plan` of absolute host paths and renders it as `bwrap` arguments or a
Seatbelt profile. Both renderers are pure functions with golden tests. `test/e2e.sh` runs the same assertions on both
platforms.

`--dry-run` shows exactly what the backend receives. Start there when something is unexpectedly blocked or visible.

## 🤝 Contributing

Issues and pull requests are very welcome, be it new / updated policies or actual CLI features.

## ⚖️ License

Released under the [MIT License](LICENSE) © 4TechTeams.

<div align="center">
<sub>Built by <a href="https://github.com/4TechTeams">4TechTeams</a></sub>
</div>
