package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateReplacesCurrentExecutableAfterChecksumVerification(t *testing.T) {
	archive := tarArchive(t, "gutil", []byte("new binary"))
	server := updateServer(t, "v0.2.0", "gutil_v0.2.0_linux_amd64.tar.gz", archive)
	destination := filepath.Join(t.TempDir(), "gutil")
	if err := os.WriteFile(destination, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := Service{ReleaseURL: server.URL + "/latest", DownloadBaseURL: server.URL, ExecutablePath: func() (string, error) { return destination, nil }, GOOS: "linux", GOARCH: "amd64"}.Update(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != "v0.2.0" || result.UpToDate || result.Scheduled {
		t.Fatalf("result = %#v", result)
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new binary" {
		t.Fatalf("binary = %q", content)
	}
}

func TestUpdateSchedulesWindowsReplacement(t *testing.T) {
	archive := zipArchive(t, "gutil.exe", []byte("new windows binary"))
	server := updateServer(t, "v0.2.0", "gutil_v0.2.0_windows_amd64.zip", archive)
	destination := filepath.Join(t.TempDir(), "gutil.exe")
	var source string
	result, err := Service{
		ReleaseURL:      server.URL + "/latest",
		DownloadBaseURL: server.URL,
		ExecutablePath:  func() (string, error) { return destination, nil },
		GOOS:            "windows",
		GOARCH:          "amd64",
		StartWindowsReplace: func(gotSource, gotDestination string) error {
			source = gotSource
			if gotDestination != destination {
				t.Fatalf("destination = %q", gotDestination)
			}
			return nil
		},
	}.Update(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(source)
	if !result.Scheduled || result.Version != "v0.2.0" {
		t.Fatalf("result = %#v", result)
	}
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new windows binary" {
		t.Fatalf("scheduled binary = %q", content)
	}
}

func TestUpdateDoesNotDownloadAssetsWhenAlreadyCurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/latest" {
			t.Fatalf("unexpected request %s", request.URL.Path)
		}
		_, _ = io.WriteString(writer, `{"tag_name":"v0.2.0"}`)
	}))
	defer server.Close()

	result, err := Service{ReleaseURL: server.URL + "/latest", DownloadBaseURL: server.URL, GOOS: "linux", GOARCH: "amd64"}.Update(context.Background(), "v0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if !result.UpToDate || result.Version != "v0.2.0" {
		t.Fatalf("result = %#v", result)
	}
}

func TestUpdateRefusesInvalidChecksum(t *testing.T) {
	archive := tarArchive(t, "gutil", []byte("new binary"))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/latest":
			_, _ = io.WriteString(writer, `{"tag_name":"v0.2.0"}`)
		case "/v0.2.0/gutil_v0.2.0_linux_amd64.tar.gz":
			_, _ = writer.Write(archive)
		case "/v0.2.0/checksums.txt":
			_, _ = io.WriteString(writer, "not-a-checksum  gutil_v0.2.0_linux_amd64.tar.gz\n")
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	_, err := Service{ReleaseURL: server.URL + "/latest", DownloadBaseURL: server.URL, GOOS: "linux", GOARCH: "amd64"}.Update(context.Background(), "v0.1.0")
	if err == nil || !strings.Contains(err.Error(), "checksum verification failed") {
		t.Fatalf("err = %v", err)
	}
}

func updateServer(t *testing.T, version, archiveName string, archive []byte) *httptest.Server {
	t.Helper()
	digest := sha256.Sum256(archive)
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/latest":
			_, _ = io.WriteString(writer, `{"tag_name":"`+version+`"}`)
		case "/" + version + "/" + archiveName:
			_, _ = writer.Write(archive)
		case "/" + version + "/checksums.txt":
			_, _ = io.WriteString(writer, fmtChecksum(digest, archiveName))
		default:
			http.NotFound(writer, request)
		}
	}))
}

func fmtChecksum(digest [sha256.Size]byte, name string) string {
	return fmt.Sprintf("%x  %s\n", digest, name)
}

func tarArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return compressed.Bytes()
}

func zipArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var result bytes.Buffer
	writer := zip.NewWriter(&result)
	entry, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return result.Bytes()
}
