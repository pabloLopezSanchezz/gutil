# gUtil Conflict Continue Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a safe `gutil conflict --continue` command that commits staged conflict resolutions with a custom message and pushes the source branch without ever staging files automatically.

**Architecture:** A JSON state store under Git's private path records the exact gUtil-owned merge. The conflict workflow validates state and Git identity before commit, persists a committed phase before push, and supports push-only retry after failure.

**Tech Stack:** Go standard library, Git CLI, existing process/Git abstractions, temporary-repository integration tests.

---

## File map

```text
internal/commands/conflict/state.go             versioned JSON state and atomic persistence
internal/commands/conflict/state_test.go        store validation and lifecycle tests
internal/commands/conflict/command.go           --continue dispatch and help
internal/commands/conflict/workflow.go          prepare persistence, continue, retry, abort/status integration
internal/commands/conflict/workflow_test.go     isolated orchestration tests
internal/commands/conflict/integration_test.go  real prepare-resolve-continue-push tests
internal/git/client.go                          identity, staged, commit, containment, and push operations
internal/git/client_test.go                     exact Git invocation/result tests
README.md                                       continuation usage and recovery
CHANGELOG.md                                    feature entry
```

### Task 1: Add Git operations required by continuation

**Files:**
- Modify: `internal/git/client.go`
- Modify: `internal/git/client_test.go`

- [ ] **Step 1: Write failing exact-command tests**

Add recording-runner tests for:

```text
symbolic-ref --quiet --short HEAD
rev-parse HEAD
rev-parse MERGE_HEAD
diff --cached --name-only --diff-filter=ACMR
commit -m <message>
push origin <branch>
merge-base --is-ancestor <commit> origin/<branch>
rev-parse --git-path gutil/conflict-state.json
```

Verify CRLF-safe path parsing and that `merge-base` exit code `1` means not contained rather than operational failure.

- [ ] **Step 2: Run the tests and observe RED**

Run `go test ./internal/git -run 'Test(Current|Staged|Commit|Push|Remote|GitPath)'`.

Expected: compilation fails because the new methods do not exist.

- [ ] **Step 3: Implement the typed methods**

Add:

```go
CurrentBranch(context.Context) (string, error)
CurrentCommit(context.Context) (string, error)
MergeHead(context.Context) (string, error)
StagedFiles(context.Context) ([]string, error)
Commit(context.Context, string) error
PushOrigin(context.Context, string) error
RemoteContains(context.Context, string, string) (bool, error)
GitPath(context.Context, string) (string, error)
```

Reuse branch validation for push. Reject an empty commit message. Normalize relative Git paths against the client's working directory.

- [ ] **Step 4: Verify GREEN and commit**

```bash
gofmt -w internal/git
go test ./internal/git
go vet ./internal/git
git add internal/git
git commit -m "feat: add git continuation operations"
```

Expected: PASS with no vet findings.

### Task 2: Implement atomic gUtil conflict state

**Files:**
- Create: `internal/commands/conflict/state.go`
- Create: `internal/commands/conflict/state_test.go`

- [ ] **Step 1: Write failing store tests**

Test round-trip persistence of:

```go
type ConflictState struct {
    Version      int      `json:"version"`
    SourceBranch string   `json:"sourceBranch"`
    TargetBranch string   `json:"targetBranch"`
    SourceCommit string   `json:"sourceCommit"`
    MergeCommit  string   `json:"mergeCommit"`
    ConflictFiles []string `json:"conflictFiles"`
    Phase        string   `json:"phase"`
    Commit       string   `json:"commit,omitempty"`
}
```

Cover sorted/deduplicated files, missing file, unsupported version, invalid phase, empty required fields, committed phase without commit, atomic replacement, and idempotent removal.

- [ ] **Step 2: Run the tests and observe RED**

Run `go test ./internal/commands/conflict -run 'TestState'`.

Expected: compilation fails because `StateStore` is undefined.

- [ ] **Step 3: Implement the store**

Define `StateStore{Path string}` with `Load`, `Save`, `Exists`, and `Remove`. `Save` validates and canonicalizes state, creates the parent with mode `0700`, writes a sibling temporary file with mode `0600`, calls `Sync`, closes it, and atomically renames it. `Load` distinguishes `ErrStateNotFound` and `ErrInvalidState`.

- [ ] **Step 4: Verify GREEN and commit**

```bash
gofmt -w internal/commands/conflict/state.go internal/commands/conflict/state_test.go
go test ./internal/commands/conflict -run 'TestState'
git add internal/commands/conflict/state.go internal/commands/conflict/state_test.go
git commit -m "feat: persist gutil conflict state"
```

Expected: PASS.

### Task 3: Implement prepare-state, continue, retry, status, and abort behavior

**Files:**
- Modify: `internal/commands/conflict/command.go`
- Modify: `internal/commands/conflict/workflow.go`
- Modify: `internal/commands/conflict/workflow_test.go`
- Modify: `cmd/gutil/main.go`

- [ ] **Step 1: Write failing command and workflow tests**

Add tests proving:

- `--continue` dispatches with exit code `0` on success and rejects extra arguments.
- Prepare refuses existing state and records source/target commits plus sorted conflict files.
- Continue rejects missing state, manual merge, wrong branch, changed `HEAD`, changed `MERGE_HEAD`, unresolved files, and original conflict files not present in staged paths.
- Continue never calls a staging operation.
- One conflict uses `[gUtil] Conflict Resolution - 1 file fixed.`; multiple use `[gUtil] Conflict Resolution - N files fixed.`.
- Commit failure does not push and preserves resolving state.
- Push failure preserves committed state.
- Committed state retries only push.
- Remote containment after successful prior push removes stale state without another push.
- Abort removes state only after successful abort.
- Status displays phase and counts.

- [ ] **Step 2: Run tests and observe RED**

Run `go test ./internal/commands/conflict -run 'Test(CommandContinue|PrepareState|Continue|AbortState|StatusState)'`.

Expected: FAIL because dispatch and workflow support are absent.

- [ ] **Step 3: Extend interfaces and implement orchestration**

Add the Task 1 methods to `GitService`. Resolve the state path through `GitPath` for every workflow invocation, instantiate the store through an injectable factory, and implement `Continue` as two explicit phase handlers: `continueResolving` and `continueCommitted`.

Persist `Phase: committed` and the new `HEAD` immediately after successful commit and before push. Remove state only after successful push, successful abort, or confirmation that `origin/<source>` already contains the stored committed identifier.

- [ ] **Step 4: Wire and document usage output**

Add `gutil conflict --continue` to conflict usage. The real composition continues to inject the working directory through the Git client; the store path is obtained from Git rather than hard-coded.

- [ ] **Step 5: Verify GREEN and commit**

```bash
gofmt -w cmd/gutil internal/commands/conflict
go test ./internal/commands/conflict
go test ./...
go vet ./...
git add cmd/gutil internal/commands/conflict
git commit -m "feat: continue and push resolved conflicts"
```

Expected: PASS.

### Task 4: Add real repository continuation and retry tests

**Files:**
- Modify: `internal/commands/conflict/integration_test.go`

- [ ] **Step 1: Add the successful end-to-end test**

Extend the local bare-remote harness: prepare a real conflict, resolve the file, run `git add shared.txt` from the test harness, invoke `--continue`, and assert the merge is no longer active, the custom message is at `HEAD`, `origin/feature/a` equals local `HEAD`, and state is absent.

- [ ] **Step 2: Add unresolved and unstaged rejection tests**

Assert unresolved paths are printed before any commit. Resolve without staging and assert the original path is reported as not staged.

- [ ] **Step 3: Add push-only retry test**

Install a rejecting `pre-receive` hook in the bare remote, resolve/stage/continue, and assert one new local commit plus committed state. Remove the hook, rerun `--continue`, and assert the same commit is pushed with no second commit.

- [ ] **Step 4: Verify repeatedly and commit**

```bash
go test ./internal/commands/conflict -run Integration -count=3 -v
go test -race ./...
git add internal/commands/conflict/integration_test.go
git commit -m "test: verify conflict continuation and push retry"
```

Expected: all repetitions PASS.

### Task 5: Document, build, install, and verify

**Files:**
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `internal/cli/docs_test.go`

- [ ] **Step 1: Extend the failing documentation contract**

Require README content for `gutil conflict --continue`, staged-file ownership, custom message, automatic push to the source branch, and push retry. Run `go test ./internal/cli -run TestReadmeDocumentsPublicContract`; expect failure before editing README.

- [ ] **Step 2: Update English documentation**

Add the continuation command, VS Code staging requirement, validation errors, message format, push behavior, retry semantics, and abort cleanup. Add the feature under `Unreleased` in the changelog.

- [ ] **Step 3: Run the complete quality gate**

```bash
test -z "$(gofmt -l .)"
go vet ./...
go test -race ./...
go test ./... -count=3
go build -trimpath -ldflags "-X main.version=v0.1.0-dev" -o work/gutil ./cmd/gutil
```

Expected: every command exits `0`.

- [ ] **Step 4: Install and smoke-test the updated binary**

```bash
install -m 0755 work/gutil "$HOME/.local/bin/gutil"
zsh -lic 'gutil version; gutil conflict --help'
```

Expected: `v0.1.0-dev` and usage containing `--continue`.

- [ ] **Step 5: Commit documentation**

```bash
git add README.md CHANGELOG.md internal/cli/docs_test.go
git commit -m "docs: explain conflict continuation workflow"
```

- [ ] **Step 6: Stop before GitHub publication**

Run `git status --short --branch` and report the clean result. Resume private GitHub repository creation only after `gh auth login` succeeds and `gh auth status` confirms the `pabloLopezSanchezz` account.

