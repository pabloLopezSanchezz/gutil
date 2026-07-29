package update

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	updatepkg "github.com/pabloLopezSanchezz/gutil/internal/update"
)

type stubUpdater struct {
	result  updatepkg.Result
	err     error
	version string
}

func (s *stubUpdater) Update(_ context.Context, version string) (updatepkg.Result, error) {
	s.version = version
	return s.result, s.err
}

func TestCommandReportsUpdateOutcomes(t *testing.T) {
	tests := []struct {
		name   string
		result updatepkg.Result
		err    error
		code   int
		text   string
	}{
		{"updated", updatepkg.Result{Version: "v0.2.0"}, nil, 0, "Updated gUtil to v0.2.0"},
		{"already current", updatepkg.Result{Version: "v0.2.0", UpToDate: true}, nil, 0, "already up to date"},
		{"windows schedule", updatepkg.Result{Version: "v0.2.0", Scheduled: true}, nil, 0, "Open a new terminal"},
		{"failure", updatepkg.Result{}, errors.New("network unavailable"), 1, "Update failed: network unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			updater := &stubUpdater{result: tt.result, err: tt.err}
			command := Command{Updater: updater, Stdout: &stdout, Stderr: &stderr}
			if code := command.Run("v0.1.0"); code != tt.code {
				t.Fatalf("code = %d", code)
			}
			if updater.version != "v0.1.0" {
				t.Fatalf("version = %q", updater.version)
			}
			if output := stdout.String() + stderr.String(); !strings.Contains(output, tt.text) {
				t.Fatalf("output = %q", output)
			}
		})
	}
}
