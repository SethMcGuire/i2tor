package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type InstallRecord struct {
	Source              string    `json:"source"`
	Version             string    `json:"version,omitempty"`
	ArtifactURL         string    `json:"artifact_url,omitempty"`
	ArtifactPath        string    `json:"artifact_path,omitempty"`
	ChecksumSHA256      string    `json:"checksum_sha256,omitempty"`
	ChecksumVerified    bool      `json:"checksum_verified"`
	ChecksumVerifiedAt  time.Time `json:"checksum_verified_at,omitempty"`
	SignatureVerified   bool      `json:"signature_verified"`
	SignatureVerifiedAt time.Time `json:"signature_verified_at,omitempty"`
	InstalledAt         time.Time `json:"installed_at,omitempty"`
}

type ManagedProcessRecord struct {
	PID       int       `json:"pid,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
	Command   string    `json:"command,omitempty"`
	Args      []string  `json:"args,omitempty"`
	Owns      bool      `json:"owns"`
}

type Manifest struct {
	AppVersion                  string               `json:"app_version"`
	OS                          string               `json:"os"`
	Arch                        string               `json:"arch"`
	TorBrowser                  InstallRecord        `json:"tor_browser"`
	I2P                         InstallRecord        `json:"i2p"`
	Java                        InstallRecord        `json:"java"`
	LastSuccessfulLaunchAt      time.Time            `json:"last_successful_launch_at,omitempty"`
	LauncherManagedI2P          ManagedProcessRecord `json:"launcher_managed_i2p"`
	GeneratedPACPath            string               `json:"generated_pac_path,omitempty"`
	DedicatedProfilePath        string               `json:"dedicated_profile_path,omitempty"`
	LastManifestUpdateTimestamp time.Time            `json:"last_manifest_update_timestamp,omitempty"`
}

func DefaultManifest() Manifest {
	return Manifest{}
}

func LoadManifest(ctx context.Context, path string) (Manifest, error) {
	_ = ctx
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultManifest(), nil
		}
		return Manifest{}, fmt.Errorf("read manifest %q: %w", path, err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest %q: %w", path, err)
	}
	return manifest, nil
}

func SaveManifest(ctx context.Context, path string, manifest Manifest) error {
	_ = ctx
	manifest.LastManifestUpdateTimestamp = time.Now().UTC()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "manifest-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp manifest: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp manifest: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp manifest: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace manifest: %w", err)
	}
	return nil
}
