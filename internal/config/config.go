// Package config handles YAML-based configuration with environment variable overrides.
package config

import (
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for Navi.
type Config struct {
	LogLevel  string           `yaml:"log_level"`
	Cache     CacheConfig      `yaml:"cache"`
	Vault     *VaultConfig     `yaml:"vault"`
	Chronicle *ChronicleConfig `yaml:"chronicle"`
	Beacon    *BeaconConfig    `yaml:"beacon"`
	Radar     *RadarConfig     `yaml:"radar"`
}

// CacheConfig controls the in-memory TTL cache.
type CacheConfig struct {
	DefaultTTL time.Duration `yaml:"default_ttl"`
	MaxEntries int           `yaml:"max_entries"`
}

// VaultConfig holds settings for the billing provider.
type VaultConfig struct {
	Backend string `yaml:"backend"`
	APIKey  string `yaml:"-"` // never from YAML; always from env
}

// ChronicleConfig holds settings for the communications provider.
type ChronicleConfig struct {
	Backend string `yaml:"backend"`
	APIKey  string `yaml:"-"` // never from YAML; always from env
}

// BeaconConfig holds settings for the support ticket provider.
type BeaconConfig struct {
	Backend   string `yaml:"backend"`
	Subdomain string `yaml:"subdomain"`
	APIKey    string `yaml:"-"` // never from YAML; always from env
}

// RadarConfig holds settings for the product usage analytics provider.
type RadarConfig struct {
	Backend   string `yaml:"backend"`
	APIKey    string `yaml:"-"` // never from YAML; always from env
	ProjectID string `yaml:"project_id"`
}

// Load reads the YAML config file and applies environment variable overrides.
func Load() (*Config, error) {
	cfg := &Config{
		LogLevel: "info",
		Cache: CacheConfig{
			DefaultTTL: 5 * time.Minute,
			MaxEntries: 1000,
		},
	}

	// Load YAML file if specified or if default exists.
	path := os.Getenv("NAVI_CONFIG")
	if path == "" {
		path = "navi.yaml"
	}

	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}
	// If file doesn't exist, that's fine — use defaults + env vars.

	// Environment variable overrides.
	if v := os.Getenv("NAVI_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}

	// Vault config from env vars.
	vaultAPIKey := os.Getenv("NAVI_VAULT_API_KEY")
	vaultBackend := os.Getenv("NAVI_VAULT_BACKEND")
	if vaultAPIKey != "" || vaultBackend != "" || cfg.Vault != nil {
		if cfg.Vault == nil {
			cfg.Vault = &VaultConfig{Backend: "stripe"}
		}
		cfg.Vault.APIKey = vaultAPIKey
		if vaultBackend != "" {
			cfg.Vault.Backend = vaultBackend
		}
	}

	// Chronicle config from env vars.
	chronicleAPIKey := os.Getenv("NAVI_CHRONICLE_API_KEY")
	chronicleBackend := os.Getenv("NAVI_CHRONICLE_BACKEND")
	if chronicleAPIKey != "" || chronicleBackend != "" || cfg.Chronicle != nil {
		if cfg.Chronicle == nil {
			cfg.Chronicle = &ChronicleConfig{Backend: "hubspot"}
		}
		cfg.Chronicle.APIKey = chronicleAPIKey
		if chronicleBackend != "" {
			cfg.Chronicle.Backend = chronicleBackend
		}
	}

	// Beacon config from env vars.
	beaconAPIKey := os.Getenv("NAVI_BEACON_API_KEY")
	beaconBackend := os.Getenv("NAVI_BEACON_BACKEND")
	beaconSubdomain := os.Getenv("NAVI_BEACON_SUBDOMAIN")
	if beaconAPIKey != "" || beaconBackend != "" || beaconSubdomain != "" || cfg.Beacon != nil {
		if cfg.Beacon == nil {
			cfg.Beacon = &BeaconConfig{Backend: "zendesk"}
		}
		cfg.Beacon.APIKey = beaconAPIKey
		if beaconBackend != "" {
			cfg.Beacon.Backend = beaconBackend
		}
		if beaconSubdomain != "" {
			cfg.Beacon.Subdomain = beaconSubdomain
		}
	}

	// Radar config from env vars.
	radarAPIKey := os.Getenv("NAVI_RADAR_API_KEY")
	radarBackend := os.Getenv("NAVI_RADAR_BACKEND")
	radarProjectID := os.Getenv("NAVI_RADAR_PROJECT_ID")
	if radarAPIKey != "" || radarBackend != "" || radarProjectID != "" || cfg.Radar != nil {
		if cfg.Radar == nil {
			cfg.Radar = &RadarConfig{Backend: "mixpanel"}
		}
		cfg.Radar.APIKey = radarAPIKey
		if radarBackend != "" {
			cfg.Radar.Backend = radarBackend
		}
		if radarProjectID != "" {
			cfg.Radar.ProjectID = radarProjectID
		}
	}

	return cfg, nil
}

// ParseLogLevel maps a human-readable level string to an integer slog level.
func ParseLogLevel(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return -4 // slog.LevelDebug
	case "info", "":
		return 0 // slog.LevelInfo
	case "warn":
		return 4 // slog.LevelWarn
	case "error":
		return 8 // slog.LevelError
	default:
		return 0
	}
}
