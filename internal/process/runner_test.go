package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestOSRunnerCapturesOutputAndDirectory(t *testing.T) {
	dir := t.TempDir()
	result, err := (OSRunner{}).Run(context.Background(), Command{
		Name: os.Args[0],
		Args: []string{"-test.run=TestHelperProcess", "--", "success"},
		Dir:  dir,
		Env:  []string{"GO_WANT_HELPER_PROCESS=1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Stdout, dir) || result.Stderr != "warning\n" {
		t.Fatalf("result = %#v", result)
	}
}

func TestOSRunnerReturnsTypedExitError(t *testing.T) {
	command := Command{Name: os.Args[0], Args: []string{"-test.run=TestHelperProcess", "--", "failure"}, Env: []string{"GO_WANT_HELPER_PROCESS=1"}}
	result, err := (OSRunner{}).Run(context.Background(), command)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error %v is not ExitError", err)
	}
	if exitErr.Code != 7 || !strings.Contains(exitErr.Output, "failed") || result.Stdout != "failed\n" {
		t.Fatalf("exit error = %#v, result = %#v", exitErr, result)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	if mode == "success" {
		wd, _ := os.Getwd()
		fmt.Fprintln(os.Stdout, wd)
		fmt.Fprintln(os.Stderr, "warning")
		os.Exit(0)
	}
	fmt.Fprintln(os.Stdout, "failed")
	fmt.Fprintln(os.Stderr, "details")
	os.Exit(7)
}
