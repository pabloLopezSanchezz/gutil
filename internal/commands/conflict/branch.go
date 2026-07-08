package conflict

import (
	"fmt"
	"time"
)

type PrepareOptions struct{ NewBranch bool }

func parsePrepareArgs(args []string) (string, string, PrepareOptions, bool) {
	if len(args) == 2 && validArgument(args[0]) && validArgument(args[1]) && args[0] != args[1] {
		return args[0], args[1], PrepareOptions{}, true
	}
	if len(args) == 3 && validArgument(args[0]) && validArgument(args[1]) && args[0] != args[1] && (args[2] == "--new-branch" || args[2] == "--newBranch") {
		return args[0], args[1], PrepareOptions{NewBranch: true}, true
	}
	return "", "", PrepareOptions{}, false
}

func resolutionBranchName(target string, now time.Time) string {
	return fmt.Sprintf("feature/conflictResolution/%s/%s", target, now.Format("02012006"))
}
