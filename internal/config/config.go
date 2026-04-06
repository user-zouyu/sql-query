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

	// S3 configuration (Phase 2)
	S3AccessKey string
	S3SecretKey string
	S3Region    string
	S3Endpoint  string // optional, for OSS/MinIO compatibility
	AuditLogDir string // audit log directory, default "."
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		DBDSN:        os.Getenv("DB_DSN"),
		QueryTimeout: 300,
		S3AccessKey:  os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:  os.Getenv("S3_SECRET_KEY"),
		S3Region:     os.Getenv("S3_REGION"),
		S3Endpoint:   os.Getenv("S3_ENDPOINT"),
		AuditLogDir:  os.Getenv("AUDIT_LOG_DIR"),
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

// HasS3Config checks whether S3 credentials are fully configured.
func (c *Config) HasS3Config() bool {
	return c.S3AccessKey != "" && c.S3SecretKey != "" && c.S3Region != ""
}
