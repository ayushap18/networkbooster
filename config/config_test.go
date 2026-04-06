package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ayush18/networkbooster/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := config.Default()
	assert.Equal(t, 8, cfg.Connections)
	assert.Equal(t, "medium", cfg.Profile)
	assert.Equal(t, "download", cfg.Mode)
}

func TestConfig_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := config.Default()
	cfg.Connections = 16
	cfg.SelfHostedURL = "http://myserver.com"

	err := config.Save(cfg, path)
	require.NoError(t, err)

	loaded, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, 16, loaded.Connections)
	assert.Equal(t, "http://myserver.com", loaded.SelfHostedURL)
}

func TestConfig_LoadMissing_ReturnsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.yaml")
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, config.Default(), cfg)
}

func TestConfig_LoadFromEnvPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := config.Default()
	cfg.Connections = 32
	config.Save(cfg, path)

	t.Setenv("NETWORKBOOSTER_CONFIG", path)

	loaded, err := config.LoadDefault()
	require.NoError(t, err)
	assert.Equal(t, 32, loaded.Connections)

	os.Unsetenv("NETWORKBOOSTER_CONFIG")
}
