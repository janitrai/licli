package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// EnvConfigPath overrides the default config path.
	EnvConfigPath = "LI_CONFIG_PATH"

	defaultDirName  = "li"
	defaultFileName = "config.json"
)

type Config struct {
	Auth AuthConfig `json:"auth"`
}

type AuthConfig struct {
	LiAt       string    `json:"li_at"`
	JSessionID string    `json:"jsessionid"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

func (a AuthConfig) LoggedIn() bool {
	return a.LiAt != "" && a.JSessionID != ""
}

func DefaultPath() (string, error) {
	if p := os.Getenv(EnvConfigPath); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(dir, defaultDirName, defaultFileName), nil
}

func Load(path string) (Config, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Config{}, err
		}
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config JSON: %w", err)
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	cfg.Auth.UpdatedAt = time.Now().UTC()

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	b = append(b, '\n')

	tmp, err := os.CreateTemp(dir, defaultFileName+".tmp.*")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	// Best-effort. On Windows this might be ignored, but on Unix we want 0600.
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp config: %w", err)
	}

	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
