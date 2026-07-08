package conflict

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gitpkg "github.com/pablo/gutil/internal/git"
	"github.com/pablo/gutil/internal/output"
)

type fakeGit struct {
	calls     []string
	clean     bool
	dirty     string
	state     gitpkg.OperationState
	locations map[string]gitpkg.BranchLocation
	conflicts []string
	status    string
	mergeErr  error
	errAt     string
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

type fakeEditor struct {
	calls int
	err   error
}

func (e *fakeEditor) Open(context.Context, string) error { e.calls++; return e.err }

func newCommand(g *fakeGit, editor Editor) (*Command, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	printer := output.Printer{Stdout: &stdout, Stderr: &stderr}
	workflow := Workflow{Git: g, Editor: editor, Output: printer}
	return &Command{Workflow: workflow, Output: printer}, &stdout, &stderr
}

func TestCommandRejectsInvalidArguments(t *testing.T) {
	tests := [][]string{{}, {"one"}, {"a", "a"}, {"--status", "extra"}, {"--bad"}, {"a", "b", "c"}}
	for _, args := range tests {
		command, _, stderr := newCommand(&fakeGit{}, &fakeEditor{})
		if code := command.Run(args); code != 2 || !strings.Contains(stderr.String(), "Usage:") {
			t.Fatalf("args %v: code/output = %d/%q", args, code, stderr.String())
		}
	}
}

func TestPrepareRunsExpectedSequenceAndOpensEditorForConflicts(t *testing.T) {
	g := &fakeGit{clean: true, locations: map[string]gitpkg.BranchLocation{"develop": gitpkg.Local, "feature/a": gitpkg.RemoteOnly}, conflicts: []string{"file.txt"}, mergeErr: errors.New("merge conflicts")}
	editor := &fakeEditor{}
	command, stdout, _ := newCommand(g, editor)
	if code := command.Run([]string{"feature/a", "develop"}); code != 0 {
		t.Fatalf("code = %d", code)
	}
	want := "validate,operation,clean,fetch,locate develop,checkout develop,pull develop,locate feature/a,track feature/a,pull feature/a,merge develop,conflicts"
	if got := strings.Join(g.calls, ","); got != want {
		t.Fatalf("calls = %s\nwant  = %s", got, want)
	}
	if editor.calls != 1 || !strings.Contains(stdout.String(), "file.txt") {
		t.Fatalf("editor calls/output = %d/%q", editor.calls, stdout.String())
	}
}

func TestPrepareStopsForDirtyTree(t *testing.T) {
	g := &fakeGit{clean: false, dirty: "?? local.txt\n"}
	command, _, stderr := newCommand(g, &fakeEditor{})
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
	command, stdout, _ := newCommand(g, &fakeEditor{})
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

func TestEditorFailureIsWarningNotCommandFailure(t *testing.T) {
	g := &fakeGit{clean: true, locations: map[string]gitpkg.BranchLocation{"develop": gitpkg.Local, "feature/a": gitpkg.Local}, conflicts: []string{"file.txt"}, mergeErr: errors.New("conflict")}
	command, _, stderr := newCommand(g, &fakeEditor{err: errors.New("code missing")})
	if code := command.Run([]string{"feature/a", "develop"}); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(stderr.String(), "could not be opened") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
