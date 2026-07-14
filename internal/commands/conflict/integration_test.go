package conflict

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gitpkg "github.com/pabloLopezSanchezz/gutil/internal/git"
	"github.com/pabloLopezSanchezz/gutil/internal/output"
	processpkg "github.com/pabloLopezSanchezz/gutil/internal/process"
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

func TestIntegrationContinueRetriesRejectedPushWithoutSecondCommit(t *testing.T) {
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
	workflow := Workflow{Git: gitpkg.NewClient(runner, repo), Editor: &fakeEditor{}, Output: printer}
	command := &Command{Workflow: workflow, Output: printer}
	if code := command.Run([]string{"feature/a", "develop"}); code != 0 {
		t.Fatalf("prepare code = %d: %s", code, stderr.String())
	}

	// Resolve to the source (HEAD) version. Git records the conflict as resolved,
	// but the path intentionally disappears from the cached diff.
	writeFile(t, filepath.Join(repo, "shared.txt"), "source\n")
	runGit(t, repo, "add", "shared.txt")
	if staged := strings.TrimSpace(runGit(t, repo, "diff", "--cached", "--name-only")); staged != "" {
		t.Fatalf("expected no cached diff for ours resolution, got %q", staged)
	}
	hook := filepath.Join(origin, "hooks", "pre-receive")
	writeFile(t, hook, "#!/bin/sh\necho rejected >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}

	stderr.Reset()
	if code := command.Run([]string{"--continue"}); code != 1 || !strings.Contains(stderr.String(), "push failed") {
		t.Fatalf("first continue code/output = %d/%q", code, stderr.String())
	}
	firstCommit := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	if message := strings.TrimSpace(runGit(t, repo, "log", "-1", "--pretty=%s")); message != "[gUtil] Conflict Resolution - 1 file fixed." {
		t.Fatalf("message = %q", message)
	}

	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if code := command.Run([]string{"--continue"}); code != 0 {
		t.Fatalf("retry code = %d: %s", code, stderr.String())
	}
	secondCommit := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	remoteCommit := strings.TrimSpace(runGit(t, repo, "rev-parse", "origin/feature/a"))
	if secondCommit != firstCommit || remoteCommit != firstCommit {
		t.Fatalf("commits local1/local2/remote = %s/%s/%s", firstCommit, secondCommit, remoteCommit)
	}
	statePath := strings.TrimSpace(runGit(t, repo, "rev-parse", "--git-path", "gutil/conflict-state.json"))
	if !filepath.IsAbs(statePath) {
		statePath = filepath.Join(repo, statePath)
	}
	if _, err := os.Stat(statePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("state still exists: %v", err)
	}
}

func TestIntegrationNewBranchPreservesProtectedSource(t *testing.T) {
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
	protectedCommit := strings.TrimSpace(runGit(t, repo, "rev-parse", "feature/a"))

	var stdout, stderr bytes.Buffer
	runner := processpkg.OSRunner{}
	printer := output.Printer{Stdout: &stdout, Stderr: &stderr}
	workflow := Workflow{Git: gitpkg.NewClient(runner, repo), Editor: &fakeEditor{}, Output: printer, Clock: func() time.Time { return time.Date(2026, time.July, 8, 12, 0, 0, 0, time.Local) }}
	command := &Command{Workflow: workflow, Output: printer}
	generated := "feature/conflictResolution/develop/08072026"
	if code := command.Run([]string{"feature/a", "develop", "--new-branch"}); code != 0 {
		t.Fatalf("prepare code = %d: %s", code, stderr.String())
	}
	if branch := strings.TrimSpace(runGit(t, repo, "branch", "--show-current")); branch != generated {
		t.Fatalf("branch = %q", branch)
	}
	writeFile(t, filepath.Join(repo, "shared.txt"), "resolved\n")
	runGit(t, repo, "add", "shared.txt")
	if code := command.Run([]string{"--continue"}); code != 0 {
		t.Fatalf("continue code = %d: %s", code, stderr.String())
	}
	generatedCommit := strings.TrimSpace(runGit(t, repo, "rev-parse", generated))
	remoteGenerated := strings.TrimSpace(runGit(t, repo, "rev-parse", "origin/"+generated))
	if generatedCommit != remoteGenerated {
		t.Fatalf("generated local/remote = %s/%s", generatedCommit, remoteGenerated)
	}
	if local := strings.TrimSpace(runGit(t, repo, "rev-parse", "feature/a")); local != protectedCommit {
		t.Fatalf("local protected branch changed: %s", local)
	}
	if remote := strings.TrimSpace(runGit(t, repo, "rev-parse", "origin/feature/a")); remote != protectedCommit {
		t.Fatalf("remote protected branch changed: %s", remote)
	}
	runGit(t, repo, "checkout", "feature/a")
	stderr.Reset()
	if code := command.Run([]string{"feature/a", "develop", "--new-branch"}); code != 1 || !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("collision code/output = %d/%q", code, stderr.String())
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
