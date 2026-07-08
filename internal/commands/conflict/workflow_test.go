package conflict

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gitpkg "github.com/pablo/gutil/internal/git"
	"github.com/pablo/gutil/internal/output"
)

type fakeGit struct {
	calls          []string
	clean          bool
	dirty          string
	state          gitpkg.OperationState
	locations      map[string]gitpkg.BranchLocation
	conflicts      []string
	status         string
	mergeErr       error
	errAt          string
	branch         string
	currentCommit  string
	mergeHead      string
	staged         []string
	commitMessage  string
	pushErr        error
	remoteContains bool
}

func (f *fakeGit) record(call string) error {
	f.calls = append(f.calls, call)
	if f.errAt == call {
		return errors.New("forced failure")
	}
	return nil
}
func (f *fakeGit) ValidateRepository(context.Context) error { return f.record("validate") }
func (f *fakeGit) WorkingTreeClean(context.Context) (bool, string, error) {
	if err := f.record("clean"); err != nil {
		return false, "", err
	}
	return f.clean, f.dirty, nil
}
func (f *fakeGit) FetchOrigin(context.Context) error { return f.record("fetch") }
func (f *fakeGit) BranchLocation(_ context.Context, branch string) (gitpkg.BranchLocation, error) {
	if err := f.record("locate " + branch); err != nil {
		return 0, err
	}
	return f.locations[branch], nil
}
func (f *fakeGit) CreateTrackingBranch(_ context.Context, branch string) error {
	return f.record("track " + branch)
}
func (f *fakeGit) Checkout(_ context.Context, branch string) error {
	return f.record("checkout " + branch)
}
func (f *fakeGit) PullOrigin(_ context.Context, branch string) error {
	return f.record("pull " + branch)
}
func (f *fakeGit) MergeNoCommit(_ context.Context, branch string) error {
	f.calls = append(f.calls, "merge "+branch)
	return f.mergeErr
}
func (f *fakeGit) ConflictFiles(context.Context) ([]string, error) {
	if err := f.record("conflicts"); err != nil {
		return nil, err
	}
	return f.conflicts, nil
}
func (f *fakeGit) Status(context.Context) (string, error) {
	if err := f.record("status"); err != nil {
		return "", err
	}
	return f.status, nil
}
func (f *fakeGit) AbortMerge(context.Context) error { return f.record("abort") }
func (f *fakeGit) OperationState(context.Context) (gitpkg.OperationState, error) {
	if err := f.record("operation"); err != nil {
		return "", err
	}
	return f.state, nil
}
func (f *fakeGit) CurrentBranch(context.Context) (string, error) {
	if err := f.record("current branch"); err != nil {
		return "", err
	}
	return f.branch, nil
}
func (f *fakeGit) CurrentCommit(context.Context) (string, error) {
	if err := f.record("current commit"); err != nil {
		return "", err
	}
	return f.currentCommit, nil
}
func (f *fakeGit) MergeHead(context.Context) (string, error) {
	if err := f.record("merge head"); err != nil {
		return "", err
	}
	return f.mergeHead, nil
}
func (f *fakeGit) StagedFiles(context.Context) ([]string, error) {
	if err := f.record("staged"); err != nil {
		return nil, err
	}
	return f.staged, nil
}
func (f *fakeGit) Commit(_ context.Context, message string) error {
	f.commitMessage = message
	if err := f.record("commit"); err != nil {
		return err
	}
	f.currentCommit = "resolved"
	f.state = gitpkg.NoOperation
	return nil
}
func (f *fakeGit) PushOrigin(_ context.Context, branch string) error {
	f.calls = append(f.calls, "push "+branch)
	return f.pushErr
}
func (f *fakeGit) RemoteContains(context.Context, string, string) (bool, error) {
	if err := f.record("remote contains"); err != nil {
		return false, err
	}
	return f.remoteContains, nil
}
func (f *fakeGit) GitPath(context.Context, string) (string, error) {
	return "", errors.New("test workflow must inject a store")
}
func (f *fakeGit) ValidateBranch(_ context.Context, branch string) error {
	return f.record("validate branch " + branch)
}
func (f *fakeGit) CreateBranch(_ context.Context, branch string) error {
	if err := f.record("create branch " + branch); err != nil {
		return err
	}
	f.branch = branch
	return nil
}

type fakeEditor struct {
	calls int
	err   error
}

func (e *fakeEditor) Open(context.Context, string) error { e.calls++; return e.err }

func newCommand(t *testing.T, g *fakeGit, editor Editor) (*Command, *bytes.Buffer, *bytes.Buffer, StateStore) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	printer := output.Printer{Stdout: &stdout, Stderr: &stderr}
	store := StateStore{Path: filepath.Join(t.TempDir(), "state.json")}
	workflow := Workflow{Git: g, Editor: editor, Output: printer, Store: &store}
	return &Command{Workflow: workflow, Output: printer}, &stdout, &stderr, store
}

func TestCommandRejectsInvalidArguments(t *testing.T) {
	tests := [][]string{{}, {"one"}, {"a", "a"}, {"--status", "extra"}, {"--bad"}, {"a", "b", "c"}}
	for _, args := range tests {
		command, _, stderr, _ := newCommand(t, &fakeGit{}, &fakeEditor{})
		if code := command.Run(args); code != 2 || !strings.Contains(stderr.String(), "Usage:") {
			t.Fatalf("args %v: code/output = %d/%q", args, code, stderr.String())
		}
	}
}

func TestPrepareRunsExpectedSequenceAndOpensEditorForConflicts(t *testing.T) {
	g := &fakeGit{clean: true, locations: map[string]gitpkg.BranchLocation{"develop": gitpkg.Local, "feature/a": gitpkg.RemoteOnly}, conflicts: []string{"file.txt"}, mergeErr: errors.New("merge conflicts"), branch: "feature/a", currentCommit: "source", mergeHead: "target"}
	editor := &fakeEditor{}
	command, stdout, _, _ := newCommand(t, g, editor)
	if code := command.Run([]string{"feature/a", "develop"}); code != 0 {
		t.Fatalf("code = %d", code)
	}
	want := "validate,operation,clean,fetch,locate develop,checkout develop,pull develop,locate feature/a,track feature/a,pull feature/a,current branch,current commit,merge develop,conflicts,merge head"
	if got := strings.Join(g.calls, ","); got != want {
		t.Fatalf("calls = %s\nwant  = %s", got, want)
	}
	if editor.calls != 1 || !strings.Contains(stdout.String(), "file.txt") {
		t.Fatalf("editor calls/output = %d/%q", editor.calls, stdout.String())
	}
}

func TestPrepareCreatesDatedBranchFromUpdatedSource(t *testing.T) {
	g := &fakeGit{clean: true, locations: map[string]gitpkg.BranchLocation{"develop": gitpkg.Local, "feature/a": gitpkg.Local}, conflicts: []string{"file.txt"}, mergeErr: errors.New("conflict"), branch: "feature/a", currentCommit: "source", mergeHead: "target"}
	command, _, _, store := newCommand(t, g, &fakeEditor{})
	command.Workflow.Clock = func() time.Time { return time.Date(2026, time.July, 8, 12, 0, 0, 0, time.Local) }
	if code := command.Run([]string{"feature/a", "develop", "--new-branch"}); code != 0 {
		t.Fatalf("code = %d", code)
	}
	generated := "feature/conflictResolution/develop/08072026"
	calls := strings.Join(g.calls, ",")
	if !strings.Contains(calls, "current commit,validate branch "+generated+",locate "+generated+",create branch "+generated+",current branch,current commit,merge develop") {
		t.Fatalf("calls = %s", calls)
	}
	state, err := store.Load()
	if err != nil || state.SourceBranch != generated || state.OriginalSourceBranch != "feature/a" || !state.GeneratedBranch {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
}

func TestPrepareStopsForDirtyTree(t *testing.T) {
	g := &fakeGit{clean: false, dirty: "?? local.txt\n"}
	command, _, stderr, _ := newCommand(t, g, &fakeEditor{})
	if code := command.Run([]string{"feature/a", "develop"}); code != 1 {
		t.Fatalf("code = %d", code)
	}
	if got := strings.Join(g.calls, ","); got != "validate,operation,clean" {
		t.Fatalf("calls = %s", got)
	}
	if !strings.Contains(stderr.String(), "local.txt") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestStatusAndAbort(t *testing.T) {
	g := &fakeGit{status: "On branch feature/a\n", conflicts: []string{"file.txt"}, state: gitpkg.MergeOperation}
	command, stdout, _, _ := newCommand(t, g, &fakeEditor{})
	if code := command.Run([]string{"--status"}); code != 0 {
		t.Fatalf("status code = %d", code)
	}
	if !strings.Contains(stdout.String(), "On branch") || !strings.Contains(stdout.String(), "file.txt") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if code := command.Run([]string{"--abort"}); code != 0 {
		t.Fatalf("abort code = %d", code)
	}
	if !strings.Contains(strings.Join(g.calls, ","), "abort") {
		t.Fatalf("calls = %v", g.calls)
	}
}

func TestStatusReportsCommittedGUtilStateWithoutUnmergedFiles(t *testing.T) {
	g := &fakeGit{status: "On branch feature/a\n"}
	command, stdout, _, store := newCommand(t, g, &fakeEditor{})
	if err := store.Save(ConflictState{Version: 2, SourceBranch: "feature/a", TargetBranch: "develop", SourceCommit: "source", MergeCommit: "target", ConflictFiles: []string{"a.txt"}, Phase: PhaseCommitted, Commit: "resolved"}); err != nil {
		t.Fatal(err)
	}
	if code := command.Run([]string{"--status"}); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(stdout.String(), "committed") || !strings.Contains(stdout.String(), "feature/a -> develop") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestEditorFailureIsWarningNotCommandFailure(t *testing.T) {
	g := &fakeGit{clean: true, locations: map[string]gitpkg.BranchLocation{"develop": gitpkg.Local, "feature/a": gitpkg.Local}, conflicts: []string{"file.txt"}, mergeErr: errors.New("conflict"), branch: "feature/a", currentCommit: "source", mergeHead: "target"}
	command, _, stderr, _ := newCommand(t, g, &fakeEditor{err: errors.New("code missing")})
	if code := command.Run([]string{"feature/a", "develop"}); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(stderr.String(), "could not be opened") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestCommandContinueCommitsAndPushesStagedResolution(t *testing.T) {
	g := &fakeGit{state: gitpkg.MergeOperation, branch: "feature/a", currentCommit: "source", mergeHead: "target", staged: []string{"a.txt", "b.txt"}}
	command, _, _, store := newCommand(t, g, &fakeEditor{})
	if err := store.Save(ConflictState{Version: 2, SourceBranch: "feature/a", TargetBranch: "develop", SourceCommit: "source", MergeCommit: "target", ConflictFiles: []string{"a.txt", "b.txt"}, Phase: PhaseResolving}); err != nil {
		t.Fatal(err)
	}
	if code := command.Run([]string{"--continue"}); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if g.commitMessage != "[gUtil] Conflict Resolution - 2 files fixed." {
		t.Fatalf("message = %q", g.commitMessage)
	}
	if !strings.Contains(strings.Join(g.calls, ","), "push feature/a") {
		t.Fatalf("calls = %v", g.calls)
	}
	if _, err := store.Load(); !errors.Is(err, ErrStateNotFound) {
		t.Fatalf("state remains: %v", err)
	}
}

func TestContinueListsUnresolvedFiles(t *testing.T) {
	g := &fakeGit{state: gitpkg.MergeOperation, branch: "feature/a", currentCommit: "source", mergeHead: "target", conflicts: []string{"a.txt"}}
	command, _, stderr, store := newCommand(t, g, &fakeEditor{})
	if err := store.Save(ConflictState{Version: 2, SourceBranch: "feature/a", TargetBranch: "develop", SourceCommit: "source", MergeCommit: "target", ConflictFiles: []string{"a.txt"}, Phase: PhaseResolving}); err != nil {
		t.Fatal(err)
	}
	if code := command.Run([]string{"--continue"}); code != 1 || !strings.Contains(stderr.String(), "a.txt") {
		t.Fatalf("code/output = %d/%q", code, stderr.String())
	}
	if g.commitMessage != "" {
		t.Fatalf("commit attempted: %q", g.commitMessage)
	}
}

func TestContinuePreservesCommittedStateWhenPushFails(t *testing.T) {
	g := &fakeGit{state: gitpkg.MergeOperation, branch: "feature/a", currentCommit: "source", mergeHead: "target", staged: []string{"a.txt"}, pushErr: errors.New("rejected")}
	command, _, _, store := newCommand(t, g, &fakeEditor{})
	if err := store.Save(ConflictState{Version: 2, SourceBranch: "feature/a", TargetBranch: "develop", SourceCommit: "source", MergeCommit: "target", ConflictFiles: []string{"a.txt"}, Phase: PhaseResolving}); err != nil {
		t.Fatal(err)
	}
	if code := command.Run([]string{"--continue"}); code != 1 {
		t.Fatalf("code = %d", code)
	}
	state, err := store.Load()
	if err != nil || state.Phase != PhaseCommitted || state.Commit != "resolved" {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
	if g.commitMessage != "[gUtil] Conflict Resolution - 1 file fixed." {
		t.Fatalf("message = %q", g.commitMessage)
	}
}

func TestContinueRejectsResolvedButUnstagedFile(t *testing.T) {
	g := &fakeGit{state: gitpkg.MergeOperation, branch: "feature/a", currentCommit: "source", mergeHead: "target", staged: []string{"other.txt"}}
	command, _, stderr, store := newCommand(t, g, &fakeEditor{})
	if err := store.Save(ConflictState{Version: 2, SourceBranch: "feature/a", TargetBranch: "develop", SourceCommit: "source", MergeCommit: "target", ConflictFiles: []string{"a.txt"}, Phase: PhaseResolving}); err != nil {
		t.Fatal(err)
	}
	if code := command.Run([]string{"--continue"}); code != 1 || !strings.Contains(stderr.String(), "a.txt") || !strings.Contains(stderr.String(), "not staged") {
		t.Fatalf("code/output = %d/%q", code, stderr.String())
	}
}

func TestContinueCommittedStateRetriesOnlyPush(t *testing.T) {
	g := &fakeGit{state: gitpkg.NoOperation, branch: "feature/a", currentCommit: "resolved"}
	command, _, _, store := newCommand(t, g, &fakeEditor{})
	if err := store.Save(ConflictState{Version: 2, SourceBranch: "feature/a", TargetBranch: "develop", SourceCommit: "source", MergeCommit: "target", ConflictFiles: []string{"a.txt"}, Phase: PhaseCommitted, Commit: "resolved"}); err != nil {
		t.Fatal(err)
	}
	if code := command.Run([]string{"--continue"}); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if g.commitMessage != "" || !strings.Contains(strings.Join(g.calls, ","), "push feature/a") {
		t.Fatalf("message/calls = %q/%v", g.commitMessage, g.calls)
	}
	if _, err := store.Load(); !errors.Is(err, ErrStateNotFound) {
		t.Fatalf("state remains: %v", err)
	}
}
