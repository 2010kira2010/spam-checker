package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"spam-checker/internal/models"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type NotificationService struct {
	db *gorm.DB
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
	return &NotificationService{db: db}
}

// SendNotification sends notification to all active channels
func (s *NotificationService) SendNotification(subject, message string) error {
	var notifications []models.Notification
	if err := s.db.Where("is_active = ?", true).Find(&notifications).Error; err != nil {
		return fmt.Errorf("failed to get active notifications: %w", err)
	}

	var errors []string
	for _, notification := range notifications {
		var err error
		switch notification.Type {
		case "telegram":
			err = s.sendTelegramNotification(notification.Config, message)
		case "email":
			err = s.sendEmailNotification(notification.Config, subject, message)
		default:
			logrus.Warnf("Unknown notification type: %s", notification.Type)
			continue
		}

		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", notification.Type, err))
			logrus.Errorf("Failed to send %s notification: %v", notification.Type, err)
		} else {
			logrus.Infof("Sent %s notification successfully", notification.Type)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("notification errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// sendTelegramNotification sends notification via Telegram
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

	// Send request
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
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
