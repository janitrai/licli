package config

import (
	"os"
	"path/filepath"
	"runtime"
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
