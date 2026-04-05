package downloader

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"i2tor/internal/verifier"
)

func VerifySHA256(ctx context.Context, artifactPath string, meta ArtifactMetadata) error {
	expected, err := resolveExpectedChecksum(ctx, meta)
	if err != nil {
		return err
	}
	if expected == "" {
		return fmt.Errorf("no checksum available for %s", meta.Name)
	}
	actual, err := fileSHA256(artifactPath)
	if err != nil {
		return fmt.Errorf("calculate checksum for %q: %w", artifactPath, err)
	}
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch for %q: expected %s, got %s", artifactPath, expected, actual)
	}
	return nil
}

func VerifyDetachedSignature(ctx context.Context, downloadsDir, artifactPath string, meta ArtifactMetadata) error {
	if meta.SignatureURL == "" {
		return nil
	}
	if meta.SignatureKey == "" {
		return fmt.Errorf("missing signature key for %s", meta.Name)
	}
	signaturePath, err := DownloadURL(ctx, downloadsDir, meta.SignatureURL, signatureFileName(meta))
	if err != nil {
		return fmt.Errorf("download signature for %s from %s: %w", meta.Name, meta.SignatureURL, err)
	}
	if err := verifier.VerifyDetachedSignature(ctx, meta.SignatureKey, artifactPath, signaturePath); err != nil {
		return err
	}
	return nil
}

func VerifySignedChecksum(ctx context.Context, downloadsDir string, meta ArtifactMetadata) error {
	if meta.ChecksumURL == "" || meta.SignatureURL == "" {
		return nil
	}
	if meta.SignatureKey == "" {
		return fmt.Errorf("missing signature key for checksum verification of %s", meta.Name)
	}
	checksumPath, err := DownloadURL(ctx, downloadsDir, meta.ChecksumURL, filepath.Base(meta.ChecksumURL))
	if err != nil {
		return fmt.Errorf("download checksum file for %s from %s: %w", meta.Name, meta.ChecksumURL, err)
	}
	signaturePath, err := DownloadURL(ctx, downloadsDir, meta.SignatureURL, signatureFileName(meta))
	if err != nil {
		return fmt.Errorf("download checksum signature for %s from %s: %w", meta.Name, meta.SignatureURL, err)
	}
	if err := verifier.VerifyDetachedSignature(ctx, meta.SignatureKey, checksumPath, signaturePath); err != nil {
		return err
	}
	return nil
}

func resolveExpectedChecksum(ctx context.Context, meta ArtifactMetadata) (string, error) {
	if meta.ChecksumSHA256 != "" {
		return meta.ChecksumSHA256, nil
	}
	if meta.ChecksumURL == "" {
		return "", nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, meta.ChecksumURL, nil)
	if err != nil {
		return "", fmt.Errorf("create checksum request for %s: %w", meta.ChecksumURL, err)
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download checksum file %s: %w", meta.ChecksumURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download checksum file %s: unexpected status %s", meta.ChecksumURL, resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, meta.FileName) {
			fields := strings.Fields(line)
			if len(fields) >= 1 {
				return fields[0], nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan checksum file %s: %w", meta.ChecksumURL, err)
	}
	return "", fmt.Errorf("checksum entry for %s not found in %s", meta.FileName, meta.ChecksumURL)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func signatureFileName(meta ArtifactMetadata) string {
	if meta.SignatureURL == "" {
		return ""
	}
	return filepath.Base(meta.SignatureURL)
}
