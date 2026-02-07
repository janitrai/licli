package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	in := Config{
		Auth: AuthConfig{
			LiAt:       "liat",
			JSessionID: "\"ajax:123\"",
		},
	}

	if err := Save(path, in); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("config perms = %o, want %o", got, 0o600)
		}
	}

	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if out.Auth.LiAt != in.Auth.LiAt {
		t.Fatalf("LiAt = %q, want %q", out.Auth.LiAt, in.Auth.LiAt)
	}
	if out.Auth.JSessionID != in.Auth.JSessionID {
		t.Fatalf("JSessionID = %q, want %q", out.Auth.JSessionID, in.Auth.JSessionID)
	}
	if out.Auth.UpdatedAt.IsZero() {
		t.Fatalf("UpdatedAt is zero")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() of missing file should not error, got: %v", err)
	}
	if cfg.Auth.LiAt != "" || cfg.Auth.JSessionID != "" {
		t.Errorf("Load() of missing file should return empty config, got: %+v", cfg)
	}
	if cfg.SearchQueryID != "" {
		t.Errorf("SearchQueryID should be empty, got: %q", cfg.SearchQueryID)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte("{invalid json!!!"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() of invalid JSON should return error")
	}
	if !strings.Contains(err.Error(), "parse config JSON") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "parse config JSON")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Empty file is invalid JSON
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	// Empty file → Unmarshal of empty bytes → error
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() of empty file should return error")
	}
}

func TestLoad_ValidEmptyObject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load({}) error: %v", err)
	}
	if cfg.Auth.LoggedIn() {
		t.Error("empty config should not be LoggedIn()")
	}
}

func TestLoad_WithSearchQueryID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := `{"auth":{"li_at":"tok","jsessionid":"sid"},"search_query_id":"custom.query.123"}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.SearchQueryID != "custom.query.123" {
		t.Errorf("SearchQueryID = %q, want %q", cfg.SearchQueryID, "custom.query.123")
	}
	if !cfg.Auth.LoggedIn() {
		t.Error("expected LoggedIn() to be true")
	}
}

func TestSave_CreatesParentDirectories(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "a", "b", "c", "config.json")

	cfg := Config{
		Auth: AuthConfig{LiAt: "tok", JSessionID: "sid"},
	}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() should create parent dirs, got: %v", err)
	}

	// Verify the file exists and can be loaded
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() after save error: %v", err)
	}
	if loaded.Auth.LiAt != "tok" {
		t.Errorf("LiAt = %q, want %q", loaded.Auth.LiAt, "tok")
	}
}

func TestSave_SetsUpdatedAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Config{Auth: AuthConfig{LiAt: "a", JSessionID: "b"}}

	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Auth.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set after Save()")
	}
}

func TestDefaultPath_WithEnvVar(t *testing.T) {
	customPath := "/tmp/li-test-custom/config.json"
	t.Setenv(EnvConfigPath, customPath)

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error: %v", err)
	}
	if got != customPath {
		t.Errorf("DefaultPath() = %q, want %q", got, customPath)
	}
}

func TestDefaultPath_WithoutEnvVar(t *testing.T) {
	t.Setenv(EnvConfigPath, "")

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error: %v", err)
	}
	// Should end with /li/config.json (or equivalent)
	if !strings.HasSuffix(got, filepath.Join("li", "config.json")) {
		t.Errorf("DefaultPath() = %q, expected to end with li/config.json", got)
	}
	// Should be an absolute path
	if !filepath.IsAbs(got) {
		t.Errorf("DefaultPath() = %q, expected absolute path", got)
	}
}

func TestAuthConfig_LoggedIn(t *testing.T) {
	tests := []struct {
		name string
		a    AuthConfig
		want bool
	}{
		{"both present", AuthConfig{LiAt: "a", JSessionID: "b"}, true},
		{"missing LiAt", AuthConfig{JSessionID: "b"}, false},
		{"missing JSessionID", AuthConfig{LiAt: "a"}, false},
		{"both empty", AuthConfig{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.LoggedIn(); got != tt.want {
				t.Errorf("LoggedIn() = %v, want %v", got, tt.want)
			}
		})
	}
}
