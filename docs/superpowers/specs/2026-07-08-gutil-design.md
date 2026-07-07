# gUtil Design Specification

## Purpose

gUtil is a versioned, installable, cross-platform command-line utility for automating repetitive development workflows. The first release focuses on preparing a Git merge so a developer can resolve conflicts directly in Visual Studio Code.

All source code, terminal output, help text, errors, and documentation must be written in English.

## Initial Scope

Version 0.1 provides:

- `gutil conflict <source> <target>`
- `gutil conflict --status`
- `gutil conflict --abort`
- `gutil version`
- `gutil help`

Other Git, Salesforce, and GitHub Actions utilities are explicitly outside this release. The internal structure must allow new subcommands without changing the existing public interface.

## Platform and Runtime

gUtil is implemented in Go using the standard library. It is distributed as a standalone executable, so end users do not need Go installed.

Supported release targets are:

- macOS ARM64 and AMD64
- Linux ARM64 and AMD64
- Windows ARM64 and AMD64

Git is a required external dependency. Visual Studio Code's `code` launcher is optional and is used only when conflicts are detected.

## Architecture

The repository uses a modular internal architecture:

```text
gutil/
├── cmd/gutil/              # Minimal executable entry point
├── internal/cli/           # Argument parsing, dispatch, help, and exit codes
├── internal/commands/
│   └── conflict/           # Conflict workflow orchestration
├── internal/git/           # Typed Git operations
├── internal/process/       # External process execution abstraction
├── internal/output/        # Consistent English terminal output
├── docs/                   # User and contributor documentation
├── scripts/                # Installation scripts
├── go.mod
└── README.md
```

The command layer coordinates the workflow but does not invoke operating-system processes directly. The Git layer exposes operations such as repository validation, branch discovery, checkout, pull, merge, status, and abort. The process layer makes external execution testable. The output layer owns user-facing formatting.

The CLI uses an internal standard-library dispatcher. A third-party CLI framework and external plugin discovery are not required for the initial scope.

## Command Semantics

### Prepare a conflict merge

```text
gutil conflict <source> <target>
```

The command merges the target branch into the source branch. For example, `gutil conflict feature/ABC develop` finishes on `feature/ABC` with `develop` merged using `--no-commit --no-ff`.

The workflow is:

1. Validate that exactly two non-empty branch arguments were supplied.
2. Reject identical source and target branch names.
3. Verify that `git` is available and the current directory belongs to a Git working tree.
4. Reject an existing merge, rebase, cherry-pick, or revert operation.
5. Require a completely clean working tree, including no untracked files.
6. Run `git fetch origin --prune` to refresh remote-tracking references and remove stale ones. This does not alter local branches or working-tree files.
7. Resolve the target branch. If it exists only as `origin/<target>`, create a local tracking branch. If it exists nowhere, stop with an actionable error.
8. Check out the target branch and run `git pull origin <target>`.
9. Resolve the source branch using the same rules.
10. Check out the source branch and run `git pull origin <source>`.
11. Run `git merge --no-commit --no-ff <target>`.
12. Inspect unmerged files using `git diff --name-only --diff-filter=U`.
13. If conflicts exist, print them and attempt to launch `code .`.
14. If no conflicts exist and Git succeeded, leave the uncommitted merge ready for review and commit.

The remote is always named `origin` in version 0.1. Pull behavior deliberately matches `git pull`; gUtil does not enforce fast-forward-only behavior or rewrite branch history.

### Status

```text
gutil conflict --status
```

This command validates the repository, prints `git status`, and lists all unmerged files. It does not modify repository state.

### Abort

```text
gutil conflict --abort
```

This command validates the repository and verifies that a merge is active before executing `git merge --abort`. It must not claim success if no merge exists or Git cannot abort it.

### Version and help

`gutil version` prints the build version. Development builds use an explicit development value. Release builds receive the semantic version through linker build metadata.

`gutil help`, `gutil --help`, and invalid invocations display concise usage and the available commands. Invalid invocations return a non-zero exit status.

## Safety and Failure Behavior

gUtil never stashes, resets, cleans, commits, pushes, force-updates, or deletes local or remote branches automatically.

Each external command failure stops the workflow immediately, except that a non-zero merge result is inspected to distinguish expected merge conflicts from other failures. gUtil preserves the state left by Git and reports:

- The operation that failed.
- The relevant Git output.
- The repository's current state when useful.
- Concrete recovery steps.
- Whether the user can fix the issue and rerun gUtil or should abort an active merge.

If `code .` fails or the launcher is unavailable after conflicts have been prepared, gUtil returns a warning and instructs the user to open the repository manually. It does not abort or alter the merge.

Stable categories cover invalid environment, dirty working tree, operation already active, missing branch, checkout failure, pull failure, merge failure without conflicts, abort failure, and optional editor-launch failure. User-facing errors remain specific rather than exposing only category names.

## Exit Codes

- `0`: requested operation completed, including a merge that produced conflicts and is ready for manual resolution.
- `1`: operational failure caused by Git, repository state, or an unavailable required dependency.
- `2`: invalid CLI usage.

Failure to launch the optional `code` command after conflicts does not change the successful conflict-preparation exit code.

## Testing

Unit tests cover argument parsing, command dispatch, error mapping, output decisions, and workflow sequencing through a simulated process executor.

Integration tests create temporary Git repositories and local bare remotes. They cover:

- Local branches and branches that exist only in `origin`.
- Missing branches.
- Dirty tracked, staged, and untracked files.
- Existing Git operations.
- Successful pulls and pull failures.
- A clean merge prepared without a commit.
- A real conflicting merge and conflict-file reporting.
- Status during a conflict.
- Successful and invalid abort operations.
- Missing `code` launcher behavior.

GitHub Actions runs formatting checks, static analysis, unit tests, and supported integration tests on macOS, Linux, and Windows. Process and filesystem behavior must not rely on POSIX shell syntax.

## Distribution

The project uses semantic versioning. A release tag such as `v0.1.0` triggers GitHub Actions to test and build all supported platform and architecture combinations, then publish archives and a checksum manifest to a GitHub Release.

Installation methods are:

- A shell installer for macOS and Linux that detects the platform and downloads the matching release to `~/.local/bin` by default.
- A PowerShell installer for Windows that downloads the matching release to a per-user directory and adds that directory to the user's `PATH` when necessary.
- `go install` for contributors and developers who already have Go.

Installation scripts must support a pinned version, verify the downloaded checksum, avoid requiring administrator privileges by default, and fail without partially replacing a working installation.

Publishing to GitHub is a separate, explicitly authorized action. Local implementation does not imply permission to create a remote repository, push commits, or publish releases.

## Documentation

The repository includes:

- A README with purpose, prerequisites, installation, command examples, safety guarantees, and recovery procedures.
- A changelog following semantic versions.
- A contributing guide describing local development, tests, and how to add a subcommand.
- A license selected before public distribution.

## Acceptance Criteria

The initial release is complete when:

1. The five scoped command interfaces behave as specified.
2. Conflict preparation works against a real temporary remote on macOS, Linux, and Windows CI.
3. No workflow silently modifies or discards unrelated user work.
4. Conflicts are listed and VS Code is opened only when conflicts exist.
5. All user-facing content is in English.
6. Release automation produces standalone binaries and checksums for every supported target.
7. Both installers verify integrity and make `gutil` available from a new terminal session.
8. Documentation explains installation, normal use, failure recovery, and contribution.

