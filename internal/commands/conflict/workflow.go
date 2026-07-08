package conflict

import (
	"context"
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
}

type Editor interface {
	Open(context.Context, string) error
}

type Workflow struct {
	Git    GitService
	Editor Editor
	Output output.Printer
}

func (w Workflow) Prepare(ctx context.Context, source, target string) error {
	if err := w.preflight(ctx); err != nil {
		return err
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

	mergeErr := w.Git.MergeNoCommit(ctx, target)
	files, conflictErr := w.Git.ConflictFiles(ctx)
	if conflictErr != nil {
		return conflictErr
	}
	if len(files) > 0 {
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
		return nil
	}
	w.Output.Info("Unmerged files:")
	for _, file := range files {
		w.Output.Info("  " + file)
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
	w.Output.Success("Merge aborted successfully.")
	return nil
}
