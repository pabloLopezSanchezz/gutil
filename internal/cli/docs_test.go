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
		"--new-branch", "--newBranch", "feature/conflictResolution", "protected source branch", "already exists", "does not delete the generated branch",
		"curl -fsSL https://raw.githubusercontent.com/pabloLopezSanchezz/gutil/main/scripts/install.sh | env GUTIL_VERSION=v0.1.0 sh",
		"curl -fsSL https://raw.githubusercontent.com/pabloLopezSanchezz/gutil/main/scripts/install.sh | env GUTIL_INSTALL_DIR=\"$HOME/bin\" sh",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("README does not contain %q", required)
		}
	}
}
