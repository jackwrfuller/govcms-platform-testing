package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Port           int
	DatabaseURL    string
	SitesFile      string
	CypressTimeout time.Duration
}

type SiteEntry struct {
	Name      string `yaml:"name"`
	CanaryURL string `yaml:"canary_url"`
	Active    bool   `yaml:"active"`
}

type SitesFile struct {
	Sites []SiteEntry `yaml:"sites"`
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:           3000,
		DatabaseURL:    "postgres://platform:platform@postgres:5432/platform?sslmode=disable",
		SitesFile:      "sites.yml",
		CypressTimeout: 10 * time.Minute,
	}

	if v := os.Getenv("PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT: %w", err)
		}
		cfg.Port = p
	}

	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	// Lagoon postgres convention
	if v := os.Getenv("POSTGRES_HOST"); v != "" {
		user := envOr("POSTGRES_USERNAME", "platform")
		pass := envOr("POSTGRES_PASSWORD", "platform")
		db := envOr("POSTGRES_DATABASE", "platform")
		port := envOr("POSTGRES_PORT", "5432")
		cfg.DatabaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, v, port, db)
	}

	if v := os.Getenv("SITES_FILE"); v != "" {
		cfg.SitesFile = v
	}

	if v := os.Getenv("CYPRESS_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid CYPRESS_TIMEOUT: %w", err)
		}
		cfg.CypressTimeout = d
	}

	return cfg, nil
}

func LoadSites(path string) ([]SiteEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading sites file: %w", err)
	}

	var sf SitesFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parsing sites file: %w", err)
	}

	return sf.Sites, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
