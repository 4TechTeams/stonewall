<div align="center">

<img src="docs/logo.svg" alt="Stonewall" width="128" height="128">

# Stonewall<span style="opacity:.45">.sh</span>

**Kernel-enforced sandbox for AI coding agents**

[![Build](https://img.shields.io/github/actions/workflow/status/stonewall-sh/stonewall/build.yml?branch=main&style=flat-square&logo=githubactions&logoColor=white&label=build)](https://github.com/stonewall-sh/stonewall/actions/workflows/build.yml)
[![Release](https://img.shields.io/github/v/release/stonewall-sh/stonewall?style=flat-square&logo=github&label=release)](https://github.com/stonewall-sh/stonewall/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-E4572E.svg?style=flat-square)](https://github.com/stonewall-sh/stonewall/blob/main/LICENSE)
[![Linux](https://img.shields.io/badge/Linux-bubblewrap-FCC624?style=flat-square&logo=linux&logoColor=black)](https://github.com/stonewall-sh/stonewall/releases/latest)
[![macOS](https://img.shields.io/badge/macOS-sandbox--exec-000000?style=flat-square&logo=apple&logoColor=white)](https://github.com/stonewall-sh/stonewall/releases/latest)

</div>

---

## 🧱 What it is

Stonewall is a local sandbox for AI coding agents, drastically limiting access to tools, paths and project files, based
on strictly enforced policies.

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
agent can read history but never commit, reset or rewrite it. Mark `.env` or `secrets/` hidden and the agent cannot read
them at all, no matter how politely it asks. Stop worrying about a stray `git push --force` or a leaked API key:
the kernel refuses, not the agent.

### Files outside the Project

Outside the project there is nothing to see. Your home directory does not exist for the agent: `~/.ssh`, `~/.aws`, your
shell history, your other projects, all gone. Only what the policy explicitly exposes comes back, read-only or
read-write, such as the agent's own settings under `~/.claude`. System directories stay readable so the allowed tools
keep working.

## 📦 Install

```
curl -fsSL https://stonewall.sh/install.sh | sh
```

Installs the latest release to `/usr/local/bin` (override with `STONEWALL_INSTALL_DIR`) and, on Linux, `bubblewrap`
with your package manager. The script is
[short](https://github.com/stonewall-sh/stonewall/blob/main/install.sh), read it first.

You can also download the binary for your platform from the
[latest release](https://github.com/stonewall-sh/stonewall/releases/latest), or build from source with Go 1.27 or newer:

```
go install github.com/stonewall-sh/stonewall/v2@latest
```

## ⚡ Usage

```
cd my-project
stonewall claude
```

The first run writes a starter `.stonewall.yml` policy, depending on the agent your run. The base policy is
intentionally restrictive, Adapt it to your needs.

| Flag                | What it does                                                                                                                |
|---------------------|-----------------------------------------------------------------------------------------------------------------------------|
| `-p, --policy FILE` | Use `FILE` instead of the discovered `.stonewall.yml`.                                                                      |
| `-n, --dry-run`     | Print the effective policy and the exact `bwrap` or `sandbox-exec` command for inspection, launch nothing.                  |
| `--plain`           | No colour or formatting in Stonewall's own messages. Automatic when stderr is not a terminal; `NO_COLOR` drops colour only. |
| `-v, --version`     | Print the version.                                                                                                          |
| `-h, --help`        | Show the help page. Also shown when run without arguments.                                                                  |

Options go before the agent; everything after the agent name is passed to it untouched.

The project root is the nearest directory upwards holding `.stonewall.yml` or `.git`, else the current directory. You
can launch from a subdirectory, the agent starts there.

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

`stonewall policy include https://stonewall.sh/policy/claude.yml` (or a local path) adds a policy to the include
list and, for a remote one, runs the review and caches it right away instead of at the next launch.
`stonewall policy remove …` drops it again.

See the [available official policies](https://github.com/stonewall-sh/stonewall/tree/main/policy) you can include in
your project.

## ⚙️ How it Works

Inside the sandbox:

| Path                         | Linux (`bubblewrap`)                     | macOS (`sandbox-exec`)          |
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

**Release.** CI runs gofmt, `go vet`, the unit and end-to-end tests on Ubuntu and macOS, and a snapshot cross-build on
every push. To release, tag and push:

```
git tag v2.1.0 && git push origin v2.1.0
```

The same checks run again, then GoReleaser builds Linux and macOS binaries for amd64 and arm64 and publishes them with
checksums as a GitHub release. The tag is the only version source: it is stamped into the release binaries and reported
by `go install` builds. A local `make build` says `dev`. A new major version needs the module path suffix bumped, for
example `stonewall/v2` to `stonewall/v3`, in `go.mod` and the imports.

## 🤝 Contributing

Issues and pull requests are very welcome, be it new / updated policies or actual CLI features.

## ⚖️ License

Released under the [MIT License](https://github.com/stonewall-sh/stonewall/blob/main/LICENSE).
