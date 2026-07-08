package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadmeDocumentsPublicContract(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, required := range []string{
		"gutil conflict <source> <target>", "gutil conflict --status", "gutil conflict --abort",
		"gutil version", "origin", "--no-commit --no-ff", "untracked", "Visual Studio Code",
		"macOS", "Linux", "Windows", "gutil conflict feature/ABC develop", "develop into feature/ABC",
		"gutil conflict --continue", "already staged", "[gUtil] Conflict Resolution", "push origin", "retry only the push",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("README does not contain %q", required)
		}
	}
}
