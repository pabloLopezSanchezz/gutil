# gUtil

gUtil is a cross-platform command-line tool that automates a safe Git conflict-resolution workflow.

It is designed for teams that need to repeatedly merge a target branch into a source branch, resolve conflicts manually in Visual Studio Code, and then commit and push the result in a consistent way.

Current focus: `gutil conflict`.

## What gUtil does

`gutil conflict <source> <target>` prepares this operation:

```text
merge <target> into <source>
```

Example:

```sh
gutil conflict LQAW2 DevelopB2BEUW2
```

This means:

```text
merge DevelopB2BEUW2 into LQAW2
```

gUtil always uses the Git remote named `origin`.

## Requirements

- Git installed and available on `PATH`.
- A Git repository with remote `origin`.
- Visual Studio Code is recommended.
- The `code` launcher is optional, but useful because gUtil can open the repository automatically when conflicts exist.
- End users do not need Go when installing from a release.

Before starting a conflict workflow, gUtil requires:

- a clean working tree;
- no untracked files;
- no active merge, rebase, cherry-pick, or revert;
- both branches must exist locally or in `origin`.

## Installation

### macOS and Linux

```sh
curl -fsSL https://raw.githubusercontent.com/pabloLopezSanchezz/gutil/main/scripts/install.sh | sh
```

The installer downloads the latest release, verifies the checksum, and installs `gutil` into:

```text
~/.local/bin/gutil
```

Make sure `~/.local/bin` is in your `PATH`.

If needed, add it to your shell config:

```sh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
```

Then open a new terminal.

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/pabloLopezSanchezz/gutil/main/scripts/install.ps1 | iex
```

The installer downloads the latest release, verifies the checksum, and installs `gutil.exe` into:

```text
$HOME\.local\bin\gutil.exe
```

It also adds that directory to the user `PATH` when required.

Open a new PowerShell terminal after installation.

### Install a specific version

macOS/Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/pabloLopezSanchezz/gutil/main/scripts/install.sh | env GUTIL_VERSION=v0.1.0 sh
```

Windows PowerShell:

```powershell
$env:GUTIL_VERSION = "v0.1.0"
irm https://raw.githubusercontent.com/pabloLopezSanchezz/gutil/main/scripts/install.ps1 | iex
```

### Custom install directory

macOS/Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/pabloLopezSanchezz/gutil/main/scripts/install.sh | env GUTIL_INSTALL_DIR="$HOME/bin" sh
```

Windows PowerShell:

```powershell
$env:GUTIL_INSTALL_DIR = "$HOME\bin"
irm https://raw.githubusercontent.com/pabloLopezSanchezz/gutil/main/scripts/install.ps1 | iex
```

### Build from source

Use this only if you are developing gUtil or you already have Go installed:

```sh
go install github.com/pabloLopezSanchezz/gutil/cmd/gutil@latest
```

## Verify installation

```sh
gutil version
```

Expected output:

```text
gutil <version>
```

You can also print the command list:

```sh
gutil help
```

## Commands

```text
gutil conflict <source> <target>
gutil conflict <source> <target> --new-branch
gutil conflict <source> <target> --newBranch
gutil conflict --status
gutil conflict --continue
gutil conflict --abort
gutil version
gutil help
```

## Parameters and flags

### `<source>`

The branch that receives the merge.

gUtil checks out this branch and prepares the merge on top of it.

Example:

```sh
gutil conflict feature/my-work develop
```

This merges `develop` into `feature/my-work`.

Another example:

```sh
gutil conflict feature/ABC develop
```

This merges `develop into feature/ABC`.

### `<target>`

The branch that is merged into `<source>`.

Example:

```sh
gutil conflict LQAW2 DevelopB2BEUW2
```

This merges `DevelopB2BEUW2` into `LQAW2`.

### `--new-branch`

Creates a new conflict-resolution branch from the updated source branch before preparing the merge.

Use this when you cannot push directly to the protected source branch.

Example:

```sh
gutil conflict LQAW2 DevelopB2BEUW2 --new-branch
```

gUtil creates a branch using this format:

```text
feature/conflictResolution/<target>/DDMMYYYY
```

Example:

```text
feature/conflictResolution/DevelopB2BEUW2/14072026
```

After `gutil conflict --continue`, gUtil commits and pushes only the generated branch.

The original source branch is not pushed.

If the generated branch already exists locally or in `origin`, gUtil stops without overwriting it.

### `--newBranch`

Alias for `--new-branch`.

It is supported for convenience, but `--new-branch` is the recommended documented form.

### `--status`

Shows the current Git status and the current gUtil conflict workflow, if one exists.

```sh
gutil conflict --status
```

Use it when:

- you want to confirm whether conflicts are still unresolved;
- you want to confirm the workflow source and target;
- `--continue` reports an error;
- you are not sure what state the repository is in.

### `--continue`

Finishes a gUtil conflict workflow after you have resolved and staged the conflicts.

```sh
gutil conflict --continue
```

gUtil checks that:

- the workflow was started by gUtil;
- the current branch matches the workflow state;
- the merge still matches the workflow state;
- Git has no unresolved conflict entries.

Then it creates a commit with this message format:

```text
[gUtil] Conflict Resolution - <N> files fixed.
```

Example:

```text
[gUtil] Conflict Resolution - 3 files fixed.
```

Finally, it pushes to:

```text
origin/<source>
```

Internally, this is a normal `git push origin <source>` operation.

If `--new-branch` was used, it pushes to:

```text
origin/feature/conflictResolution/<target>/DDMMYYYY
```

Important: gUtil does not run `git add`. You must stage the resolved files yourself, normally from Visual Studio Code.

In other words, the resolved files must be already staged before running `gutil conflict --continue`.

### `--abort`

Aborts the active Git merge and removes the gUtil workflow state.

```sh
gutil conflict --abort
```

If `--new-branch` created a branch, `--abort` does not delete that branch.

More explicitly: `gutil conflict --abort` does not delete the generated branch.

Delete it manually if you no longer need it.

## Standard workflow

### 1. Start the conflict workflow

```sh
gutil conflict <source> <target>
```

Example:

```sh
gutil conflict LQAW2 DevelopB2BEUW2
```

Internally, gUtil does this:

1. validates that the repository is safe to operate on;
2. runs `git fetch origin --prune`;
3. checks out `<target>`;
4. runs `git pull origin <target>`;
5. checks out `<source>`;
6. runs `git pull origin <source>`;
7. runs `git merge --no-commit --no-ff <target>`;
8. opens Visual Studio Code if conflicts exist.

`git fetch origin --prune` updates local knowledge of `origin` and removes stale remote-tracking references for branches that no longer exist in `origin`.

### 2. Resolve conflicts in Visual Studio Code

Open each conflicted file and choose the correct final content.

Then stage the resolved files.

You can stage them in Visual Studio Code from the Source Control panel.

### 3. Check status

```sh
gutil conflict --status
```

You should see no unmerged files before continuing.

### 4. Continue

```sh
gutil conflict --continue
```

gUtil commits and pushes the conflict-resolution result.

## Protected branch workflow

Use this when `<source>` is a protected source branch and you cannot push directly to it.

```sh
gutil conflict <source> <target> --new-branch
```

Example:

```sh
gutil conflict LQAW2 DevelopB2BEUW2 --new-branch
```

gUtil will:

1. update `<target>` from `origin`;
2. update `<source>` from `origin`;
3. create a new branch from `<source>`;
4. prepare the merge of `<target>` into the generated branch;
5. open Visual Studio Code if conflicts exist.

After resolving and staging conflicts:

```sh
gutil conflict --continue
```

Then open a pull request from the generated branch into the protected source branch.

## Clean merge behavior

If the merge has no conflicts, gUtil leaves the merge prepared but uncommitted.

This is intentional.

Review the result and commit manually if it is correct.

## Error handling and recovery

### Inspect current state

```sh
gutil conflict --status
```

### Abort the merge

```sh
gutil conflict --abort
```

### Push failed after commit

If the commit was created but the push failed, fix the push problem and run:

```sh
gutil conflict --continue
```

gUtil will retry only the push.

It will not create a second commit.

### Remaining conflicts

If `--continue` reports unresolved files:

1. open the reported files;
2. remove all conflict markers;
3. stage the resolved files;
4. run:

```sh
gutil conflict --status
gutil conflict --continue
```

### Resolved files not shown in staged diff

Git can omit a resolved file from the staged diff if the final content is identical to the source branch version.

That is valid.

gUtil checks Git's unresolved conflict entries instead of requiring every originally conflicted file to appear in `git diff --cached`.

## Safety guarantees

gUtil does not:

- create commits before you resolve conflicts;
- run `git add`;
- run `git reset --hard`;
- run `git clean`;
- run `git stash`;
- force push;
- delete branches;
- push to any remote other than `origin`;
- continue workflows that were not started by gUtil.

## Exit codes

- `0`: command completed successfully.
- `1`: repository, Git, or environment error.
- `2`: invalid command usage.

## Development

Clone the repository:

```sh
git clone git@github.com:pabloLopezSanchezz/gutil.git
cd gutil
```

Run tests:

```sh
go test ./...
```

Build locally:

```sh
go build -o gutil ./cmd/gutil
```

Run locally:

```sh
./gutil version
```

## Release notes

Releases follow semantic versioning.

See [CHANGELOG.md](CHANGELOG.md).
