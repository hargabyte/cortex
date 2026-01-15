package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Verify scan defaults
	if len(cfg.Scan.Languages) != 1 || cfg.Scan.Languages[0] != "go" {
		t.Errorf("expected default language [go], got %v", cfg.Scan.Languages)
	}

	if len(cfg.Scan.Exclude) != 4 {
		t.Errorf("expected 4 exclude patterns, got %d", len(cfg.Scan.Exclude))
	}

	// Verify metrics defaults
	if cfg.Metrics.PageRankDamping != 0.85 {
		t.Errorf("expected pagerank_damping 0.85, got %f", cfg.Metrics.PageRankDamping)
	}

	if cfg.Metrics.PageRankIterations != 100 {
		t.Errorf("expected pagerank_iterations 100, got %d", cfg.Metrics.PageRankIterations)
	}

	if cfg.Metrics.KeystoneThreshold != 0.30 {
		t.Errorf("expected keystone_threshold 0.30, got %f", cfg.Metrics.KeystoneThreshold)
	}

	if cfg.Metrics.BottleneckThreshold != 0.20 {
		t.Errorf("expected bottleneck_threshold 0.20, got %f", cfg.Metrics.BottleneckThreshold)
	}

	// Verify output defaults
	if cfg.Output.DefaultDensity != "medium" {
		t.Errorf("expected default_density medium, got %s", cfg.Output.DefaultDensity)
	}

	if cfg.Output.DefaultHops != 1 {
		t.Errorf("expected default_hops 1, got %d", cfg.Output.DefaultHops)
	}

	if cfg.Output.MaxTokens != 4000 {
		t.Errorf("expected max_tokens 4000, got %d", cfg.Output.MaxTokens)
	}
}

func TestIsValidDensity(t *testing.T) {
	tests := []struct {
		density string
		valid   bool
	}{
		{"sparse", true},
		{"medium", true},
		{"dense", true},
		{"invalid", false},
		{"", false},
		{"MEDIUM", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.density, func(t *testing.T) {
			result := IsValidDensity(tt.density)
			if result != tt.valid {
				t.Errorf("IsValidDensity(%q) = %v, want %v", tt.density, result, tt.valid)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid default config",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name: "invalid density",
			modify: func(c *Config) {
				c.Output.DefaultDensity = "invalid"
			},
			wantErr: true,
		},
		{
			name: "pagerank damping too high",
			modify: func(c *Config) {
				c.Metrics.PageRankDamping = 1.5
			},
			wantErr: true,
		},
		{
			name: "pagerank damping negative",
			modify: func(c *Config) {
				c.Metrics.PageRankDamping = -0.1
			},
			wantErr: true,
		},
		{
			name: "pagerank iterations zero",
			modify: func(c *Config) {
				c.Metrics.PageRankIterations = 0
			},
			wantErr: true,
		},
		{
			name: "keystone threshold too high",
			modify: func(c *Config) {
				c.Metrics.KeystoneThreshold = 1.5
			},
			wantErr: true,
		},
		{
			name: "bottleneck threshold negative",
			modify: func(c *Config) {
				c.Metrics.BottleneckThreshold = -0.1
			},
			wantErr: true,
		},
		{
			name: "negative hops",
			modify: func(c *Config) {
				c.Output.DefaultHops = -1
			},
			wantErr: true,
		},
		{
			name: "zero max_tokens",
			modify: func(c *Config) {
				c.Output.MaxTokens = 0
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := Validate(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	defaults := DefaultConfig()

	t.Run("empty loaded uses all defaults", func(t *testing.T) {
		loaded := &Config{}
		merged := Merge(loaded, defaults)

		if merged.Output.DefaultDensity != defaults.Output.DefaultDensity {
			t.Errorf("expected density %s, got %s", defaults.Output.DefaultDensity, merged.Output.DefaultDensity)
		}

		if merged.Metrics.PageRankDamping != defaults.Metrics.PageRankDamping {
			t.Errorf("expected damping %f, got %f", defaults.Metrics.PageRankDamping, merged.Metrics.PageRankDamping)
		}
	})

	t.Run("loaded values take precedence", func(t *testing.T) {
		loaded := &Config{
			Output: OutputConfig{
				DefaultDensity: "sparse",
				MaxTokens:      8000,
			},
			Metrics: MetricsConfig{
				PageRankDamping: 0.90,
			},
		}
		merged := Merge(loaded, defaults)

		if merged.Output.DefaultDensity != "sparse" {
			t.Errorf("expected density sparse, got %s", merged.Output.DefaultDensity)
		}

		if merged.Output.MaxTokens != 8000 {
			t.Errorf("expected max_tokens 8000, got %d", merged.Output.MaxTokens)
		}

		if merged.Metrics.PageRankDamping != 0.90 {
			t.Errorf("expected damping 0.90, got %f", merged.Metrics.PageRankDamping)
		}

		// Unset values should use defaults
		if merged.Output.DefaultHops != defaults.Output.DefaultHops {
			t.Errorf("expected default hops %d, got %d", defaults.Output.DefaultHops, merged.Output.DefaultHops)
		}
	})
}

func TestFindConfigDir(t *testing.T) {
	// Create a temp directory structure
	tmpDir, err := os.MkdirTemp("", "cx-config-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nested directories: tmpDir/project/subdir
	projectDir := filepath.Join(tmpDir, "project")
	subDir := filepath.Join(projectDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	t.Run("no config dir returns error", func(t *testing.T) {
		_, err := FindConfigDir(subDir)
		if err == nil {
			t.Error("expected error when no .cx directory exists")
		}
	})

	// Create .cx directory in project root
	configDir := filepath.Join(projectDir, ConfigDirName)
	if err := os.Mkdir(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	t.Run("finds config dir in current directory", func(t *testing.T) {
		found, err := FindConfigDir(projectDir)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if found != configDir {
			t.Errorf("expected %s, got %s", configDir, found)
		}
	})

	t.Run("finds config dir in parent directory", func(t *testing.T) {
		found, err := FindConfigDir(subDir)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if found != configDir {
			t.Errorf("expected %s, got %s", configDir, found)
		}
	})
}

func TestEnsureConfigDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-config-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("creates config directory", func(t *testing.T) {
		dir, err := EnsureConfigDir(tmpDir)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedDir := filepath.Join(tmpDir, ConfigDirName)
		if dir != expectedDir {
			t.Errorf("expected %s, got %s", expectedDir, dir)
		}

		// Verify directory exists
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("config directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected directory, got file")
		}
	})

	t.Run("returns existing directory", func(t *testing.T) {
		// Call again, should return same directory without error
		dir, err := EnsureConfigDir(tmpDir)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedDir := filepath.Join(tmpDir, ConfigDirName)
		if dir != expectedDir {
			t.Errorf("expected %s, got %s", expectedDir, dir)
		}
	})
}

func TestLoadFromPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-config-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("loads valid config file", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "config.yaml")
		content := `
scan:
  languages: [go, python]
  exclude:
    - vendor/**
output:
  default_density: dense
  max_tokens: 8000
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := LoadFromPath(configPath)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Check loaded values
		if len(cfg.Scan.Languages) != 2 {
			t.Errorf("expected 2 languages, got %d", len(cfg.Scan.Languages))
		}
		if cfg.Output.DefaultDensity != "dense" {
			t.Errorf("expected density dense, got %s", cfg.Output.DefaultDensity)
		}
		if cfg.Output.MaxTokens != 8000 {
			t.Errorf("expected max_tokens 8000, got %d", cfg.Output.MaxTokens)
		}

		// Check defaults were applied for missing values
		if cfg.Metrics.PageRankDamping != 0.85 {
			t.Errorf("expected default damping 0.85, got %f", cfg.Metrics.PageRankDamping)
		}
		if cfg.Output.DefaultHops != 1 {
			t.Errorf("expected default hops 1, got %d", cfg.Output.DefaultHops)
		}
	})

	t.Run("returns defaults for non-existent file", func(t *testing.T) {
		cfg, err := LoadFromPath(filepath.Join(tmpDir, "nonexistent.yaml"))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		defaults := DefaultConfig()
		if cfg.Output.DefaultDensity != defaults.Output.DefaultDensity {
			t.Errorf("expected default density, got %s", cfg.Output.DefaultDensity)
		}
	})

	t.Run("returns error for invalid YAML", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "invalid.yaml")
		if err := os.WriteFile(configPath, []byte("invalid: yaml: content"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := LoadFromPath(configPath)
		if err == nil {
			t.Error("expected error for invalid YAML")
		}
	})

	t.Run("returns error for invalid config values", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "bad-values.yaml")
		content := `
output:
  default_density: invalid_density
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := LoadFromPath(configPath)
		if err == nil {
			t.Error("expected error for invalid density")
		}
	})
}

func TestLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-config-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("returns defaults when no config dir exists", func(t *testing.T) {
		cfg, err := Load(tmpDir)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		defaults := DefaultConfig()
		if cfg.Output.DefaultDensity != defaults.Output.DefaultDensity {
			t.Errorf("expected default config")
		}
	})

	t.Run("loads config from .cx directory", func(t *testing.T) {
		// Create .cx directory and config file
		configDir := filepath.Join(tmpDir, ConfigDirName)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatal(err)
		}

		content := `
output:
  default_density: sparse
`
		configPath := filepath.Join(configDir, ConfigFileName)
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(tmpDir)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if cfg.Output.DefaultDensity != "sparse" {
			t.Errorf("expected density sparse, got %s", cfg.Output.DefaultDensity)
		}
	})
}

func TestSaveDefault(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-config-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("creates default config file", func(t *testing.T) {
		configPath, err := SaveDefault(tmpDir)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedPath := filepath.Join(tmpDir, ConfigDirName, ConfigFileName)
		if configPath != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, configPath)
		}

		// Verify file exists and is valid
		cfg, err := LoadFromPath(configPath)
		if err != nil {
			t.Errorf("failed to load saved config: %v", err)
		}

		defaults := DefaultConfig()
		if cfg.Output.DefaultDensity != defaults.Output.DefaultDensity {
			t.Errorf("saved config doesn't match defaults")
		}
	})

	t.Run("fails if config already exists", func(t *testing.T) {
		// Config was created in previous test
		_, err := SaveDefault(tmpDir)
		if err == nil {
			t.Error("expected error when config already exists")
		}
	})
}
