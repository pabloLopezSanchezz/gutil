package conflict

import (
	"testing"
	"time"
)

func TestParsePrepareArgsNewBranchFlags(t *testing.T) {
	tests := []struct {
		args          []string
		newBranch, ok bool
	}{
		{[]string{"source", "target"}, false, true},
		{[]string{"source", "target", "--new-branch"}, true, true},
		{[]string{"source", "target", "--newBranch"}, true, true},
		{[]string{"--new-branch", "source", "target"}, false, false},
		{[]string{"source", "target", "--new-branch", "--newBranch"}, false, false},
		{[]string{"source", "target", "--unknown"}, false, false},
	}
	for _, tt := range tests {
		_, _, options, ok := parsePrepareArgs(tt.args)
		if ok != tt.ok || options.NewBranch != tt.newBranch {
			t.Fatalf("args %v: options/ok = %#v/%v", tt.args, options, ok)
		}
	}
}

func TestResolutionBranchName(t *testing.T) {
	now := time.Date(2026, time.July, 8, 23, 0, 0, 0, time.FixedZone("local", 3600))
	if got := resolutionBranchName("release/next", now); got != "feature/conflictResolution/release/next/08072026" {
		t.Fatalf("name = %q", got)
	}
}
