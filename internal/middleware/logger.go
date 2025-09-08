package middleware

import (
	"time"

	"spam-checker/internal/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// LoggerConfig defines the config for logger middleware
type LoggerConfig struct {
	SkipPaths []string // Paths to skip logging (e.g., /health)
}

// NewLogger creates a new logger middleware
func NewLogger(config ...LoggerConfig) fiber.Handler {
	cfg := LoggerConfig{
		SkipPaths: []string{},
	}

	if len(config) > 0 {
		cfg = config[0]
	}

	return func(c *fiber.Ctx) error {
		// Skip logging for certain paths
		for _, path := range cfg.SkipPaths {
			if c.Path() == path {
				return c.Next()
			}
		}

		// Generate request ID
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Set request ID in context
		c.Locals(logger.RequestIDKey, requestID)
		c.Set("X-Request-ID", requestID)

		// Start timer
		start := time.Now()

		// Log request
		logger.WithFields(logrus.Fields{
			logger.RequestIDKey: requestID,
			"method":            c.Method(),
			"path":              c.Path(),
			"ip":                c.IP(),
			"user_agent":        c.Get("User-Agent"),
		}).Info("Incoming request")

		// Process request
		err := c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get status code
		status := c.Response().StatusCode()

		// Prepare log fields
		fields := logrus.Fields{
			logger.RequestIDKey: requestID,
			"method":            c.Method(),
			"path":              c.Path(),
			"status":            status,
			"latency":           latency.Milliseconds(),
			"latency_human":     latency.String(),
			"ip":                c.IP(),
			"bytes_sent":        len(c.Response().Body()),
		}

		// Add error if exists
		if err != nil {
			fields["error"] = err.Error()
		}

		// Log based on status code
		entry := logger.WithFields(fields)

		switch {
		case status >= 500:
			entry.Error("Server error")
		case status >= 400:
			entry.Warn("Client error")
		case status >= 300:
			entry.Info("Redirection")
		default:
			entry.Info("Request completed")
		}

		return err
	}
}

// GetRequestLogger returns a logger instance with request context
func GetRequestLogger(c *fiber.Ctx) *logrus.Entry {
	requestID := c.Locals(logger.RequestIDKey)

	return logger.WithFields(logrus.Fields{
		logger.RequestIDKey: requestID,
		"method":            c.Method(),
		"path":              c.Path(),
		"ip":                c.IP(),
	})
}
