package database

import (
	"strings"
	"testing"
)

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "db.example.test")
	t.Setenv("POSTGRES_PORT", "6432")
	t.Setenv("POSTGRES_DB", "routing_test")
	t.Setenv("POSTGRES_USER", "repository_test")
	t.Setenv("POSTGRES_PASSWORD", "secret with spaces")
	t.Setenv("POSTGRES_SSLMODE", "require")

	config, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	if config.Host != "db.example.test" || config.Port != 6432 {
		t.Fatalf("unexpected address: %s:%d", config.Host, config.Port)
	}
	if config.Database != "routing_test" || config.User != "repository_test" {
		t.Fatalf("unexpected database identity: database=%q user=%q", config.Database, config.User)
	}
	if config.Password != "secret with spaces" || config.SSLMode != "require" {
		t.Fatal("password or SSL mode was not preserved")
	}
}

func TestConfigFromEnvUsesProjectDefaults(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "")
	t.Setenv("POSTGRES_PORT", "")
	t.Setenv("POSTGRES_DB", "")
	t.Setenv("POSTGRES_USER", "")
	t.Setenv("POSTGRES_PASSWORD", "local-test-password")
	t.Setenv("POSTGRES_SSLMODE", "")

	config, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	if config.Host != defaultPostgresHost || config.Port != defaultPostgresPort {
		t.Fatalf("unexpected default address: %s:%d", config.Host, config.Port)
	}
	if config.Database != defaultPostgresDB || config.User != defaultPostgresUser {
		t.Fatalf("unexpected default database identity: database=%q user=%q", config.Database, config.User)
	}
	if config.SSLMode != defaultPostgresSSLMode {
		t.Fatalf("unexpected default SSL mode: %q", config.SSLMode)
	}
}

func TestConfigFromEnvRejectsInvalidPort(t *testing.T) {
	t.Setenv("POSTGRES_PORT", "not-a-port")
	t.Setenv("POSTGRES_PASSWORD", "not-reported")

	_, err := ConfigFromEnv()
	if err == nil {
		t.Fatal("expected invalid port error")
	}
	if !strings.Contains(err.Error(), "POSTGRES_PORT") {
		t.Fatalf("error does not identify POSTGRES_PORT: %v", err)
	}
}

func TestConfigurationErrorsDoNotExposePassword(t *testing.T) {
	const password = "password-that-must-not-be-reported"
	config := Config{
		Host:     defaultPostgresHost,
		Port:     defaultPostgresPort,
		Database: defaultPostgresDB,
		User:     defaultPostgresUser,
		Password: password,
		SSLMode:  "unsupported",
	}

	_, err := config.connectionString()
	if err == nil {
		t.Fatal("expected unsupported SSL mode error")
	}
	if strings.Contains(err.Error(), password) {
		t.Fatalf("configuration error exposes password: %v", err)
	}
}
