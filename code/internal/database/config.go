package database

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	defaultPostgresHost    = "127.0.0.1"
	defaultPostgresPort    = uint16(55432)
	defaultPostgresDB      = "waste_routing"
	defaultPostgresUser    = "waste_routing"
	defaultPostgresSSLMode = "disable"
)

// Config contains the PostgreSQL connection parameters used by the pool.
type Config struct {
	Host     string
	Port     uint16
	Database string
	User     string
	Password string
	SSLMode  string
}

// ConfigFromEnv reads connection settings from the POSTGRES_* environment.
// It intentionally does not load .env files; callers decide how process
// environment is populated.
func ConfigFromEnv() (Config, error) {
	portText := envOrDefault("POSTGRES_PORT", strconv.Itoa(int(defaultPostgresPort)))
	parsedPort, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || parsedPort == 0 {
		return Config{}, fmt.Errorf("read PostgreSQL configuration: invalid POSTGRES_PORT %q", portText)
	}

	password, ok := os.LookupEnv("POSTGRES_PASSWORD")
	if !ok || password == "" {
		return Config{}, fmt.Errorf("read PostgreSQL configuration: POSTGRES_PASSWORD is required")
	}

	config := Config{
		Host:     envOrDefault("POSTGRES_HOST", defaultPostgresHost),
		Port:     uint16(parsedPort),
		Database: envOrDefault("POSTGRES_DB", defaultPostgresDB),
		User:     envOrDefault("POSTGRES_USER", defaultPostgresUser),
		Password: password,
		SSLMode:  envOrDefault("POSTGRES_SSLMODE", defaultPostgresSSLMode),
	}
	if err := config.validate(); err != nil {
		return Config{}, fmt.Errorf("read PostgreSQL configuration: %w", err)
	}
	return config, nil
}

func envOrDefault(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func (config Config) validate() error {
	if strings.TrimSpace(config.Host) == "" {
		return fmt.Errorf("host is required")
	}
	if config.Port == 0 {
		return fmt.Errorf("port must be greater than zero")
	}
	if strings.TrimSpace(config.Database) == "" {
		return fmt.Errorf("database is required")
	}
	if strings.TrimSpace(config.User) == "" {
		return fmt.Errorf("user is required")
	}
	if config.Password == "" {
		return fmt.Errorf("password is required")
	}

	switch config.SSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
		return nil
	default:
		return fmt.Errorf("unsupported SSL mode %q", config.SSLMode)
	}
}

func (config Config) connectionString() (string, error) {
	if err := config.validate(); err != nil {
		return "", err
	}

	connectionURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(config.User, config.Password),
		Host:   net.JoinHostPort(config.Host, strconv.Itoa(int(config.Port))),
		Path:   "/" + config.Database,
	}
	query := connectionURL.Query()
	query.Set("sslmode", config.SSLMode)
	connectionURL.RawQuery = query.Encode()
	return connectionURL.String(), nil
}
