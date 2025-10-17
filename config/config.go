package config

import (
	"fmt"
	"os"
)

type Config struct {
	ServerPort string
	DBPath     string
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

	return &Config{
		ServerPort: port,
		DBPath:     dbPath,
	}
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
