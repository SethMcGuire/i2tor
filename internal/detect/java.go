package detect

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

func DetectExistingJava(ctx context.Context, minMajor int) (InstallCandidate, error) {
	pathExec, err := exec.LookPath("java")
	if err != nil {
		return InstallCandidate{}, fmt.Errorf("java not found in PATH: %w", err)
	}
	version, err := JavaMajorVersion(ctx, pathExec)
	if err != nil {
		return InstallCandidate{}, err
	}
	if version < minMajor {
		return InstallCandidate{}, fmt.Errorf("java at %s is version %d, need %d+", pathExec, version, minMajor)
	}
	return InstallCandidate{
		Name:          "java",
		Source:        "existing",
		RootPath:      pathExec,
		Executable:    pathExec,
		Version:       strconv.Itoa(version),
		ReadOnly:      true,
		DetectionHint: "PATH lookup",
	}, nil
}

func JavaMajorVersion(ctx context.Context, javaPath string) (int, error) {
	cmd := exec.CommandContext(ctx, javaPath, "-version")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("run %s -version: %w", javaPath, err)
	}
	re := regexp.MustCompile(`version "([0-9]+)`)
	m := re.FindStringSubmatch(output.String())
	if len(m) != 2 {
		return 0, fmt.Errorf("parse java version output from %s", javaPath)
	}
	v, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, fmt.Errorf("parse java major version: %w", err)
	}
	return v, nil
}
