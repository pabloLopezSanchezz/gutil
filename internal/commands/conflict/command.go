package conflict

import (
	"context"
	"fmt"
	"strings"

	"github.com/pablo/gutil/internal/output"
)

const usage = `Usage:
  gutil conflict <source> <target>
  gutil conflict --status
  gutil conflict --abort`

type Command struct {
	Workflow Workflow
	Output   output.Printer
}

func (c *Command) Run(args []string) int {
	ctx := context.Background()
	var err error
	switch {
	case len(args) == 1 && args[0] == "--status":
		err = c.Workflow.Status(ctx)
	case len(args) == 1 && args[0] == "--abort":
		err = c.Workflow.Abort(ctx)
	case len(args) == 2 && validArgument(args[0]) && validArgument(args[1]) && args[0] != args[1]:
		err = c.Workflow.Prepare(ctx, args[0], args[1])
	default:
		c.Output.Error(usage)
		return 2
	}
	if err != nil {
		c.Output.Error(describeError(err))
		return 1
	}
	return 0
}

func validArgument(value string) bool { return value != "" && !strings.HasPrefix(value, "-") }

func describeError(err error) string {
	return fmt.Sprintf("%v\nFix the reported issue, inspect the repository with 'gutil conflict --status', and run the command again.", err)
}
