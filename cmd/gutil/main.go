package main

import (
	"os"

	"github.com/pabloLopezSanchezz/gutil/internal/cli"
	"github.com/pabloLopezSanchezz/gutil/internal/commands/conflict"
	updatecommand "github.com/pabloLopezSanchezz/gutil/internal/commands/update"
	gitpkg "github.com/pabloLopezSanchezz/gutil/internal/git"
	"github.com/pabloLopezSanchezz/gutil/internal/output"
	processpkg "github.com/pabloLopezSanchezz/gutil/internal/process"
	updatepkg "github.com/pabloLopezSanchezz/gutil/internal/update"
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
	updater := updatecommand.Command{Updater: updatepkg.Service{}, Stdout: os.Stdout, Stderr: os.Stderr}
	os.Exit(cli.Run(os.Args[1:], version, os.Stdout, os.Stderr, command, updater))
}
