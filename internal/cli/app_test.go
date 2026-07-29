package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelpAndVersion(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		version  string
		wantCode int
		wantText string
	}{
		{"help command", []string{"help"}, "dev", 0, "Usage: gutil <command>"},
		{"help flag", []string{"--help"}, "dev", 0, "Available commands:"},
		{"version", []string{"version"}, "v0.1.0", 0, "gutil v0.1.0"},
		{"missing command", nil, "dev", 2, "Usage: gutil <command>"},
		{"unknown command", []string{"unknown"}, "dev", 2, `Unknown command "unknown"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, tt.version, &stdout, &stderr, nil, nil)
			if code != tt.wantCode {
				t.Fatalf("Run() code = %d, want %d", code, tt.wantCode)
			}
			if got := stdout.String() + stderr.String(); !strings.Contains(got, tt.wantText) {
				t.Fatalf("output %q does not contain %q", got, tt.wantText)
			}
		})
	}
}

type stubConflict struct {
	args []string
	code int
}

func (s *stubConflict) Run(args []string) int {
	s.args = args
	return s.code
}

func TestRunDispatchesConflictArguments(t *testing.T) {
	command := &stubConflict{code: 7}
	code := Run([]string{"conflict", "feature/a", "develop"}, "dev", &bytes.Buffer{}, &bytes.Buffer{}, command, nil)
	if code != 7 {
		t.Fatalf("Run() code = %d, want 7", code)
	}
	if got := strings.Join(command.args, " "); got != "feature/a develop" {
		t.Fatalf("conflict args = %q", got)
	}
}

type stubUpdate struct {
	version string
	code    int
}

func (s *stubUpdate) Run(version string) int {
	s.version = version
	return s.code
}

func TestRunDispatchesUpdate(t *testing.T) {
	command := &stubUpdate{code: 4}
	code := Run([]string{"update"}, "v0.1.0", &bytes.Buffer{}, &bytes.Buffer{}, nil, command)
	if code != 4 || command.version != "v0.1.0" {
		t.Fatalf("code/version = %d/%q", code, command.version)
	}
}
