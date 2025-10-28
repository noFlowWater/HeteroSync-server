package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	ServerPort string
	DBPath     string

	// Auto-Sync default configuration
	AutoSyncIntervalSec int // Default interval between syncs in seconds
	AutoSyncSampleCount int // Default number of samples per sync
	AutoSyncIntervalMs  int // Default interval between samples in milliseconds
}

func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./time-sync.db"
	}

	// Load auto-sync configuration with defaults
	autoSyncIntervalSec := getEnvAsInt("AUTO_SYNC_INTERVAL_SEC", 600)
	autoSyncSampleCount := getEnvAsInt("AUTO_SYNC_SAMPLE_COUNT", 15)
	autoSyncIntervalMs := getEnvAsInt("AUTO_SYNC_INTERVAL_MS", 200)

	return &Config{
		ServerPort:          port,
		DBPath:              dbPath,
		AutoSyncIntervalSec: autoSyncIntervalSec,
		AutoSyncSampleCount: autoSyncSampleCount,
		AutoSyncIntervalMs:  autoSyncIntervalMs,
	}
}

// getEnvAsInt reads an environment variable as int, returns defaultVal if not set or invalid
func getEnvAsInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func (c *Config) Validate() error {
	if c.ServerPort == "" {
		return fmt.Errorf("server port is required")
	}
	if c.DBPath == "" {
		return fmt.Errorf("database path is required")
	}
	return nil
}
