package cli

import (
	"fmt"
	"io"
)

const usage = `Usage: gutil <command> [options]

Available commands:
  conflict    Prepare or inspect a conflict merge
  update      Install the latest gUtil release
  version     Print the gUtil version
  help        Show this help
`

type ConflictCommand interface {
	Run(args []string) int
}

type UpdateCommand interface {
	Run(currentVersion string) int
}

func Run(args []string, version string, stdout, stderr io.Writer, conflict ConflictCommand, update UpdateCommand) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return 0
	case "version":
		fmt.Fprintf(stdout, "gutil %s\n", version)
		return 0
	case "conflict":
		if conflict == nil {
			fmt.Fprintln(stderr, "Conflict command is unavailable.")
			return 1
		}
		return conflict.Run(args[1:])
	case "update":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "Usage: gutil update")
			return 2
		}
		if update == nil {
			fmt.Fprintln(stderr, "Update command is unavailable.")
			return 1
		}
		return update.Run(version)
	default:
		fmt.Fprintf(stderr, "Unknown command %q.\n\n%s", args[0], usage)
		return 2
	}
}
