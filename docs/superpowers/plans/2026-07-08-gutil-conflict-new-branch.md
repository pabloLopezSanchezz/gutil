# gUtil Conflict New Branch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--new-branch` conflict preparation that branches from the updated source and ensures continuation pushes only the generated branch.

**Architecture:** Parsing produces typed prepare options. The workflow uses an injected clock and pure naming function, validates collisions through the Git service, creates the branch after source synchronization, and persists original/effective branch identity in versioned state.

**Tech Stack:** Go standard library, existing Git/process abstractions, local bare-repository integration tests.

---

### Task 1: Add branch creation and format validation

**Files:**
- Modify: `internal/git/client.go`
- Modify: `internal/git/client_test.go`

- [ ] **Step 1: Write failing tests** for exact commands `git check-ref-format --branch <name>` and `git checkout -b <name>`, including exit-code-one invalid format and dash-prefixed names.
- [ ] **Step 2: Run** `go test ./internal/git -run 'Test(CreateBranch|ValidateBranch)'`; expect missing methods.
- [ ] **Step 3: Implement** `ValidateBranch(context.Context, string) error` and `CreateBranch(context.Context, string) error`. Validation returns a specific invalid-branch error without executing checkout.
- [ ] **Step 4: Run** `gofmt -w internal/git && go test ./internal/git && go vet ./internal/git`; expect PASS.
- [ ] **Step 5: Commit** with `git commit -m "feat: add safe branch creation operations"`.

### Task 2: Parse typed preparation options and generate names

**Files:**
- Modify: `internal/commands/conflict/command.go`
- Create: `internal/commands/conflict/branch.go`
- Create: `internal/commands/conflict/branch_test.go`
- Modify: `internal/commands/conflict/workflow_test.go`

- [ ] **Step 1: Write failing parser tests** for no flag, canonical flag, alias, and rejection of misplaced, duplicated, combined, or unknown flags.
- [ ] **Step 2: Write failing naming tests** using a fixed clock for `08072026`, zero padding, and target `release/next` producing `feature/conflictResolution/release/next/08072026`.
- [ ] **Step 3: Run** `go test ./internal/commands/conflict -run 'Test(CommandNewBranch|ResolutionBranchName)'`; expect failure.
- [ ] **Step 4: Implement** `PrepareOptions{NewBranch bool}`, command parsing, `Clock func() time.Time`, and pure `resolutionBranchName(target string, now time.Time) string`. Pass options into `Workflow.Prepare`.
- [ ] **Step 5: Run** package tests and commit as `feat: parse conflict resolution branch mode`.

### Task 3: Create and persist the effective resolution branch

**Files:**
- Modify: `internal/commands/conflict/state.go`
- Modify: `internal/commands/conflict/state_test.go`
- Modify: `internal/commands/conflict/workflow.go`
- Modify: `internal/commands/conflict/workflow_test.go`

- [ ] **Step 1: Write failing state tests** for schema version 2 fields `originalSourceBranch` and `generatedBranch`, including normal mode identity and generated mode identity.
- [ ] **Step 2: Write failing workflow tests** proving the branch is named after synchronization, checked for local/remote collision, format-validated, created at the updated source commit, and used as effective source. Cover both collision locations and unchanged normal mode.
- [ ] **Step 3: Run** focused tests; expect failure from absent fields and operations.
- [ ] **Step 4: Implement** version-2 state validation, new GitService methods, clock default `time.Now`, collision checks, branch creation, post-creation identity checks, and state persistence with original/effective branches.
- [ ] **Step 5: Verify** `go test ./... && go vet ./...`; expect PASS.
- [ ] **Step 6: Commit** as `feat: prepare conflicts on generated branches`.

### Task 4: Prove protected-source behavior with a real remote

**Files:**
- Modify: `internal/commands/conflict/integration_test.go`

- [ ] **Step 1: Add a real integration test** with fixed local date, source and target commits, conflict preparation using `--new-branch`, staged resolution, and `--continue`.
- [ ] **Step 2: Assert** the generated branch starts from the updated source, original local and remote source refs remain at their prior commit, generated remote ref equals the resolution commit, and current branch is generated.
- [ ] **Step 3: Invoke the same command again** after returning to the source and assert collision before any ref changes.
- [ ] **Step 4: Run** `go test ./internal/commands/conflict -run Integration -count=3 -v` and `go test -race ./...`; expect PASS.
- [ ] **Step 5: Commit** as `test: verify protected source branch workflow`.

### Task 5: Document, verify, merge, and install

**Files:**
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `internal/cli/docs_test.go`

- [ ] **Step 1: Extend the README contract test** to require `--new-branch`, alias, generated template, collision behavior, protected source, and abort preservation; run it and observe failure.
- [ ] **Step 2: Update English README and changelog** with exact examples and behavior.
- [ ] **Step 3: Run** `test -z "$(gofmt -l .)"`, `go vet ./...`, `go test -race ./...`, `go test ./... -count=3`, and build `work/gutil`; expect all exit `0`.
- [ ] **Step 4: Merge the isolated branch into `main`, rerun `go test ./...`, install with `install -m 0755 work/gutil "$HOME/.local/bin/gutil"`, and verify help lists `--new-branch`.
- [ ] **Step 5: Keep GitHub publication paused until `gh auth status` confirms `pabloLopezSanchezz`; do not create or push a remote without that prerequisite.

