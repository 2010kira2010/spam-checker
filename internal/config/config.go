package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	JWT      JWTConfig
	OCR      OCRConfig
	Swagger  SwaggerConfig
}

type AppConfig struct {
	Name        string
	Port        string
	Environment string
	LogLevel    string
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

type JWTConfig struct {
	Secret                string
	ExpirationHours       int
	RefreshExpirationDays int
}

type OCRConfig struct {
	TesseractPath string
	Language      string
	ConfigPath    string
}

type SwaggerConfig struct {
	Host        string
	BasePath    string
	Title       string
	Description string
	Version     string
}

func Load() (*Config, error) {
	// Load .env file if exists
	if err := godotenv.Load(); err != nil {
		// Not an error if .env doesn't exist
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("error loading .env file: %w", err)
		}
	}

	cfg := &Config{
		App: AppConfig{
			Name:        getEnv("APP_NAME", "SpamChecker"),
			Port:        getEnv("APP_PORT", "8080"),
			Environment: getEnv("APP_ENV", "development"),
			LogLevel:    getEnv("LOG_LEVEL", "info"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			Name:     getEnv("DB_NAME", "spamchecker"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			Secret:                getEnv("JWT_SECRET", "your-secret-key"),
			ExpirationHours:       getEnvAsInt("JWT_EXPIRATION_HOURS", 24),
			RefreshExpirationDays: getEnvAsInt("JWT_REFRESH_EXPIRATION_DAYS", 7),
		},
		OCR: OCRConfig{
			TesseractPath: getEnv("TESSERACT_PATH", "/usr/bin/tesseract"),
			Language:      getEnv("OCR_LANGUAGE", "rus+eng"),
			ConfigPath:    getEnv("OCR_CONFIG_PATH", ""),
		},
		Swagger: SwaggerConfig{
			Host:        getEnv("SWAGGER_HOST", "localhost:8080"),
			BasePath:    getEnv("SWAGGER_BASE_PATH", "/api/v1"),
			Title:       getEnv("SWAGGER_TITLE", "SpamChecker API"),
			Description: getEnv("SWAGGER_DESCRIPTION", "API for checking phone numbers in spam services"),
			Version:     getEnv("SWAGGER_VERSION", "1.0.0"),
		},
	}

	return cfg, nil
}

func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return defaultValue
}
