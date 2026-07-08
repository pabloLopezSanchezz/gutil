package scripts_test

import (
	"os"
	"strings"
	"testing"
)

func TestInstallersVerifyChecksumsAndSupportOverrides(t *testing.T) {
	tests := []struct { file string; required []string }{
		{"install.sh", []string{"GUTIL_VERSION", "GUTIL_INSTALL_DIR", "checksums.txt", "sha256", "mktemp", "Darwin", "Linux", "arm64", "amd64"}},
		{"install.ps1", []string{"GUTIL_VERSION", "GUTIL_INSTALL_DIR", "checksums.txt", "Get-FileHash", "Expand-Archive", "User", "ARM64", "AMD64"}},
	}
	for _, tt := range tests {
		content, err := os.ReadFile(tt.file)
		if err != nil { t.Fatal(err) }
		for _, required := range tt.required {
			if !strings.Contains(string(content), required) { t.Errorf("%s does not contain %q", tt.file, required) }
		}
	}
}
