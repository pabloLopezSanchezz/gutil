package conflict

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gitpkg "github.com/pablo/gutil/internal/git"
	"github.com/pablo/gutil/internal/output"
	processpkg "github.com/pablo/gutil/internal/process"
)

func TestIntegrationConflictStatusAndAbort(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	repo := filepath.Join(root, "repo")
	runGit(t, root, "init", "--bare", origin)
	runGit(t, root, "clone", origin, repo)
	runGit(t, repo, "config", "user.name", "gutil-test")
	runGit(t, repo, "config", "user.email", "gutil-test@example.invalid")
	runGit(t, repo, "checkout", "-b", "main")
	writeFile(t, filepath.Join(repo, "shared.txt"), "base\n")
	runGit(t, repo, "add", "shared.txt")
	runGit(t, repo, "commit", "-m", "initial")
	runGit(t, repo, "push", "-u", "origin", "main")

	runGit(t, repo, "checkout", "-b", "develop")
	writeFile(t, filepath.Join(repo, "shared.txt"), "target\n")
	runGit(t, repo, "commit", "-am", "target change")
	runGit(t, repo, "push", "-u", "origin", "develop")

	runGit(t, repo, "checkout", "main")
	runGit(t, repo, "checkout", "-b", "feature/a")
	writeFile(t, filepath.Join(repo, "shared.txt"), "source\n")
	runGit(t, repo, "commit", "-am", "source change")
	runGit(t, repo, "push", "-u", "origin", "feature/a")

	var stdout, stderr bytes.Buffer
	runner := processpkg.OSRunner{}
	printer := output.Printer{Stdout: &stdout, Stderr: &stderr}
	editor := &fakeEditor{}
	workflow := Workflow{Git: gitpkg.NewClient(runner, repo), Editor: editor, Output: printer}
	command := &Command{Workflow: workflow, Output: printer}

	if code := command.Run([]string{"feature/a", "develop"}); code != 0 {
		t.Fatalf("prepare code = %d\n%s", code, stderr.String())
	}
	if editor.calls != 1 || !strings.Contains(stdout.String(), "shared.txt") {
		t.Fatalf("editor/output = %d/%q", editor.calls, stdout.String())
	}
	if branch := strings.TrimSpace(runGit(t, repo, "branch", "--show-current")); branch != "feature/a" {
		t.Fatalf("branch = %q", branch)
	}
	if state, err := workflow.Git.OperationState(context.Background()); err != nil || state != gitpkg.MergeOperation {
		t.Fatalf("state = %q, err = %v", state, err)
	}

	stdout.Reset()
	if code := command.Run([]string{"--status"}); code != 0 || !strings.Contains(stdout.String(), "shared.txt") {
		t.Fatalf("status code/output = %d/%q", code, stdout.String())
	}
	if code := command.Run([]string{"--abort"}); code != 0 {
		t.Fatalf("abort code = %d", code)
	}
	if status := runGit(t, repo, "status", "--porcelain"); status != "" {
		t.Fatalf("status after abort = %q", status)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
