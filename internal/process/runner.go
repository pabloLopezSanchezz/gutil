package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Command struct {
	Name string
	Args []string
	Dir  string
	Env  []string
}

type Result struct {
	Stdout string
	Stderr string
}

type Runner interface {
	Run(context.Context, Command) (Result, error)
}

type ExitError struct {
	Command Command
	Code    int
	Output  string
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("%s exited with code %d: %s", e.Command.Name, e.Code, strings.TrimSpace(e.Output))
}

type OSRunner struct{}

func (OSRunner) Run(ctx context.Context, command Command) (Result, error) {
	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	cmd.Dir = command.Dir
	if len(command.Env) > 0 {
		cmd.Env = append(os.Environ(), command.Env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	var processErr *exec.ExitError
	if errors.As(err, &processErr) {
		return result, &ExitError{Command: command, Code: processErr.ExitCode(), Output: result.Stdout + result.Stderr}
	}
	return result, fmt.Errorf("start %s: %w", command.Name, err)
}
