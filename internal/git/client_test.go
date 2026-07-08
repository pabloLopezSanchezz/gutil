package git

import (
	"context"
	"errors"
	"reflect"
	"testing"

	processpkg "github.com/pablo/gutil/internal/process"
)

type recordedRunner struct {
	commands []processpkg.Command
	results  []processpkg.Result
	errors   []error
}

func (r *recordedRunner) Run(_ context.Context, command processpkg.Command) (processpkg.Result, error) {
	r.commands = append(r.commands, command)
	index := len(r.commands) - 1
	var result processpkg.Result
	var err error
	if index < len(r.results) {
		result = r.results[index]
	}
	if index < len(r.errors) {
		err = r.errors[index]
	}
	return result, err
}

func TestGitCommands(t *testing.T) {
	tests := []struct {
		name   string
		call   func(Client) error
		args   []string
		stdout string
	}{
		{"validate", func(c Client) error { return c.ValidateRepository(context.Background()) }, []string{"rev-parse", "--is-inside-work-tree"}, "true\n"},
		{"fetch", func(c Client) error { return c.FetchOrigin(context.Background()) }, []string{"fetch", "origin", "--prune"}, ""},
		{"track", func(c Client) error { return c.CreateTrackingBranch(context.Background(), "feature/a") }, []string{"checkout", "--track", "-b", "feature/a", "origin/feature/a"}, ""},
		{"checkout", func(c Client) error { return c.Checkout(context.Background(), "develop") }, []string{"checkout", "develop"}, ""},
		{"pull", func(c Client) error { return c.PullOrigin(context.Background(), "develop") }, []string{"pull", "origin", "develop"}, ""},
		{"merge", func(c Client) error { return c.MergeNoCommit(context.Background(), "develop") }, []string{"merge", "--no-commit", "--no-ff", "develop"}, ""},
		{"abort", func(c Client) error { return c.AbortMerge(context.Background()) }, []string{"merge", "--abort"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &recordedRunner{results: []processpkg.Result{{Stdout: tt.stdout}}}
			if err := tt.call(NewClient(runner, "/repo")); err != nil {
				t.Fatal(err)
			}
			if got := runner.commands[0]; got.Name != "git" || got.Dir != "/repo" || !reflect.DeepEqual(got.Args, tt.args) {
				t.Fatalf("command = %#v, want args %#v", got, tt.args)
			}
		})
	}
}

func TestBranchLocation(t *testing.T) {
	notFound := &processpkg.ExitError{Code: 1}
	tests := []struct {
		name   string
		errors []error
		want   BranchLocation
	}{
		{"local", nil, Local},
		{"remote only", []error{notFound, nil}, RemoteOnly},
		{"missing", []error{notFound, notFound}, Missing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &recordedRunner{errors: tt.errors}
			got, err := NewClient(runner, "/repo").BranchLocation(context.Background(), "feature/a")
			if err != nil || got != tt.want {
				t.Fatalf("location = %v, err = %v", got, err)
			}
		})
	}
}

func TestRejectsUnsafeBranchName(t *testing.T) {
	runner := &recordedRunner{}
	err := NewClient(runner, "/repo").Checkout(context.Background(), "--force")
	if !errors.Is(err, ErrInvalidBranch) || len(runner.commands) != 0 {
		t.Fatalf("err = %v, commands = %v", err, runner.commands)
	}
}

func TestConflictFiles(t *testing.T) {
	runner := &recordedRunner{results: []processpkg.Result{{Stdout: "a.txt\r\nb.txt\n\n"}}}
	files, err := NewClient(runner, "/repo").ConflictFiles(context.Background())
	if err != nil || !reflect.DeepEqual(files, []string{"a.txt", "b.txt"}) {
		t.Fatalf("files = %#v, err = %v", files, err)
	}
}
