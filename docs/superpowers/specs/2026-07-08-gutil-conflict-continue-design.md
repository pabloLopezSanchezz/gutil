# gUtil Conflict Continue Design Specification

## Purpose

Extend the gUtil conflict workflow with `gutil conflict --continue`. The command completes a conflict merge prepared by gUtil after the user resolves and stages every conflicted file in Visual Studio Code.

All source code, terminal output, help text, commit messages, and documentation remain in English.

## Public Interface

```text
gutil conflict --continue
```

The command accepts no additional arguments. Invalid combinations return exit code `2` and display conflict usage.

## Ownership Boundary

`--continue` operates only on merges initiated by `gutil conflict <source> <target>`. It rejects manual merges and gUtil state that does not match the active repository operation.

gUtil never stages files during continuation. The user must resolve and stage every conflicted file in Visual Studio Code before running the command.

## Persisted Merge State

When conflict preparation detects one or more unmerged files, gUtil writes an internal JSON state file containing:

- Schema version.
- Source branch.
- Target branch.
- Source `HEAD` commit recorded immediately before `git merge`.
- Target merge commit read from `MERGE_HEAD` after Git reports conflicts.
- Exact sorted list of files that were conflicted.
- Phase: `resolving` or `committed`.
- Commit identifier when the phase is `committed`.

The file lives under a path resolved through `git rev-parse --git-path gutil/conflict-state.json`. This keeps it outside tracked project content and gives linked worktrees their correct Git-managed location. State writes use a temporary file and atomic rename to avoid partial JSON.

Preparation writes state only after Git reports conflicts. A clean merge does not create continuation state. A new conflict preparation refuses to overwrite existing valid state and explains how to continue or abort it.

## Continue Workflow

For state in the `resolving` phase, `gutil conflict --continue` performs these steps in order:

1. Verify that Git is available and the current directory is a Git working tree.
2. Load and validate gUtil conflict state.
3. Verify that a merge is active.
4. Verify that the current branch equals the stored source branch.
5. Verify that the current `HEAD` equals the stored source commit and `MERGE_HEAD` equals the stored target merge commit.
6. Query unmerged files. If any remain, print their names and stop without committing or pushing.
7. Query staged paths. Verify that every stored conflicted path is staged. If any are missing, print their names and instruct the user to stage them in Visual Studio Code.
8. Run:

```text
git commit -m "[gUtil] Conflict Resolution - <N> files fixed."
```

`N` is the number of distinct paths stored when conflicts were first detected. Grammar remains `1 files fixed` only if the product deliberately keeps the exact requested template; version 0.2 instead uses the grammatically correct singular `1 file fixed` and plural `<N> files fixed`.

9. Resolve the new commit identifier and atomically update state to phase `committed`.
10. Run `git push origin <source-branch>`.
11. Delete the state file only after the push succeeds.
12. Print the committed identifier and pushed branch.

## Push Retry

If commit succeeds but push fails, gUtil preserves state in the `committed` phase and reports that no second commit will be created.

The next `gutil conflict --continue` call:

1. Verifies the current branch and stored commit identifier.
2. Verifies that no merge is active, because commit already completed it.
3. Retries only `git push origin <source-branch>`.
4. Deletes state after a successful push.

If `HEAD` no longer matches the stored committed identifier, gUtil stops rather than pushing an ambiguous history.

## Abort and Status

`gutil conflict --abort` retains its requirement for an active merge. After `git merge --abort` succeeds, it deletes matching gUtil conflict state. It does not delete state when Git cannot abort.

`gutil conflict --status` additionally reports whether gUtil owns the active conflict workflow, its source and target branches, its phase, total original conflict count, unresolved count, and conflicted paths not yet staged. Invalid state is reported as an actionable error rather than ignored.

## Safety and Failures

gUtil does not run `git add`, amend commits, force-push, change branches, or delete user files during continuation.

Every failed validation stops before commit. Commit failure leaves the resolving state intact. Push failure leaves committed state intact. State deletion failure after a successful push is reported prominently; a later continuation detects that the stored commit is already reachable from `origin/<source>` and safely removes stale state without pushing a duplicate operation.

## Architecture Changes

Add a focused state component under `internal/commands/conflict` responsible for JSON validation, atomic persistence, and removal. Extend the typed Git client with current branch, current commit, merge-head commit, staged paths, commit-with-message, remote containment, and push-origin operations.

The workflow remains responsible for orchestration. The state store does not invoke Git, and the Git client does not understand gUtil state.

## Testing

Unit tests cover:

- Argument dispatch for `--continue`.
- Atomic state read, write, validation, and removal.
- Refusal of manual merges and mismatched branches or commits.
- Listing unresolved conflicts.
- Listing original conflict files that are not staged.
- Singular and plural commit messages.
- Commit failure without push.
- Push failure after commit and push-only retry.
- Stale state cleanup when the remote already contains the committed identifier.
- Abort state cleanup only after successful Git abort.
- Extended status reporting.

Integration tests use temporary repositories and a local bare `origin` to prove the complete prepare, resolve, stage, continue, commit, and push flow. A second integration test installs a rejecting pre-receive hook, verifies preserved committed state after push failure, removes the hook, and verifies push-only retry without another commit.

## Documentation and Versioning

README command reference and recovery instructions include `--continue`. The changelog records the feature under `Unreleased`. The first published release may include it in `v0.1.0`; the installed development build continues to report `v0.1.0-dev` until a release tag is created.

## Acceptance Criteria

1. `--continue` works only for conflict merges initiated by gUtil.
2. It never stages files automatically.
3. It lists and rejects unresolved or unstaged conflict files.
4. It creates exactly one custom conflict-resolution commit.
5. It pushes only the stored source branch to `origin`.
6. A failed push can be retried without creating a second commit.
7. State is removed only after successful completion or successful abort.
8. Unit and real-repository integration tests pass on macOS, Linux, and Windows CI.
