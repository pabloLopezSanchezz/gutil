package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const repository = "pabloLopezSanchezz/gutil"

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Result struct {
	Version   string
	UpToDate  bool
	Scheduled bool
}

type Service struct {
	Client              httpDoer
	ReleaseURL          string
	DownloadBaseURL     string
	ExecutablePath      func() (string, error)
	GOOS                string
	GOARCH              string
	Replace             func(string, string) error
	StartWindowsReplace func(string, string) error
}

func (s Service) Update(ctx context.Context, currentVersion string) (Result, error) {
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	releaseURL := s.ReleaseURL
	if releaseURL == "" {
		releaseURL = "https://api.github.com/repos/" + repository + "/releases/latest"
	}
	version, err := latestVersion(ctx, client, releaseURL)
	if err != nil {
		return Result{}, err
	}
	if currentVersion == version {
		return Result{Version: version, UpToDate: true}, nil
	}

	goos, goarch := s.GOOS, s.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	archive, binary, err := assetNames(version, goos, goarch)
	if err != nil {
		return Result{}, err
	}
	baseURL := s.DownloadBaseURL
	if baseURL == "" {
		baseURL = "https://github.com/" + repository + "/releases/download/"
	}
	baseURL = strings.TrimRight(baseURL, "/") + "/" + version + "/"
	archiveData, err := download(ctx, client, baseURL+archive)
	if err != nil {
		return Result{}, err
	}
	checksums, err := download(ctx, client, baseURL+"checksums.txt")
	if err != nil {
		return Result{}, err
	}
	if err := verifyChecksum(archive, archiveData, checksums); err != nil {
		return Result{}, err
	}
	executableData, err := extractBinary(archive, binary, archiveData)
	if err != nil {
		return Result{}, err
	}

	executablePath := s.ExecutablePath
	if executablePath == nil {
		executablePath = os.Executable
	}
	destination, err := executablePath()
	if err != nil {
		return Result{}, fmt.Errorf("locate the current gUtil executable: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(destination); resolveErr == nil {
		destination = resolved
	}
	temporary, err := writeReplacement(destination, executableData)
	if err != nil {
		return Result{}, err
	}

	if goos == "windows" {
		start := s.StartWindowsReplace
		if start == nil {
			start = startWindowsReplace
		}
		if err := start(temporary, destination); err != nil {
			_ = os.Remove(temporary)
			return Result{}, err
		}
		return Result{Version: version, Scheduled: true}, nil
	}
	replace := s.Replace
	if replace == nil {
		replace = os.Rename
	}
	if err := replace(temporary, destination); err != nil {
		_ = os.Remove(temporary)
		return Result{}, fmt.Errorf("replace the current gUtil executable: %w", err)
	}
	return Result{Version: version}, nil
}

func latestVersion(ctx context.Context, client httpDoer, url string) (string, error) {
	body, err := download(ctx, client, url)
	if err != nil {
		return "", fmt.Errorf("check the latest gUtil release: %w", err)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return "", fmt.Errorf("read the latest gUtil release: %w", err)
	}
	if release.TagName == "" {
		return "", fmt.Errorf("the latest gUtil release has no tag name")
	}
	return release.TagName, nil
}

func assetNames(version, goos, goarch string) (string, string, error) {
	if goos != "darwin" && goos != "linux" && goos != "windows" {
		return "", "", fmt.Errorf("unsupported operating system %q", goos)
	}
	if goarch != "amd64" && goarch != "arm64" {
		return "", "", fmt.Errorf("unsupported architecture %q", goarch)
	}
	binary := "gutil"
	if goos == "windows" {
		return fmt.Sprintf("gutil_%s_windows_%s.zip", version, goarch), binary + ".exe", nil
	}
	return fmt.Sprintf("gutil_%s_%s_%s.tar.gz", version, goos, goarch), binary, nil
}

func download(ctx context.Context, client httpDoer, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("download %s: received HTTP %d", url, response.StatusCode)
	}
	return io.ReadAll(response.Body)
}

func verifyChecksum(archive string, data, checksums []byte) error {
	want := ""
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") == archive {
			want = strings.ToLower(fields[0])
			break
		}
	}
	if want == "" {
		return fmt.Errorf("no checksum was published for %s", archive)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(data))
	if want != got {
		return fmt.Errorf("checksum verification failed for %s", archive)
	}
	return nil
}

func extractBinary(archive, binary string, data []byte) ([]byte, error) {
	if strings.HasSuffix(archive, ".zip") {
		reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("read update archive: %w", err)
		}
		for _, file := range reader.File {
			if filepath.Base(file.Name) != binary {
				continue
			}
			content, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer content.Close()
			return io.ReadAll(content)
		}
	} else {
		gzipReader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("read update archive: %w", err)
		}
		defer gzipReader.Close()
		tarReader := tar.NewReader(gzipReader)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("read update archive: %w", err)
			}
			if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == binary {
				return io.ReadAll(tarReader)
			}
		}
	}
	return nil, fmt.Errorf("update archive does not contain %s", binary)
}

func writeReplacement(destination string, data []byte) (string, error) {
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".gutil-update-*")
	if err != nil {
		return "", fmt.Errorf("create a temporary update file next to the current executable: %w", err)
	}
	path := temporary.Name()
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write the update: %w", err)
	}
	if err := temporary.Chmod(0o755); err != nil {
		temporary.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("make the update executable: %w", err)
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func startWindowsReplace(source, destination string) error {
	command := fmt.Sprintf(`ping 127.0.0.1 -n 3 > nul & move /Y "%s" "%s" > nul`, source, destination)
	if err := exec.Command("cmd.exe", "/d", "/s", "/c", command).Start(); err != nil {
		return fmt.Errorf("schedule the Windows executable replacement: %w", err)
	}
	return nil
}
