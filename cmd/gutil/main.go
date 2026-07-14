package main

import (
	"os"

	"github.com/pabloLopezSanchezz/gutil/internal/cli"
	"github.com/pabloLopezSanchezz/gutil/internal/commands/conflict"
	gitpkg "github.com/pabloLopezSanchezz/gutil/internal/git"
	"github.com/pabloLopezSanchezz/gutil/internal/output"
	processpkg "github.com/pabloLopezSanchezz/gutil/internal/process"
)

var version = "dev"

func main() {
	runner := processpkg.OSRunner{}
	printer := output.Printer{Stdout: os.Stdout, Stderr: os.Stderr}
	gitClient := gitpkg.NewClient(runner, "")
	workflow := conflict.Workflow{
		Git:    gitClient,
		Editor: conflict.CodeEditor{Runner: runner, Dir: ""},
		Output: printer,
	}
	command := &conflict.Command{Workflow: workflow, Output: printer}
	os.Exit(cli.Run(os.Args[1:], version, os.Stdout, os.Stderr, command))
}
