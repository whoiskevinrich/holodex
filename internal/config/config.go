// Package config loads Holodex configuration with the precedence defined in
// ADR-014: CLI flags > environment variables > config file (holodex.yaml) >
// built-in defaults. This package handles defaults -> YAML -> env; CLI flag
// overlay is applied by the caller (cmd/holodex).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Paths
	MediaPath    string `yaml:"media_path"`
	DataPath     string `yaml:"data_path"`
	DatabasePath string `yaml:"database_path"`

	// Server
	Port     int    `yaml:"port"`
	LogLevel string `yaml:"log_level"`

	// Scanner (ADR-011, ADR-018)
	ScanIntervalSeconds int  `yaml:"scan_interval_seconds"`
	ScanWorkers         int  `yaml:"scan_workers"`
	FollowSymlinks      bool `yaml:"follow_symlinks"`
	ScanMaxDepth        int  `yaml:"scan_max_depth"`
	ScanMinAgeSeconds   int  `yaml:"scan_min_age_seconds"`

	// Cache (ADR-008)
	CacheBackend     string `yaml:"cache_backend"`
	CacheMaxMemoryMB int    `yaml:"cache_max_memory_mb"`
	RedisURL         string `yaml:"redis_url"`

	// MCP (Phase 2, ADR-005)
	MCPEnabled   bool   `yaml:"mcp_enabled"`
	MCPTransport string `yaml:"mcp_transport"`
	MCPPort      int    `yaml:"mcp_port"`
}

// Defaults returns the built-in configuration (the lowest-precedence layer).
func Defaults() Config {
	return Config{
		DataPath:            "./data",
		Port:                7800,
		LogLevel:            "info",
		ScanIntervalSeconds: 300,
		ScanWorkers:         4,
		FollowSymlinks:      true,
		ScanMaxDepth:        64,
		ScanMinAgeSeconds:   5,
		CacheBackend:        "memory",
		CacheMaxMemoryMB:    128,
		MCPTransport:        "http",
		MCPPort:             7801,
	}
}

// Load builds a Config by layering, in increasing precedence:
// defaults -> YAML file (if path non-empty and exists) -> environment.
func Load(yamlPath string) (Config, error) {
	cfg := Defaults()

	if yamlPath != "" {
		if data, err := os.ReadFile(yamlPath); err == nil {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return cfg, fmt.Errorf("parse config %s: %w", yamlPath, err)
			}
		} else if !os.IsNotExist(err) {
			return cfg, fmt.Errorf("read config %s: %w", yamlPath, err)
		}
	}

	applyEnv(&cfg)
	cfg.derive()
	return cfg, nil
}

// derive fills computed defaults that depend on other fields.
func (c *Config) derive() {
	if c.DatabasePath == "" {
		c.DatabasePath = filepath.Join(c.DataPath, "holodex.db")
	}
}

// Overrides carries CLI-flag values — the highest-precedence layer (ADR-014,
// F9.5: CLI > env > yaml > defaults). Zero/empty fields are left untouched, so
// the caller can pass only the flags the user actually set.
type Overrides struct {
	Port      int
	MediaPath string
	DataPath  string
	LogLevel  string
}

// ApplyOverrides overlays CLI flags on top of the loaded config and re-derives
// computed fields (e.g. DatabasePath when DataPath changes).
func (c *Config) ApplyOverrides(o Overrides) {
	if o.Port != 0 {
		c.Port = o.Port
	}
	if o.MediaPath != "" {
		c.MediaPath = o.MediaPath
	}
	if o.DataPath != "" {
		c.DataPath = o.DataPath
		c.DatabasePath = "" // re-derive under the new data dir
	}
	if o.LogLevel != "" {
		c.LogLevel = o.LogLevel
	}
	c.derive()
}

func applyEnv(c *Config) {
	c.MediaPath = envStr("MEDIA_PATH", c.MediaPath)
	c.DataPath = envStr("DATA_PATH", c.DataPath)
	c.DatabasePath = envStr("DATABASE_PATH", c.DatabasePath)
	c.Port = envInt("PORT", c.Port)
	c.LogLevel = envStr("LOG_LEVEL", c.LogLevel)

	c.ScanIntervalSeconds = envInt("SCAN_INTERVAL_SECONDS", c.ScanIntervalSeconds)
	c.ScanWorkers = envInt("SCAN_WORKERS", c.ScanWorkers)
	c.FollowSymlinks = envBool("FOLLOW_SYMLINKS", c.FollowSymlinks)
	c.ScanMaxDepth = envInt("SCAN_MAX_DEPTH", c.ScanMaxDepth)
	c.ScanMinAgeSeconds = envInt("SCAN_MIN_AGE_SECONDS", c.ScanMinAgeSeconds)

	c.CacheBackend = envStr("CACHE_BACKEND", c.CacheBackend)
	c.CacheMaxMemoryMB = envInt("CACHE_MAX_MEMORY_MB", c.CacheMaxMemoryMB)
	c.RedisURL = envStr("REDIS_URL", c.RedisURL)

	c.MCPEnabled = envBool("MCP_ENABLED", c.MCPEnabled)
	c.MCPTransport = envStr("MCP_TRANSPORT", c.MCPTransport)
	c.MCPPort = envInt("MCP_PORT", c.MCPPort)
}

func envStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
