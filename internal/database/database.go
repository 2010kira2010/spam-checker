package database

import (
	"fmt"
	"spam-checker/internal/config"
	"spam-checker/internal/logger"
	"spam-checker/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Connect establishes database connection
func Connect(cfg config.DatabaseConfig) (*gorm.DB, error) {
	// Use custom GORM logger
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger:                 logger.NewGormLogger(),
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	logger.Info("Successfully connected to database")
	return db, nil
}

// Migrate runs database migrations
func Migrate(db *gorm.DB) error {
	logger.Info("Running database migrations...")

	err := db.AutoMigrate(
		&models.User{},
		&models.PhoneNumber{},
		&models.SpamService{},
		&models.CheckResult{},
		&models.ADBGateway{},
		&models.APIService{},
		&models.SystemSettings{},
		&models.Notification{},
		&models.CheckSchedule{},
		&models.SpamKeyword{},
		&models.Statistics{},
		&models.NumberAllocation{},
	)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Seed initial data
	if err := seedInitialData(db); err != nil {
		return fmt.Errorf("failed to seed initial data: %w", err)
	}

	logger.Info("Database migrations completed successfully")
	return nil
}

// seedInitialData seeds initial data
func seedInitialData(db *gorm.DB) error {
	// Seed spam services
	services := []models.SpamService{
		{Name: "Yandex АОН", Code: "yandex_aon", IsActive: true},
		{Name: "Kaspersky Who Calls", Code: "kaspersky", IsActive: true},
		{Name: "GetContact", Code: "getcontact", IsActive: true},
	}

	for _, service := range services {
		var existing models.SpamService
		if err := db.Where("code = ?", service.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&service).Error; err != nil {
					return fmt.Errorf("failed to create service %s: %w", service.Name, err)
				}
				logger.WithFields(logrus.Fields{
					"service": service.Name,
				}).Info("Created spam service")
			} else {
				return fmt.Errorf("failed to check service %s: %w", service.Name, err)
			}
		}
	}

	// Seed default admin user
	var adminUser models.User
	adminEmail := "admin@spamchecker.com"
	if err := db.Where("email = ?", adminEmail).First(&adminUser).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			adminUser = models.User{
				Username: "admin",
				Email:    adminEmail,
				Password: "$2a$10$ZxK3bqGqOXj7YR2MJxPHPuQKpFkNE2Xk7JaG5LPqJgX6WUis2XAK.", // password: admin123
				Role:     models.RoleAdmin,
				IsActive: true,
			}
			if err := db.Create(&adminUser).Error; err != nil {
				return fmt.Errorf("failed to create admin user: %w", err)
			}
			logger.WithFields(logrus.Fields{
				"username": adminUser.Username,
				"email":    adminUser.Email,
			}).Info("Created default admin user (password: admin123)")
		}
	}

	// Seed default settings
	defaultSettings := []models.SystemSettings{
		{Key: "check_interval_minutes", Value: "60", Type: "int", Category: "scheduler"},
		{Key: "max_concurrent_checks", Value: "3", Type: "int", Category: "performance"},
		{Key: "screenshot_quality", Value: "80", Type: "int", Category: "ocr"},
		{Key: "ocr_confidence_threshold", Value: "70", Type: "int", Category: "ocr"},
		{Key: "notification_batch_size", Value: "50", Type: "int", Category: "notification"},
		{Key: "check_mode", Value: "adb_only", Type: "string", Category: "general"},
	}

	for _, setting := range defaultSettings {
		var existing models.SystemSettings
		if err := db.Where("key = ?", setting.Key).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&setting).Error; err != nil {
					return fmt.Errorf("failed to create setting %s: %w", setting.Key, err)
				}
				logger.WithFields(logrus.Fields{
					"key":      setting.Key,
					"value":    setting.Value,
					"category": setting.Category,
				}).Debug("Created default setting")
			}
		}
	}

	// Seed default spam keywords
	defaultKeywords := []models.SpamKeyword{
		{Keyword: "спам", IsActive: true},
		{Keyword: "реклама", IsActive: true},
		{Keyword: "мошенник", IsActive: true},
		{Keyword: "развод", IsActive: true},
		{Keyword: "коллектор", IsActive: true},
		{Keyword: "банк", IsActive: true},
		{Keyword: "кредит", IsActive: true},
		{Keyword: "микрозайм", IsActive: true},
		{Keyword: "spam", IsActive: true},
		{Keyword: "scam", IsActive: true},
		{Keyword: "fraud", IsActive: true},
		{Keyword: "telemarketing", IsActive: true},
	}

	keywordsCreated := 0
	for _, keyword := range defaultKeywords {
		var existing models.SpamKeyword
		if err := db.Where("keyword = ?", keyword.Keyword).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&keyword).Error; err != nil {
					return fmt.Errorf("failed to create keyword %s: %w", keyword.Keyword, err)
				}
				keywordsCreated++
			}
		}
	}

	if keywordsCreated > 0 {
		logger.WithFields(logrus.Fields{
			"count": keywordsCreated,
		}).Info("Created default spam keywords")
	}

	return nil
}
