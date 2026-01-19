package config

// DefaultConfig returns configuration with sensible defaults.
// These defaults are used when no config file exists or when
// config file is missing specific fields.
func DefaultConfig() *Config {
	return &Config{
		Storage: StorageConfig{
			Backend: "dolt",
		},
		Scan: ScanConfig{
			Languages: []string{"go"},
			Exclude: []string{
				"vendor/**",
				"node_modules/**",
				"dist/**",
				"build/**",
				"*_test.go",
				"**/*_mock.go",
				"**/testdata/**",
			},
		},
		Metrics: MetricsConfig{
			PageRankDamping:     0.85,
			PageRankIterations:  100,
			KeystoneThreshold:   0.30,
			BottleneckThreshold: 0.20,
		},
		Output: OutputConfig{
			DefaultDensity: "medium",
			DefaultHops:    1,
			MaxTokens:      4000,
		},
		Guard: GuardConfig{
			FailOnCoverageRegression: true,
			MinCoverageForKeystones:  50.0,
			FailOnWarnings:           false,
		},
	}
}

// Merge merges loaded config with defaults.
// Values from loaded config take precedence over defaults.
// Returns a new Config with merged values.
func Merge(loaded, defaults *Config) *Config {
	result := &Config{}

	// Merge Storage config
	result.Storage = mergeStorageConfig(loaded.Storage, defaults.Storage)

	// Merge Scan config
	result.Scan = mergeScanConfig(loaded.Scan, defaults.Scan)

	// Merge Metrics config
	result.Metrics = mergeMetricsConfig(loaded.Metrics, defaults.Metrics)

	// Merge Output config
	result.Output = mergeOutputConfig(loaded.Output, defaults.Output)

	// Merge Guard config
	result.Guard = mergeGuardConfig(loaded.Guard, defaults.Guard)

	return result
}

func mergeStorageConfig(loaded, defaults StorageConfig) StorageConfig {
	result := StorageConfig{}

	// Backend: use loaded if non-empty
	if loaded.Backend != "" {
		result.Backend = loaded.Backend
	} else {
		result.Backend = defaults.Backend
	}

	return result
}

func mergeScanConfig(loaded, defaults ScanConfig) ScanConfig {
	result := ScanConfig{}

	// Use loaded languages if provided, otherwise defaults
	if len(loaded.Languages) > 0 {
		result.Languages = loaded.Languages
	} else {
		result.Languages = defaults.Languages
	}

	// Use loaded exclude patterns if provided, otherwise defaults
	if len(loaded.Exclude) > 0 {
		result.Exclude = loaded.Exclude
	} else {
		result.Exclude = defaults.Exclude
	}

	return result
}

func mergeMetricsConfig(loaded, defaults MetricsConfig) MetricsConfig {
	result := MetricsConfig{}

	// PageRankDamping: use loaded if non-zero
	if loaded.PageRankDamping != 0 {
		result.PageRankDamping = loaded.PageRankDamping
	} else {
		result.PageRankDamping = defaults.PageRankDamping
	}

	// PageRankIterations: use loaded if non-zero
	if loaded.PageRankIterations != 0 {
		result.PageRankIterations = loaded.PageRankIterations
	} else {
		result.PageRankIterations = defaults.PageRankIterations
	}

	// KeystoneThreshold: use loaded if non-zero
	if loaded.KeystoneThreshold != 0 {
		result.KeystoneThreshold = loaded.KeystoneThreshold
	} else {
		result.KeystoneThreshold = defaults.KeystoneThreshold
	}

	// BottleneckThreshold: use loaded if non-zero
	if loaded.BottleneckThreshold != 0 {
		result.BottleneckThreshold = loaded.BottleneckThreshold
	} else {
		result.BottleneckThreshold = defaults.BottleneckThreshold
	}

	return result
}

func mergeOutputConfig(loaded, defaults OutputConfig) OutputConfig {
	result := OutputConfig{}

	// DefaultDensity: use loaded if non-empty
	if loaded.DefaultDensity != "" {
		result.DefaultDensity = loaded.DefaultDensity
	} else {
		result.DefaultDensity = defaults.DefaultDensity
	}

	// DefaultHops: use loaded if non-zero
	if loaded.DefaultHops != 0 {
		result.DefaultHops = loaded.DefaultHops
	} else {
		result.DefaultHops = defaults.DefaultHops
	}

	// MaxTokens: use loaded if non-zero
	if loaded.MaxTokens != 0 {
		result.MaxTokens = loaded.MaxTokens
	} else {
		result.MaxTokens = defaults.MaxTokens
	}

	return result
}

func mergeGuardConfig(loaded, defaults GuardConfig) GuardConfig {
	result := GuardConfig{}

	// FailOnCoverageRegression: use loaded value (bool can't distinguish unset from false)
	// For booleans, we use the loaded value since YAML unmarshals missing as false
	// Users who want false will set it explicitly
	result.FailOnCoverageRegression = loaded.FailOnCoverageRegression
	if !loaded.FailOnCoverageRegression && defaults.FailOnCoverageRegression {
		// If loaded is false but default is true, check if it was explicitly set
		// For now, default to true if loaded is zero-value
		result.FailOnCoverageRegression = defaults.FailOnCoverageRegression
	}

	// MinCoverageForKeystones: use loaded if non-zero
	if loaded.MinCoverageForKeystones != 0 {
		result.MinCoverageForKeystones = loaded.MinCoverageForKeystones
	} else {
		result.MinCoverageForKeystones = defaults.MinCoverageForKeystones
	}

	// FailOnWarnings: use loaded value (same bool handling)
	result.FailOnWarnings = loaded.FailOnWarnings

	return result
}

// ValidDensities lists the valid values for output density
var ValidDensities = []string{"sparse", "medium", "dense"}

// IsValidDensity checks if the given density value is valid
func IsValidDensity(density string) bool {
	for _, valid := range ValidDensities {
		if density == valid {
			return true
		}
	}
	return false
}
