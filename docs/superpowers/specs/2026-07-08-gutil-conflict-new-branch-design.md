# gUtil Conflict New Branch Design Specification

## Purpose

Add an optional conflict-resolution branch mode for users who cannot push to a protected source branch.

## Public Interface

The documented form is:

```text
gutil conflict <source> <target> --new-branch
```

`--newBranch` is accepted as a compatibility alias. The flag must appear after the two branch arguments. Supplying either spelling more than once, combining both spellings, or adding any other argument is invalid usage and returns exit code `2`.

## Generated Branch

The generated branch name is:

```text
feature/conflictResolution/<target>/DDMMYYYY
```

The date uses the computer's local timezone at command execution time, with a zero-padded day and month and four-digit year. The target branch name is preserved exactly, including embedded `/` separators. Because the target has already passed Git branch validation, the generated name must also pass `git check-ref-format --branch` before any branch creation.

Example on 8 July 2026:

```text
feature/conflictResolution/develop/08072026
```

## Workflow

The existing preflight rules remain unchanged. For `gutil conflict feature/ABC develop --new-branch`, gUtil performs:

1. Validate arguments, repository, active operations, and clean working tree.
2. Run `git fetch origin --prune`.
3. Resolve, check out, and run `git pull origin develop`.
4. Resolve, check out, and run `git pull origin feature/ABC`.
5. Record the updated source commit.
6. Generate `feature/conflictResolution/develop/DDMMYYYY` using the local date.
7. Check both `refs/heads/<generated>` and `refs/remotes/origin/<generated>`.
8. If either exists, stop with an actionable collision error. gUtil never reuses, resets, deletes, or force-updates the branch.
9. Create and check out the generated local branch from the current updated source commit using `git checkout -b <generated>`.
10. Confirm the current branch and commit before starting the merge.
11. Run `git merge --no-commit --no-ff develop`.
12. Continue with the existing clean-merge or conflict behavior.

Without `--new-branch`, current behavior remains unchanged.

## Continuation State

The conflict state schema gains:

- `originalSourceBranch`: the protected or original source branch requested by the user.
- `sourceBranch`: the effective branch on which resolution occurs and which continuation pushes.
- `generatedBranch`: true when gUtil created the effective branch for this workflow.

For the normal mode, `originalSourceBranch` and `sourceBranch` are identical and `generatedBranch` is false. For new-branch mode, `sourceBranch` is the generated branch and `originalSourceBranch` remains the requested source.

The schema version increments. State reading does not silently reinterpret unsupported older state; it reports an actionable version error. No released version exists yet, so an automatic state migration is unnecessary.

`gutil conflict --continue` validates and commits on the effective `sourceBranch`, then executes:

```text
git push origin <effective-source-branch>
```

It never pushes to `originalSourceBranch` in new-branch mode. Push retry uses the same effective branch stored in state.

## Abort and Failure Behavior

`gutil conflict --abort` aborts the merge and removes matching gUtil state after Git succeeds. It does not delete the generated branch or change back to the original branch.

If any operation fails after the generated branch is created, gUtil preserves the branch and repository state. The error identifies the current branch and provides recovery instructions. Automatic branch deletion is outside scope because it could discard manual work created after branch creation.

If the merge completes without conflicts, gUtil leaves the uncommitted merge on the generated branch, matching normal conflict-command behavior. No continuation state is created because `--continue` remains specifically a conflict-resolution workflow.

## Architecture

Argument parsing produces a preparation options value rather than passing a raw boolean through the CLI. The workflow receives an injected clock so date-dependent behavior is deterministic in tests. Branch naming is a pure function that formats the local date and validates the result through the Git client.

The Git client adds a create-branch operation equivalent to `git checkout -b <branch>` and a branch-format validation operation based on `git check-ref-format --branch <branch>`. Existing branch-location logic handles local and remote collision checks.

## Testing

Unit tests cover:

- Documented and alias flag parsing.
- Rejection of duplicate, combined, misplaced, and unknown flags.
- Date formatting with zero padding and local timezone.
- Target names containing `/`.
- Exact workflow order and branch creation from the updated source commit.
- Local and remote generated-branch collisions.
- Generated-name validation before creation.
- State fields for normal and generated modes.
- Continuation commit and push exclusively on the effective generated branch.
- Abort preserving the generated branch.

Integration tests use a temporary bare `origin` to verify that:

1. The generated branch starts at the updated source commit.
2. A real conflict is prepared on the generated branch.
3. The original source reference remains unchanged.
4. `--continue` pushes the generated branch and does not push the original source.
5. Repeating the same new-branch command on the same date reports a collision without modifying either branch.

## Documentation

README includes the new command, generated-name example, protected-branch use case, collision behavior, continuation flow, and the fact that abort preserves the generated branch. The changelog records the feature under `Unreleased`.

## Acceptance Criteria

1. Both flag spellings select new-branch mode; documentation uses `--new-branch`.
2. The generated branch is based on the fully updated source branch.
3. The exact naming template and local date are used.
4. Any local or remote collision stops before branch creation.
5. Existing behavior without the flag does not change.
6. Conflict continuation commits and pushes only the generated branch.
7. The protected original source branch remains unchanged locally and remotely after branch creation.
8. Abort and errors never delete the generated branch automatically.
9. Unit, race, and real-repository integration tests pass.

