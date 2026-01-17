package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	CohereAPIKey string `json:"cohere_api_key"`
	ObsidianDir  string `json:"obsidian_dir"`
	EmbedModel   string `json:"embed_model"`
	RerankModel  string `json:"rerank_model"`
	EmbedDim     int    `json:"embed_dim"`
}

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "obsvec"), nil
}

func configPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func DBPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "obsvec.db"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaultConfig(), nil
	}
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.EmbedModel == "" {
		cfg.EmbedModel = "embed-v4.0"
	}
	if cfg.RerankModel == "" {
		cfg.RerankModel = "rerank-v3.5"
	}
	if cfg.EmbedDim == 0 {
		cfg.EmbedDim = 1024
	}

	return &cfg, nil
}

func (c *Config) Save() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}

func defaultConfig() *Config {
	return &Config{
		EmbedModel:  "embed-v4.0",
		RerankModel: "rerank-v3.5",
		EmbedDim:    1024,
	}
}
