package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"spam-checker/internal/logger"
	"spam-checker/internal/models"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type NotificationService struct {
	db  *gorm.DB
	log *logrus.Entry
}

type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type EmailConfig struct {
	SMTPHost     string   `json:"smtp_host"`
	SMTPPort     string   `json:"smtp_port"`
	SMTPUser     string   `json:"smtp_user"`
	SMTPPassword string   `json:"smtp_password"`
	FromEmail    string   `json:"from_email"`
	ToEmails     []string `json:"to_emails"`
}

func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{
		db:  db,
		log: logger.WithField("service", "NotificationService"),
	}
}

// SendNotification sends notification to all active channels
func (s *NotificationService) SendNotification(subject, message string) error {
	log := s.log.WithFields(logrus.Fields{
		"method": "SendNotification",
	})

	var notifications []models.Notification
	if err := s.db.Where("is_active = ?", true).Find(&notifications).Error; err != nil {
		return fmt.Errorf("failed to get active notifications: %w", err)
	}

	if len(notifications) == 0 {
		log.Warn("No active notification channels configured")
		return nil
	}

	var errors []string
	successCount := 0

	for _, notification := range notifications {
		var err error
		switch notification.Type {
		case "telegram":
			err = s.sendTelegramNotification(notification.Config, message)
		case "email":
			err = s.sendEmailNotification(notification.Config, subject, message)
		default:
			log.Warnf("Unknown notification type: %s", notification.Type)
			continue
		}

		if err != nil {
			// Check if it's a configuration error (don't log as error)
			if strings.Contains(err.Error(), "invalid bot token") ||
				strings.Contains(err.Error(), "forbidden") ||
				strings.Contains(err.Error(), "bad request") {
				log.Warnf("Notification configuration issue for %s: %v", notification.Type, err)
				errors = append(errors, fmt.Sprintf("%s (config issue): %v", notification.Type, err))
			} else {
				// Temporary error
				errors = append(errors, fmt.Sprintf("%s: %v", notification.Type, err))
				log.Errorf("Failed to send %s notification: %v", notification.Type, err)
			}
		} else {
			successCount++
			log.Infof("Sent %s notification successfully", notification.Type)
		}
	}

	// Return error only if ALL notifications failed
	if successCount == 0 && len(errors) > 0 {
		return fmt.Errorf("all notifications failed: %s", strings.Join(errors, "; "))
	} else if len(errors) > 0 {
		// Some succeeded, some failed - just log warning
		log.Warnf("Some notifications failed (%d/%d succeeded): %s",
			successCount, len(notifications), strings.Join(errors, "; "))
	}

	return nil
}

// sendTelegramNotification sends notification via Telegram with retry
func (s *NotificationService) sendTelegramNotification(configJSON string, message string) error {
	var config TelegramConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return fmt.Errorf("invalid telegram config: %w", err)
	}

	if config.BotToken == "" || config.ChatID == "" {
		return fmt.Errorf("telegram bot token and chat ID are required")
	}

	// Prepare API URL
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", config.BotToken)

	// Prepare request body
	reqBody := map[string]interface{}{
		"chat_id":    config.ChatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry logic
	maxRetries := 3
	var lastError error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create HTTP client with timeout
		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		// Send request
		resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			lastError = fmt.Errorf("failed to send telegram message (attempt %d/%d): %w", attempt, maxRetries, err)
			s.log.Warnf("Telegram API request failed: %v", lastError)

			// Wait before retry
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
			}
			continue
		}
		defer resp.Body.Close()

		// Read response body for debugging
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyString := string(bodyBytes)

		// Check response status
		switch resp.StatusCode {
		case http.StatusOK:
			// Success
			s.log.Debug("Telegram notification sent successfully")
			return nil

		case http.StatusBadRequest:
			// Client error - don't retry
			return fmt.Errorf("telegram API bad request (400): %s", bodyString)

		case http.StatusUnauthorized:
			// Invalid token - don't retry
			return fmt.Errorf("telegram API unauthorized (401): invalid bot token")

		case http.StatusForbidden:
			// Bot blocked or chat not found - don't retry
			return fmt.Errorf("telegram API forbidden (403): %s", bodyString)

		case http.StatusNotFound:
			// Method not found - don't retry
			return fmt.Errorf("telegram API not found (404): %s", bodyString)

		case http.StatusTooManyRequests:
			// Rate limited - retry with exponential backoff
			retryAfter := resp.Header.Get("Retry-After")
			waitTime := time.Duration(attempt) * 5 * time.Second

			if retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil {
					waitTime = time.Duration(seconds) * time.Second
				}
			}

			lastError = fmt.Errorf("telegram API rate limited (429), retry after %v", waitTime)
			s.log.Warnf("Telegram API rate limited: %v", lastError)

			if attempt < maxRetries {
				time.Sleep(waitTime)
			}
			continue

		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			// Server errors - retry
			lastError = fmt.Errorf("telegram API server error (%d): %s", resp.StatusCode, bodyString)
			s.log.Warnf("Telegram API server error: %v", lastError)

			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt) * 3 * time.Second)
			}
			continue

		default:
			// Unknown error - don't retry
			return fmt.Errorf("telegram API returned unexpected status %d: %s", resp.StatusCode, bodyString)
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastError)
}

// sendEmailNotification sends notification via email
func (s *NotificationService) sendEmailNotification(configJSON, subject, message string) error {
	var config EmailConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return fmt.Errorf("invalid email config: %w", err)
	}

	if config.SMTPHost == "" || config.SMTPPort == "" || len(config.ToEmails) == 0 {
		return fmt.Errorf("email configuration is incomplete")
	}

	// Setup authentication
	auth := smtp.PlainAuth("", config.SMTPUser, config.SMTPPassword, config.SMTPHost)

	// Prepare email
	to := strings.Join(config.ToEmails, ", ")
	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		config.FromEmail,
		to,
		subject,
		s.formatEmailBody(subject, message),
	))

	// Send email
	addr := fmt.Sprintf("%s:%s", config.SMTPHost, config.SMTPPort)
	err := smtp.SendMail(addr, auth, config.FromEmail, config.ToEmails, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// formatEmailBody formats message for email
func (s *NotificationService) formatEmailBody(subject, message string) string {
	// Convert plain text to HTML
	htmlMessage := strings.ReplaceAll(message, "\n", "<br>")

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body {
            font-family: Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .header {
            background-color: #4CAF50;
            color: white;
            padding: 20px;
            text-align: center;
            border-radius: 5px 5px 0 0;
        }
        .content {
            background-color: #f4f4f4;
            padding: 20px;
            border-radius: 0 0 5px 5px;
        }
        .footer {
            margin-top: 20px;
            text-align: center;
            color: #666;
            font-size: 12px;
        }
    </style>
</head>
<body>
    <div class="header">
        <h2>SpamChecker Notification</h2>
    </div>
    <div class="content">
        <h3>%s</h3>
        <p>%s</p>
    </div>
    <div class="footer">
        <p>This is an automated message from SpamChecker</p>
    </div>
</body>
</html>
	`, subject, htmlMessage)
}

// GetNotifications gets all notifications
func (s *NotificationService) GetNotifications() ([]models.Notification, error) {
	var notifications []models.Notification
	if err := s.db.Find(&notifications).Error; err != nil {
		return nil, fmt.Errorf("failed to get notifications: %w", err)
	}
	return notifications, nil
}

// GetNotificationByID gets notification by ID
func (s *NotificationService) GetNotificationByID(id uint) (*models.Notification, error) {
	var notification models.Notification
	if err := s.db.First(&notification, id).Error; err != nil {
		return nil, fmt.Errorf("notification not found: %w", err)
	}
	return &notification, nil
}

// CreateNotification creates a new notification channel
func (s *NotificationService) CreateNotification(notification *models.Notification) error {
	// Validate config based on type
	switch notification.Type {
	case "telegram":
		var config TelegramConfig
		if err := json.Unmarshal([]byte(notification.Config), &config); err != nil {
			return fmt.Errorf("invalid telegram config: %w", err)
		}
		if config.BotToken == "" || config.ChatID == "" {
			return fmt.Errorf("telegram bot token and chat ID are required")
		}
	case "email":
		var config EmailConfig
		if err := json.Unmarshal([]byte(notification.Config), &config); err != nil {
			return fmt.Errorf("invalid email config: %w", err)
		}
		if config.SMTPHost == "" || config.SMTPPort == "" {
			return fmt.Errorf("SMTP host and port are required")
		}
	default:
		return fmt.Errorf("unsupported notification type: %s", notification.Type)
	}

	if err := s.db.Create(notification).Error; err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	return nil
}

// UpdateNotification updates a notification channel
func (s *NotificationService) UpdateNotification(id uint, updates map[string]interface{}) error {
	// If config is being updated, validate it
	if configStr, ok := updates["config"].(string); ok {
		var notification models.Notification
		if err := s.db.First(&notification, id).Error; err != nil {
			return fmt.Errorf("notification not found: %w", err)
		}

		// Validate new config
		tempNotif := models.Notification{
			Type:   notification.Type,
			Config: configStr,
		}

		// Use create validation logic
		if err := s.validateNotificationConfig(&tempNotif); err != nil {
			return err
		}
	}

	if err := s.db.Model(&models.Notification{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update notification: %w", err)
	}

	return nil
}

// DeleteNotification deletes a notification channel
func (s *NotificationService) DeleteNotification(id uint) error {
	if err := s.db.Delete(&models.Notification{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}
	return nil
}

// TestNotification tests a notification channel
func (s *NotificationService) TestNotification(id uint) error {
	notification, err := s.GetNotificationByID(id)
	if err != nil {
		return err
	}

	testMessage := "This is a test notification from SpamChecker. If you received this message, your notification channel is configured correctly!"

	switch notification.Type {
	case "telegram":
		return s.sendTelegramNotification(notification.Config, testMessage)
	case "email":
		return s.sendEmailNotification(notification.Config, "SpamChecker Test Notification", testMessage)
	default:
		return fmt.Errorf("unsupported notification type: %s", notification.Type)
	}
}

// validateNotificationConfig validates notification configuration
func (s *NotificationService) validateNotificationConfig(notification *models.Notification) error {
	switch notification.Type {
	case "telegram":
		var config TelegramConfig
		if err := json.Unmarshal([]byte(notification.Config), &config); err != nil {
			return fmt.Errorf("invalid telegram config: %w", err)
		}
		if config.BotToken == "" || config.ChatID == "" {
			return fmt.Errorf("telegram bot token and chat ID are required")
		}
	case "email":
		var config EmailConfig
		if err := json.Unmarshal([]byte(notification.Config), &config); err != nil {
			return fmt.Errorf("invalid email config: %w", err)
		}
		if config.SMTPHost == "" || config.SMTPPort == "" {
			return fmt.Errorf("SMTP host and port are required")
		}
		if len(config.ToEmails) == 0 {
			return fmt.Errorf("at least one recipient email is required")
		}
	default:
		return fmt.Errorf("unsupported notification type: %s", notification.Type)
	}
	return nil
}
