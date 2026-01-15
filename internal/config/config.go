package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigFileName is the name of the cx configuration file
const ConfigFileName = "config.yaml"

// ConfigDirName is the name of the cx configuration directory
const ConfigDirName = ".cx"

// Config holds all cx configuration
type Config struct {
	Scan    ScanConfig    `yaml:"scan"`
	Metrics MetricsConfig `yaml:"metrics"`
	Output  OutputConfig  `yaml:"output"`
	Guard   GuardConfig   `yaml:"guard"`
}

// GuardConfig holds configuration for the pre-commit guard
type GuardConfig struct {
	FailOnCoverageRegression bool    `yaml:"fail_on_coverage_regression"`
	MinCoverageForKeystones  float64 `yaml:"min_coverage_for_keystones"`
	FailOnWarnings           bool    `yaml:"fail_on_warnings"`
}

// ScanConfig holds configuration for code scanning
type ScanConfig struct {
	Languages []string `yaml:"languages"`
	Exclude   []string `yaml:"exclude"`
}

// MetricsConfig holds configuration for graph metrics computation
type MetricsConfig struct {
	PageRankDamping     float64 `yaml:"pagerank_damping"`
	PageRankIterations  int     `yaml:"pagerank_iterations"`
	KeystoneThreshold   float64 `yaml:"keystone_threshold"`
	BottleneckThreshold float64 `yaml:"bottleneck_threshold"`
}

// OutputConfig holds configuration for output formatting
type OutputConfig struct {
	DefaultDensity string `yaml:"default_density"`
	DefaultHops    int    `yaml:"default_hops"`
	MaxTokens      int    `yaml:"max_tokens"`
}

// ErrConfigNotFound is returned when no config file can be found
var ErrConfigNotFound = errors.New("config file not found")

// ErrInvalidConfig is returned when config validation fails
var ErrInvalidConfig = errors.New("invalid configuration")

// Load reads config from .cx/config.yaml, falling back to defaults.
// It searches for the config directory starting from workDir and walking up
// the directory tree. If no config is found, returns defaults.
func Load(workDir string) (*Config, error) {
	configDir, err := FindConfigDir(workDir)
	if err != nil {
		// No config dir found, return defaults
		return DefaultConfig(), nil
	}

	configPath := filepath.Join(configDir, ConfigFileName)
	return LoadFromPath(configPath)
}

// LoadFromPath reads config from a specific path.
// Merges loaded config with defaults and validates the result.
func LoadFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	loaded := &Config{}
	if err := yaml.Unmarshal(data, loaded); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Merge with defaults
	merged := Merge(loaded, DefaultConfig())

	// Validate the merged config
	if err := Validate(merged); err != nil {
		return nil, err
	}

	return merged, nil
}

// FindConfigDir locates the .cx directory by walking up from startDir.
// Returns the path to the .cx directory if found.
func FindConfigDir(startDir string) (string, error) {
	// Get absolute path
	absDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	currentDir := absDir
	for {
		configDir := filepath.Join(currentDir, ConfigDirName)
		info, err := os.Stat(configDir)
		if err == nil && info.IsDir() {
			return configDir, nil
		}

		// Move to parent directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached root, config not found
			return "", ErrConfigNotFound
		}
		currentDir = parentDir
	}
}

// EnsureConfigDir creates the .cx directory if it doesn't exist.
// Returns the path to the .cx directory.
func EnsureConfigDir(workDir string) (string, error) {
	// Get absolute path
	absDir, err := filepath.Abs(workDir)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	configDir := filepath.Join(absDir, ConfigDirName)

	// Check if it already exists
	info, err := os.Stat(configDir)
	if err == nil {
		if info.IsDir() {
			return configDir, nil
		}
		return "", fmt.Errorf("%s exists but is not a directory", configDir)
	}

	// Create the directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("creating config directory: %w", err)
	}

	return configDir, nil
}

// Validate checks that config values are valid.
// Returns an error if validation fails.
func Validate(cfg *Config) error {
	// Validate density
	if !IsValidDensity(cfg.Output.DefaultDensity) {
		return fmt.Errorf("%w: default_density must be one of %v, got %q",
			ErrInvalidConfig, ValidDensities, cfg.Output.DefaultDensity)
	}

	// Validate PageRank damping (should be between 0 and 1)
	if cfg.Metrics.PageRankDamping < 0 || cfg.Metrics.PageRankDamping > 1 {
		return fmt.Errorf("%w: pagerank_damping must be between 0 and 1, got %f",
			ErrInvalidConfig, cfg.Metrics.PageRankDamping)
	}

	// Validate PageRank iterations (should be positive)
	if cfg.Metrics.PageRankIterations <= 0 {
		return fmt.Errorf("%w: pagerank_iterations must be positive, got %d",
			ErrInvalidConfig, cfg.Metrics.PageRankIterations)
	}

	// Validate thresholds (should be between 0 and 1)
	if cfg.Metrics.KeystoneThreshold < 0 || cfg.Metrics.KeystoneThreshold > 1 {
		return fmt.Errorf("%w: keystone_threshold must be between 0 and 1, got %f",
			ErrInvalidConfig, cfg.Metrics.KeystoneThreshold)
	}

	if cfg.Metrics.BottleneckThreshold < 0 || cfg.Metrics.BottleneckThreshold > 1 {
		return fmt.Errorf("%w: bottleneck_threshold must be between 0 and 1, got %f",
			ErrInvalidConfig, cfg.Metrics.BottleneckThreshold)
	}

	// Validate hops (should be non-negative)
	if cfg.Output.DefaultHops < 0 {
		return fmt.Errorf("%w: default_hops must be non-negative, got %d",
			ErrInvalidConfig, cfg.Output.DefaultHops)
	}

	// Validate max_tokens (should be positive)
	if cfg.Output.MaxTokens <= 0 {
		return fmt.Errorf("%w: max_tokens must be positive, got %d",
			ErrInvalidConfig, cfg.Output.MaxTokens)
	}

	return nil
}

// SaveDefault writes the default configuration to .cx/config.yaml in workDir.
// Creates the .cx directory if it doesn't exist.
func SaveDefault(workDir string) (string, error) {
	configDir, err := EnsureConfigDir(workDir)
	if err != nil {
		return "", err
	}

	configPath := filepath.Join(configDir, ConfigFileName)

	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		return "", fmt.Errorf("config file already exists: %s", configPath)
	}

	cfg := DefaultConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshaling config: %w", err)
	}

	// Add header comment
	header := "# cx CLI configuration\n# See https://github.com/anthropics/cx for documentation\n\n"
	data = append([]byte(header), data...)

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", fmt.Errorf("writing config file: %w", err)
	}

	return configPath, nil
}
