package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	DBDSN        string
	QueryTimeout int // seconds, default 300
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		DBDSN:        os.Getenv("DB_DSN"),
		QueryTimeout: 300,
	}

	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("环境变量 DB_DSN 未配置")
	}

	if v := os.Getenv("QUERY_TIMEOUT"); v != "" {
		t, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("QUERY_TIMEOUT 格式无效: %s", v)
		}
		cfg.QueryTimeout = t
	}

	return cfg, nil
}
