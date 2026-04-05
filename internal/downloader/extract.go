package downloader

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ulikunitz/xz"
)

func Extract(ctx context.Context, archivePath, destination string) error {
	_ = ctx
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("create extract destination %q: %w", destination, err)
	}
	switch {
	case strings.HasSuffix(archivePath, ".zip"):
		return extractZip(archivePath, destination)
	case strings.HasSuffix(archivePath, ".deb"):
		return extractDeb(archivePath, destination)
	case strings.HasSuffix(archivePath, ".tar.gz"), strings.HasSuffix(archivePath, ".tgz"):
		return extractTar(archivePath, destination, "gz")
	case strings.HasSuffix(archivePath, ".tar.xz"):
		return extractTar(archivePath, destination, "xz")
	default:
		return fmt.Errorf("unsupported archive format for %q", archivePath)
	}
}

func extractDeb(archivePath, destination string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open deb archive %q: %w", archivePath, err)
	}
	defer f.Close()

	arReader, err := newARReader(f)
	if err != nil {
		return fmt.Errorf("open deb ar reader %q: %w", archivePath, err)
	}
	for {
		hdr, err := arReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read deb member from %q: %w", archivePath, err)
		}

		name := strings.TrimSpace(hdr.Name)
		if !strings.HasPrefix(name, "data.tar") {
			continue
		}

		switch {
		case strings.HasSuffix(name, ".xz"):
			return extractTarStream(io.LimitReader(arReader, hdr.Size), destination, "xz", archivePath)
		case strings.HasSuffix(name, ".gz"):
			return extractTarStream(io.LimitReader(arReader, hdr.Size), destination, "gz", archivePath)
		default:
			return fmt.Errorf("unsupported deb payload compression %q", name)
		}
	}

	return fmt.Errorf("deb archive %q did not contain data.tar payload", archivePath)
}

func extractZip(archivePath, destination string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip archive %q: %w", archivePath, err)
	}
	defer r.Close()

	for _, f := range r.File {
		target, err := safeJoin(destination, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create directory %q: %w", target, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create directory %q: %w", filepath.Dir(target), err)
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open zip member %q: %w", f.Name, err)
		}
		if err := writeExtractedFile(target, rc, f.Mode()); err != nil {
			rc.Close()
			return err
		}
		rc.Close()
	}
	return nil
}

func extractTar(archivePath, destination, compression string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive %q: %w", archivePath, err)
	}
	defer f.Close()

	return extractTarStream(f, destination, compression, archivePath)
}

func extractTarStream(source io.Reader, destination, compression, archivePath string) error {
	var reader io.Reader = source

	switch compression {
	case "gz":
		gzr, err := gzip.NewReader(source)
		if err != nil {
			return fmt.Errorf("open gzip stream %q: %w", archivePath, err)
		}
		defer gzr.Close()
		reader = gzr
	case "xz":
		xzr, err := xz.NewReader(source)
		if err != nil {
			return fmt.Errorf("open xz stream %q: %w", archivePath, err)
		}
		reader = xzr
	}

	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read archive entry from %q: %w", archivePath, err)
		}

		target, err := safeJoin(destination, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create directory %q: %w", target, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create directory %q: %w", filepath.Dir(target), err)
			}
			if err := writeExtractedFile(target, io.NopCloser(tr), os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		default:
		}
	}
}

type arHeader struct {
	Name string
	Size int64
}

type arReader struct {
	r       io.Reader
	pending int64
	pad     int64
}

func newARReader(r io.Reader) (*arReader, error) {
	magic := make([]byte, 8)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, err
	}
	if string(magic) != "!<arch>\n" {
		return nil, fmt.Errorf("invalid ar magic")
	}
	return &arReader{r: r}, nil
}

func (r *arReader) Next() (arHeader, error) {
	if err := r.discardPending(); err != nil {
		return arHeader{}, err
	}

	hdrBytes := make([]byte, 60)
	if _, err := io.ReadFull(r.r, hdrBytes); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return arHeader{}, io.EOF
		}
		return arHeader{}, err
	}
	if string(hdrBytes[58:60]) != "`\n" {
		return arHeader{}, fmt.Errorf("invalid ar header trailer")
	}

	name := strings.TrimSpace(string(hdrBytes[0:16]))
	name = strings.TrimSuffix(name, "/")
	sizeText := strings.TrimSpace(string(hdrBytes[48:58]))
	size, err := strconv.ParseInt(sizeText, 10, 64)
	if err != nil {
		return arHeader{}, fmt.Errorf("parse ar member size %q: %w", sizeText, err)
	}
	r.pending = size
	r.pad = size % 2
	return arHeader{Name: name, Size: size}, nil
}

func (r *arReader) Read(p []byte) (int, error) {
	if r.pending == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.pending {
		p = p[:r.pending]
	}
	n, err := r.r.Read(p)
	r.pending -= int64(n)
	if r.pending == 0 && r.pad > 0 {
		if _, padErr := io.CopyN(io.Discard, r.r, r.pad); padErr != nil {
			return n, padErr
		}
		r.pad = 0
	}
	return n, err
}

func (r *arReader) discardPending() error {
	if r.pending > 0 {
		if _, err := io.CopyN(io.Discard, r, r.pending); err != nil {
			return err
		}
	}
	if r.pad > 0 {
		if _, err := io.CopyN(io.Discard, r.r, r.pad); err != nil {
			return err
		}
		r.pad = 0
	}
	return nil
}

func writeExtractedFile(target string, content io.ReadCloser, mode os.FileMode) error {
	defer content.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm()|0o600)
	if err != nil {
		return fmt.Errorf("create extracted file %q: %w", target, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, content); err != nil {
		return fmt.Errorf("write extracted file %q: %w", target, err)
	}
	return nil
}

func safeJoin(root, name string) (string, error) {
	clean := filepath.Clean(name)
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("archive entry %q uses an absolute path", name)
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry %q escapes destination", name)
	}
	target := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", fmt.Errorf("validate archive entry %q: %w", name, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry %q escapes destination", name)
	}
	return target, nil
}
