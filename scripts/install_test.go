package scripts_test

import (
	"os"
	"strings"
	"testing"
)

func TestInstallersVerifyChecksumsAndSupportOverrides(t *testing.T) {
	tests := []struct {
		file     string
		required []string
	}{
		{"install.sh", []string{"GUTIL_VERSION", "GUTIL_INSTALL_DIR", "checksums.txt", "sha256", "mktemp", "Darwin", "Linux", "arm64", "amd64"}},
		{"install.ps1", []string{"GUTIL_VERSION", "GUTIL_INSTALL_DIR", "checksums.txt", "Get-FileHash", "Expand-Archive", "User", "PROCESSOR_ARCHITEW6432", "PROCESSOR_ARCHITECTURE", "ARM64", "AMD64", "Tls12", "UseBasicParsing", "TimeoutSec", "Downloading gUtil", "Verifying checksum", "Installed and verified", "$env:Path", "Unblock-File", "AppLocker", "endpoint-security"}},
	}
	for _, tt := range tests {
		content, err := os.ReadFile(tt.file)
		if err != nil {
			t.Fatal(err)
		}
		for _, required := range tt.required {
			if !strings.Contains(string(content), required) {
				t.Errorf("%s does not contain %q", tt.file, required)
			}
		}
	}
}

func TestWindowsInstallerReportsNetworkFailuresClearly(t *testing.T) {
	content, err := os.ReadFile("install.ps1")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"Check access to github.com and try again", "Check access to api.github.com and try again"} {
		if !strings.Contains(string(content), required) {
			t.Errorf("install.ps1 does not contain actionable error %q", required)
		}
	}
}

func TestWindowsInstallerSupportsWindowsPowerShell(t *testing.T) {
	content, err := os.ReadFile("install.ps1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "RuntimeInformation") {
		t.Fatal("install.ps1 must not depend on RuntimeInformation, which is unavailable in Windows PowerShell 5.1")
	}
}
