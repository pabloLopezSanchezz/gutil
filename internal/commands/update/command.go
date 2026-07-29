package update

import (
	"context"
	"fmt"
	"io"

	updatepkg "github.com/pabloLopezSanchezz/gutil/internal/update"
)

type Updater interface {
	Update(context.Context, string) (updatepkg.Result, error)
}

type Command struct {
	Updater Updater
	Stdout  io.Writer
	Stderr  io.Writer
}

func (c Command) Run(currentVersion string) int {
	result, err := c.Updater.Update(context.Background(), currentVersion)
	if err != nil {
		fmt.Fprintf(c.Stderr, "Update failed: %v\n", err)
		return 1
	}
	if result.UpToDate {
		fmt.Fprintf(c.Stdout, "gUtil %s is already up to date.\n", result.Version)
		return 0
	}
	if result.Scheduled {
		fmt.Fprintf(c.Stdout, "gUtil %s will replace the current executable in a moment. Open a new terminal before using it.\n", result.Version)
		return 0
	}
	fmt.Fprintf(c.Stdout, "Updated gUtil to %s.\n", result.Version)
	return 0
}
