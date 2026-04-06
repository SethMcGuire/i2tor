package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ReuseExistingTorBrowser bool   `json:"reuse_existing_tor_browser"`
	ReuseExistingI2P        bool   `json:"reuse_existing_i2p"`
	AutoCheckUpdates        bool   `json:"auto_check_updates"`
	AutoStartOnLaunch       bool   `json:"auto_start_on_launch"`
	AllowLocalhostAccess    bool   `json:"allow_localhost_access"`
	KeepI2PRunning          bool   `json:"keep_i2p_running"`
	DataDir                 string `json:"data_dir"`
	LogLevel                string `json:"log_level"`
}

func Default() Config {
	return Config{
		ReuseExistingTorBrowser: true,
		ReuseExistingI2P:        true,
		AutoCheckUpdates:        true,
		AutoStartOnLaunch:       false,
		KeepI2PRunning:          false,
		LogLevel:                "info",
	}
}

func Load(ctx context.Context, path string) (Config, error) {
	_ = ctx
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	return cfg, nil
}

func Save(ctx context.Context, path string, cfg Config) error {
	_ = ctx
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
