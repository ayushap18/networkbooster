package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Mode          string `yaml:"mode"`
	Profile       string `yaml:"profile"`
	Connections   int    `yaml:"connections"`
	SelfHostedURL string `yaml:"self_hosted_url,omitempty"`
}

func Default() Config {
	return Config{
		Mode:        "download",
		Profile:     "medium",
		Connections: 8,
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return Config{}, err
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(cfg Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func LoadDefault() (Config, error) {
	path := os.Getenv("NETWORKBOOSTER_CONFIG")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Default(), nil
		}
		path = filepath.Join(home, ".networkbooster", "config.yaml")
	}
	return Load(path)
}
