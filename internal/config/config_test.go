package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	if cfg.EmbedModel != "embed-v4.0" {
		t.Errorf("expected embed model 'embed-v4.0', got '%s'", cfg.EmbedModel)
	}

	if cfg.RerankModel != "rerank-v3.5" {
		t.Errorf("expected rerank model 'rerank-v3.5', got '%s'", cfg.RerankModel)
	}

	if cfg.EmbedDim != 1024 {
		t.Errorf("expected embed dim 1024, got %d", cfg.EmbedDim)
	}
}

func TestConfigSaveLoad(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "obsvec-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config file path
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := &Config{
		CohereAPIKey: "test-api-key",
		ObsidianDir:  "/path/to/vault",
		EmbedModel:   "embed-v4.0",
		RerankModel:  "rerank-v3.5",
		EmbedDim:     1024,
	}

	// Save config
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Read it back
	readData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	var loadedCfg Config
	if err := json.Unmarshal(readData, &loadedCfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if loadedCfg.CohereAPIKey != cfg.CohereAPIKey {
		t.Errorf("expected API key '%s', got '%s'", cfg.CohereAPIKey, loadedCfg.CohereAPIKey)
	}

	if loadedCfg.ObsidianDir != cfg.ObsidianDir {
		t.Errorf("expected dir '%s', got '%s'", cfg.ObsidianDir, loadedCfg.ObsidianDir)
	}
}

func TestConfigDefaultsApplied(t *testing.T) {
	// Test that zero values get defaults applied
	cfg := &Config{
		CohereAPIKey: "key",
		ObsidianDir:  "/vault",
		// Leave other fields as zero values
	}

	cfg.ApplyDefaults()

	if cfg.EmbedModel != "embed-v4.0" {
		t.Errorf("expected default embed model, got '%s'", cfg.EmbedModel)
	}

	if cfg.RerankModel != "rerank-v3.5" {
		t.Errorf("expected default rerank model, got '%s'", cfg.RerankModel)
	}

	if cfg.EmbedDim != 1024 {
		t.Errorf("expected default embed dim 1024, got %d", cfg.EmbedDim)
	}
}
