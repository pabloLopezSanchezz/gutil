package conflict

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gitpkg "github.com/pablo/gutil/internal/git"
	"github.com/pablo/gutil/internal/output"
)

type GitService interface {
	ValidateRepository(context.Context) error
	WorkingTreeClean(context.Context) (bool, string, error)
	FetchOrigin(context.Context) error
	BranchLocation(context.Context, string) (gitpkg.BranchLocation, error)
	CreateTrackingBranch(context.Context, string) error
	Checkout(context.Context, string) error
	PullOrigin(context.Context, string) error
	MergeNoCommit(context.Context, string) error
	ConflictFiles(context.Context) ([]string, error)
	Status(context.Context) (string, error)
	AbortMerge(context.Context) error
	OperationState(context.Context) (gitpkg.OperationState, error)
	CurrentBranch(context.Context) (string, error)
	CurrentCommit(context.Context) (string, error)
	MergeHead(context.Context) (string, error)
	StagedFiles(context.Context) ([]string, error)
	Commit(context.Context, string) error
	PushOrigin(context.Context, string) error
	RemoteContains(context.Context, string, string) (bool, error)
	GitPath(context.Context, string) (string, error)
}

type Editor interface {
	Open(context.Context, string) error
}

type Workflow struct {
	Git    GitService
	Editor Editor
	Output output.Printer
	Store  *StateStore
}

func (w Workflow) Prepare(ctx context.Context, source, target string) error {
	if err := w.preflight(ctx); err != nil {
		return err
	}
	store, err := w.stateStore(ctx)
	if err != nil {
		return err
	}
	exists, err := store.Exists()
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("a gUtil conflict workflow already exists; continue or abort it before starting another")
	}
	if err := w.Git.FetchOrigin(ctx); err != nil {
		return err
	}
	if err := w.syncBranch(ctx, target); err != nil {
		return err
	}
	if err := w.syncBranch(ctx, source); err != nil {
		return err
	}
	branch, err := w.Git.CurrentBranch(ctx)
	if err != nil {
		return err
	}
	if branch != source {
		return fmt.Errorf("current branch %q does not match source branch %q", branch, source)
	}
	sourceCommit, err := w.Git.CurrentCommit(ctx)
	if err != nil {
		return err
	}

	mergeErr := w.Git.MergeNoCommit(ctx, target)
	files, conflictErr := w.Git.ConflictFiles(ctx)
	if conflictErr != nil {
		return conflictErr
	}
	if len(files) > 0 {
		mergeCommit, err := w.Git.MergeHead(ctx)
		if err != nil {
			return err
		}
		if err := store.Save(ConflictState{Version: stateVersion, SourceBranch: source, TargetBranch: target, SourceCommit: sourceCommit, MergeCommit: mergeCommit, ConflictFiles: files, Phase: PhaseResolving}); err != nil {
			return err
		}
		w.Output.Warning(fmt.Sprintf("%d conflicting file(s) found:", len(files)))
		for _, file := range files {
			w.Output.Info("  " + file)
		}
		if w.Editor != nil {
			if err := w.Editor.Open(ctx, "."); err != nil {
				w.Output.Warning("Conflicts are ready, but Visual Studio Code could not be opened automatically.\nOpen this repository in Visual Studio Code and resolve the listed files.")
			}
		}
		return nil
	}
	if mergeErr != nil {
		return mergeErr
	}
	w.Output.Success("Merge prepared without conflicts. Review the changes and commit when ready.")
	return nil
}

func (w Workflow) stateStore(ctx context.Context) (StateStore, error) {
	if w.Store != nil {
		return *w.Store, nil
	}
	path, err := w.Git.GitPath(ctx, "gutil/conflict-state.json")
	if err != nil {
		return StateStore{}, err
	}
	return StateStore{Path: path}, nil
}

func (w Workflow) Continue(ctx context.Context) error {
	if err := w.Git.ValidateRepository(ctx); err != nil {
		return err
	}
	store, err := w.stateStore(ctx)
	if err != nil {
		return err
	}
	state, err := store.Load()
	if errors.Is(err, ErrStateNotFound) {
		return fmt.Errorf("no gUtil conflict workflow is available to continue")
	}
	if err != nil {
		return err
	}
	if state.Phase == PhaseCommitted {
		return w.continueCommitted(ctx, store, state)
	}
	return w.continueResolving(ctx, store, state)
}

func (w Workflow) continueResolving(ctx context.Context, store StateStore, state ConflictState) error {
	operation, err := w.Git.OperationState(ctx)
	if err != nil {
		return err
	}
	if operation != gitpkg.MergeOperation {
		return fmt.Errorf("the gUtil state exists, but its merge is not active")
	}
	if err := w.validateIdentity(ctx, state, true); err != nil {
		return err
	}
	unresolved, err := w.Git.ConflictFiles(ctx)
	if err != nil {
		return err
	}
	if len(unresolved) > 0 {
		return fmt.Errorf("resolve and stage these remaining conflicts in Visual Studio Code:\n%s", formatPaths(unresolved))
	}
	staged, err := w.Git.StagedFiles(ctx)
	if err != nil {
		return err
	}
	missing := missingPaths(state.ConflictFiles, staged)
	if len(missing) > 0 {
		return fmt.Errorf("these resolved conflict files are not staged:\n%s\nStage them in Visual Studio Code and run --continue again", formatPaths(missing))
	}
	message := conflictCommitMessage(len(state.ConflictFiles))
	if err := w.Git.Commit(ctx, message); err != nil {
		return err
	}
	commit, err := w.Git.CurrentCommit(ctx)
	if err != nil {
		return err
	}
	state.Phase, state.Commit = PhaseCommitted, commit
	if err := store.Save(state); err != nil {
		return err
	}
	if err := w.Git.PushOrigin(ctx, state.SourceBranch); err != nil {
		return fmt.Errorf("commit %s was created, but push failed: %w; run --continue again to retry only the push", commit, err)
	}
	if err := store.Remove(); err != nil {
		return err
	}
	w.Output.Success(fmt.Sprintf("Created commit %s and pushed %s to origin.", commit, state.SourceBranch))
	return nil
}

func (w Workflow) continueCommitted(ctx context.Context, store StateStore, state ConflictState) error {
	operation, err := w.Git.OperationState(ctx)
	if err != nil {
		return err
	}
	if operation != gitpkg.NoOperation {
		return fmt.Errorf("cannot retry push while a Git %s operation is active", operation)
	}
	if err := w.validateIdentity(ctx, state, false); err != nil {
		return err
	}
	contained, err := w.Git.RemoteContains(ctx, state.Commit, state.SourceBranch)
	if err != nil {
		return err
	}
	if !contained {
		if err := w.Git.PushOrigin(ctx, state.SourceBranch); err != nil {
			return fmt.Errorf("push retry failed: %w", err)
		}
	}
	if err := store.Remove(); err != nil {
		return err
	}
	w.Output.Success(fmt.Sprintf("Commit %s is available on origin/%s.", state.Commit, state.SourceBranch))
	return nil
}

func (w Workflow) validateIdentity(ctx context.Context, state ConflictState, resolving bool) error {
	branch, err := w.Git.CurrentBranch(ctx)
	if err != nil {
		return err
	}
	if branch != state.SourceBranch {
		return fmt.Errorf("current branch %q does not match gUtil source branch %q", branch, state.SourceBranch)
	}
	commit, err := w.Git.CurrentCommit(ctx)
	if err != nil {
		return err
	}
	want := state.Commit
	if resolving {
		want = state.SourceCommit
	}
	if commit != want {
		return fmt.Errorf("current commit %q does not match gUtil state commit %q", commit, want)
	}
	if resolving {
		mergeCommit, err := w.Git.MergeHead(ctx)
		if err != nil {
			return err
		}
		if mergeCommit != state.MergeCommit {
			return fmt.Errorf("active merge commit %q does not match gUtil state %q", mergeCommit, state.MergeCommit)
		}
	}
	return nil
}

func conflictCommitMessage(count int) string {
	if count == 1 {
		return "[gUtil] Conflict Resolution - 1 file fixed."
	}
	return fmt.Sprintf("[gUtil] Conflict Resolution - %d files fixed.", count)
}

func missingPaths(required, actual []string) []string {
	set := make(map[string]struct{}, len(actual))
	for _, path := range actual {
		set[path] = struct{}{}
	}
	var missing []string
	for _, path := range required {
		if _, ok := set[path]; !ok {
			missing = append(missing, path)
		}
	}
	return missing
}

func formatPaths(paths []string) string {
	var lines []string
	for _, path := range paths {
		lines = append(lines, "  "+path)
	}
	return strings.Join(lines, "\n")
}

func (w Workflow) preflight(ctx context.Context) error {
	if err := w.Git.ValidateRepository(ctx); err != nil {
		return err
	}
	state, err := w.Git.OperationState(ctx)
	if err != nil {
		return err
	}
	if state != gitpkg.NoOperation {
		return fmt.Errorf("a Git %s operation is already in progress", state)
	}
	clean, details, err := w.Git.WorkingTreeClean(ctx)
	if err != nil {
		return err
	}
	if !clean {
		return fmt.Errorf("the working tree must be completely clean, including untracked files:\n%s", strings.TrimSpace(details))
	}
	return nil
}

func (w Workflow) syncBranch(ctx context.Context, branch string) error {
	location, err := w.Git.BranchLocation(ctx, branch)
	if err != nil {
		return err
	}
	switch location {
	case gitpkg.Local:
		if err := w.Git.Checkout(ctx, branch); err != nil {
			return err
		}
	case gitpkg.RemoteOnly:
		if err := w.Git.CreateTrackingBranch(ctx, branch); err != nil {
			return err
		}
	default:
		return fmt.Errorf("branch %q does not exist locally or in origin", branch)
	}
	return w.Git.PullOrigin(ctx, branch)
}

func (w Workflow) Status(ctx context.Context) error {
	if err := w.Git.ValidateRepository(ctx); err != nil {
		return err
	}
	status, err := w.Git.Status(ctx)
	if err != nil {
		return err
	}
	w.Output.Info(strings.TrimRight(status, "\r\n"))
	files, err := w.Git.ConflictFiles(ctx)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		w.Output.Info("No unmerged files found.")
	} else {
		w.Output.Info("Unmerged files:")
		for _, file := range files {
			w.Output.Info("  " + file)
		}
	}
	store, storeErr := w.stateStore(ctx)
	if storeErr != nil {
		return storeErr
	}
	state, stateErr := store.Load()
	if stateErr == nil {
		w.Output.Info(fmt.Sprintf("gUtil workflow: %s -> %s (%s, %d original conflicts)", state.SourceBranch, state.TargetBranch, state.Phase, len(state.ConflictFiles)))
	} else if !errors.Is(stateErr, ErrStateNotFound) {
		return stateErr
	}
	return nil
}

func (w Workflow) Abort(ctx context.Context) error {
	if err := w.Git.ValidateRepository(ctx); err != nil {
		return err
	}
	state, err := w.Git.OperationState(ctx)
	if err != nil {
		return err
	}
	if state != gitpkg.MergeOperation {
		return fmt.Errorf("no merge is currently in progress")
	}
	if err := w.Git.AbortMerge(ctx); err != nil {
		return err
	}
	store, err := w.stateStore(ctx)
	if err != nil {
		return err
	}
	if err := store.Remove(); err != nil {
		return err
	}
	w.Output.Success("Merge aborted successfully.")
	return nil
}
