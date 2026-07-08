# gUtil

gUtil is a cross-platform command-line utility for safe, repeatable development workflows. Version 0.1 prepares a Git merge and opens Visual Studio Code only when manual conflict resolution is required.

## Requirements

- Git available on `PATH`.
- Visual Studio Code's `code` launcher is optional.
- The Git remote must be named `origin`.

End users do not need Go.

## Installation

### macOS and Linux

```sh
curl -fsSL https://raw.githubusercontent.com/pablo/gutil/main/scripts/install.sh | sh
```

The installer writes to `~/.local/bin`. Ensure that directory is in `PATH`.

### Windows

```powershell
irm https://raw.githubusercontent.com/pablo/gutil/main/scripts/install.ps1 | iex
```

The installer writes to `$HOME\.local\bin` and updates the user `PATH` when required. Open a new terminal afterward.

### Build from source

```sh
go install github.com/pablo/gutil/cmd/gutil@latest
```

## Commands

```text
gutil conflict <source> <target>
gutil conflict --status
gutil conflict --continue
gutil conflict --abort
gutil version
gutil help
```

For example:

```sh
gutil conflict feature/ABC develop
```

This updates both branches from `origin`, then merges `develop into feature/ABC` using `git merge --no-commit --no-ff develop`.

Before making changes, gUtil requires a clean working tree, including no untracked files, and rejects an active merge, rebase, cherry-pick, or revert. It then:

1. Refreshes remote references with `git fetch origin --prune`.
2. Checks out and pulls the target branch.
3. Checks out and pulls the source branch.
4. Prepares the merge without committing.
5. Lists conflicting files and opens Visual Studio Code when conflicts exist.

If the `code` launcher is unavailable, the conflict state is preserved and gUtil tells you to open the repository manually. A clean merge is also left uncommitted for review.

### Continue after resolving conflicts

Resolve every conflict in Visual Studio Code and ensure the resolved files are already staged. gUtil deliberately does not run `git add`.

```sh
gutil conflict --continue
```

The command verifies that the merge was started by gUtil, checks that no unresolved or unstaged conflict files remain, and creates a commit such as:

```text
[gUtil] Conflict Resolution - 4 files fixed.
```

It then runs `git push origin <source-branch>`. If commit succeeds but push fails, run `gutil conflict --continue` again to retry only the push; it will not create a second commit.

## Recovery

Inspect the current state:

```sh
gutil conflict --status
```

Cancel an active merge:

```sh
gutil conflict --abort
```

If checkout, pull, or merge preparation fails, gUtil stops immediately and preserves Git's state. Resolve the reported Git problem, inspect the repository, and rerun the command. gUtil never stashes, resets, cleans, commits, pushes, force-updates, or deletes branches automatically.

## Exit codes

- `0`: completed; conflicts may be waiting for manual resolution.
- `1`: repository, Git, or required-environment failure.
- `2`: invalid command usage.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md). Releases follow semantic versioning and are recorded in [CHANGELOG.md](CHANGELOG.md).
