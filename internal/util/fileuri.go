package util

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

func FileURI(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	slashified := filepath.ToSlash(abs)
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(slashified, "/") {
			slashified = "/" + slashified
		}
	}

	u := url.URL{
		Scheme: "file",
		Path:   slashified,
	}
	return u.String(), nil
}
