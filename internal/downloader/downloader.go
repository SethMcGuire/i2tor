package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type ArtifactMetadata struct {
	Name           string
	Version        string
	ArtifactURL    string
	ChecksumURL    string
	ChecksumSHA256 string
	SignatureURL   string
	SignatureKey   string
	FileName       string
}

func Download(ctx context.Context, destDir string, meta ArtifactMetadata) (string, error) {
	return DownloadURL(ctx, destDir, meta.ArtifactURL, meta.FileName)
}

func DownloadURL(ctx context.Context, destDir, sourceURL, fileName string) (string, error) {
	if sourceURL == "" {
		return "", fmt.Errorf("missing artifact URL for %s", fileName)
	}
	if fileName == "" {
		return "", fmt.Errorf("missing destination file name for %s", sourceURL)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("create download directory %q: %w", destDir, err)
	}

	target := filepath.Join(destDir, fileName)
	if _, err := os.Stat(target); err == nil {
		return target, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request for %s: %w", sourceURL, err)
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", sourceURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: unexpected status %s", sourceURL, resp.Status)
	}

	tmp, err := os.CreateTemp(destDir, "download-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temp download: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write download %s: %w", sourceURL, err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close download %s: %w", sourceURL, err)
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return "", fmt.Errorf("move download into place %q: %w", target, err)
	}
	return target, nil
}

func DownloadOptional(ctx context.Context, destDir, sourceURL, fileName string) (string, error) {
	if sourceURL == "" || fileName == "" {
		return "", nil
	}
	return DownloadURL(ctx, destDir, sourceURL, fileName)
}
