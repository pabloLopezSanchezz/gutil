package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	processpkg "github.com/pablo/gutil/internal/process"
)

var ErrInvalidBranch = errors.New("invalid branch name")

type BranchLocation int

const (
	Missing BranchLocation = iota
	Local
	RemoteOnly
)

type OperationState string

const (
	NoOperation         OperationState = ""
	MergeOperation      OperationState = "merge"
	RebaseOperation     OperationState = "rebase"
	CherryPickOperation OperationState = "cherry-pick"
	RevertOperation     OperationState = "revert"
)

type OpError struct {
	Operation string
	Result    processpkg.Result
	Err       error
}

func (e *OpError) Error() string { return fmt.Sprintf("git %s failed: %v", e.Operation, e.Err) }
func (e *OpError) Unwrap() error { return e.Err }

type Client struct {
	runner processpkg.Runner
	dir    string
}

func NewClient(runner processpkg.Runner, dir string) Client { return Client{runner: runner, dir: dir} }

func (c Client) run(ctx context.Context, operation string, args ...string) (processpkg.Result, error) {
	result, err := c.runner.Run(ctx, processpkg.Command{Name: "git", Args: args, Dir: c.dir})
	if err != nil {
		return result, &OpError{Operation: operation, Result: result, Err: err}
	}
	return result, nil
}

func (c Client) ValidateRepository(ctx context.Context) error {
	result, err := c.run(ctx, "validate repository", "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return err
	}
	if strings.TrimSpace(result.Stdout) != "true" {
		return fmt.Errorf("current directory is not a Git working tree")
	}
	return nil
}

func (c Client) WorkingTreeClean(ctx context.Context) (bool, string, error) {
	result, err := c.run(ctx, "inspect working tree", "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return false, "", err
	}
	return strings.TrimSpace(result.Stdout) == "", result.Stdout, nil
}

func (c Client) FetchOrigin(ctx context.Context) error {
	_, err := c.run(ctx, "fetch origin", "fetch", "origin", "--prune")
	return err
}

func validBranch(branch string) bool { return branch != "" && !strings.HasPrefix(branch, "-") }

func (c Client) referenceExists(ctx context.Context, ref string) (bool, error) {
	_, err := c.run(ctx, "find branch", "show-ref", "--verify", "--quiet", ref)
	if err == nil {
		return true, nil
	}
	var exitErr *processpkg.ExitError
	if errors.As(err, &exitErr) && exitErr.Code == 1 {
		return false, nil
	}
	return false, err
}

func (c Client) BranchLocation(ctx context.Context, branch string) (BranchLocation, error) {
	if !validBranch(branch) {
		return Missing, ErrInvalidBranch
	}
	local, err := c.referenceExists(ctx, "refs/heads/"+branch)
	if err != nil {
		return Missing, err
	}
	if local {
		return Local, nil
	}
	remote, err := c.referenceExists(ctx, "refs/remotes/origin/"+branch)
	if err != nil {
		return Missing, err
	}
	if remote {
		return RemoteOnly, nil
	}
	return Missing, nil
}

func (c Client) CreateTrackingBranch(ctx context.Context, branch string) error {
	if !validBranch(branch) {
		return ErrInvalidBranch
	}
	_, err := c.run(ctx, "create tracking branch", "checkout", "--track", "-b", branch, "origin/"+branch)
	return err
}

func (c Client) ValidateBranch(ctx context.Context, branch string) error {
	if !validBranch(branch) {
		return ErrInvalidBranch
	}
	if _, err := c.run(ctx, "validate branch name", "check-ref-format", "--branch", branch); err != nil {
		return fmt.Errorf("%w: %q: %v", ErrInvalidBranch, branch, err)
	}
	return nil
}

func (c Client) CreateBranch(ctx context.Context, branch string) error {
	if !validBranch(branch) {
		return ErrInvalidBranch
	}
	_, err := c.run(ctx, "create branch", "checkout", "-b", branch)
	return err
}

func (c Client) Checkout(ctx context.Context, branch string) error {
	if !validBranch(branch) {
		return ErrInvalidBranch
	}
	_, err := c.run(ctx, "checkout branch", "checkout", branch)
	return err
}

func (c Client) PullOrigin(ctx context.Context, branch string) error {
	if !validBranch(branch) {
		return ErrInvalidBranch
	}
	_, err := c.run(ctx, "pull branch", "pull", "origin", branch)
	return err
}

func (c Client) MergeNoCommit(ctx context.Context, target string) error {
	if !validBranch(target) {
		return ErrInvalidBranch
	}
	_, err := c.run(ctx, "prepare merge", "merge", "--no-commit", "--no-ff", target)
	return err
}

func (c Client) ConflictFiles(ctx context.Context) ([]string, error) {
	result, err := c.run(ctx, "list conflicts", "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.ReplaceAll(result.Stdout, "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func (c Client) Status(ctx context.Context) (string, error) {
	result, err := c.run(ctx, "show status", "status")
	return result.Stdout, err
}

func (c Client) AbortMerge(ctx context.Context) error {
	_, err := c.run(ctx, "abort merge", "merge", "--abort")
	return err
}

func (c Client) CurrentBranch(ctx context.Context) (string, error) {
	result, err := c.run(ctx, "read current branch", "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(result.Stdout)
	if !validBranch(branch) {
		return "", ErrInvalidBranch
	}
	return branch, nil
}

func (c Client) CurrentCommit(ctx context.Context) (string, error) {
	return c.revision(ctx, "HEAD", "read current commit")
}

func (c Client) MergeHead(ctx context.Context) (string, error) {
	return c.revision(ctx, "MERGE_HEAD", "read merge head")
}

func (c Client) revision(ctx context.Context, revision, operation string) (string, error) {
	result, err := c.run(ctx, operation, "rev-parse", revision)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(result.Stdout)
	if value == "" {
		return "", fmt.Errorf("git %s returned an empty revision", operation)
	}
	return value, nil
}

func splitPaths(output string) []string {
	var paths []string
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths
}

func (c Client) StagedFiles(ctx context.Context) ([]string, error) {
	result, err := c.run(ctx, "list staged files", "diff", "--cached", "--name-only", "--diff-filter=ACDMR")
	if err != nil {
		return nil, err
	}
	return splitPaths(result.Stdout), nil
}

func (c Client) Commit(ctx context.Context, message string) error {
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("commit message must not be empty")
	}
	_, err := c.run(ctx, "commit conflict resolution", "commit", "-m", message)
	return err
}

func (c Client) PushOrigin(ctx context.Context, branch string) error {
	if !validBranch(branch) {
		return ErrInvalidBranch
	}
	_, err := c.run(ctx, "push source branch", "push", "origin", branch)
	return err
}

func (c Client) RemoteContains(ctx context.Context, commit, branch string) (bool, error) {
	if strings.TrimSpace(commit) == "" || !validBranch(branch) {
		return false, ErrInvalidBranch
	}
	_, err := c.run(ctx, "check remote commit", "merge-base", "--is-ancestor", commit, "origin/"+branch)
	if err == nil {
		return true, nil
	}
	var exitErr *processpkg.ExitError
	if errors.As(err, &exitErr) && exitErr.Code == 1 {
		return false, nil
	}
	return false, err
}

func (c Client) GitPath(ctx context.Context, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("Git path name must not be empty")
	}
	result, err := c.run(ctx, "resolve Git path", "rev-parse", "--git-path", name)
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(result.Stdout)
	if path == "" {
		return "", fmt.Errorf("Git returned an empty path for %q", name)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(c.dir, path)
	}
	return filepath.Clean(path), nil
}

func (c Client) OperationState(ctx context.Context) (OperationState, error) {
	checks := []struct {
		state OperationState
		paths []string
	}{
		{MergeOperation, []string{"MERGE_HEAD"}},
		{RebaseOperation, []string{"rebase-merge", "rebase-apply"}},
		{CherryPickOperation, []string{"CHERRY_PICK_HEAD"}},
		{RevertOperation, []string{"REVERT_HEAD"}},
	}
	for _, check := range checks {
		for _, name := range check.paths {
			result, err := c.run(ctx, "inspect operation state", "rev-parse", "--git-path", name)
			if err != nil {
				return NoOperation, err
			}
			path := strings.TrimSpace(result.Stdout)
			if !filepath.IsAbs(path) {
				path = filepath.Join(c.dir, path)
			}
			if _, err := os.Stat(path); err == nil {
				return check.state, nil
			} else if !errors.Is(err, os.ErrNotExist) {
				return NoOperation, err
			}
		}
	}
	return NoOperation, nil
}
