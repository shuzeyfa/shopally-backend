package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestIsMissingConfigFile(t *testing.T) {
	t.Run("viper not found", func(t *testing.T) {
		if !isMissingConfigFile(viper.ConfigFileNotFoundError{}) {
			t.Fatal("expected missing")
		}
	})

	t.Run("path not exist", func(t *testing.T) {
		err := &fs.PathError{Op: "open", Path: "x.yaml", Err: fs.ErrNotExist}
		if !isMissingConfigFile(err) {
			t.Fatal("expected missing")
		}
	})

	t.Run("path permission denied", func(t *testing.T) {
		err := &fs.PathError{Op: "open", Path: "x.yaml", Err: fs.ErrPermission}
		if isMissingConfigFile(err) {
			t.Fatal("expected not missing")
		}
	})

	t.Run("other error", func(t *testing.T) {
		if isMissingConfigFile(errors.New("broken yaml")) {
			t.Fatal("expected not missing")
		}
	})
}

func TestLoadConfigMalformedFileIsFatal(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "configs", "config.dev.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("server:\n  port: ["), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected load error for malformed config")
	}
}

func TestLoadConfigWithoutFileUsesEnvironment(t *testing.T) {
	t.Setenv("SERVER_PORT", "12345")
	t.Setenv("MONGO_DATABASE", "phase0")
	t.Setenv("REDIS_TLS", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, https://example.test")
	cfg, err := LoadConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != "12345" || cfg.Mongo.Database != "phase0" || !cfg.Redis.TLS {
		t.Fatal("environment overrides were not applied")
	}
	if got := cfg.CORSOrigins(); len(got) != 2 || got[1] != "https://example.test" {
		t.Fatalf("unexpected CORS origins: %v", got)
	}
}
