package conflict

import (
	"context"

	processpkg "github.com/pablo/gutil/internal/process"
)

type CodeEditor struct {
	Runner processpkg.Runner
	Dir    string
}

func (e CodeEditor) Open(ctx context.Context, path string) error {
	_, err := e.Runner.Run(ctx, processpkg.Command{Name: "code", Args: []string{path}, Dir: e.Dir})
	return err
}
