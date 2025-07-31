package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"spam-checker/internal/models"
	"strconv"

	"gorm.io/gorm"
)

type SettingsService struct {
	db *gorm.DB
}

func NewSettingsService(db *gorm.DB) *SettingsService {
	return &SettingsService{db: db}
}

// GetSetting gets a single setting by key
func (s *SettingsService) GetSetting(key string) (*models.SystemSettings, error) {
	var setting models.SystemSettings
	if err := s.db.Where("key = ?", key).First(&setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("setting not found")
		}
		return nil, fmt.Errorf("failed to get setting: %w", err)
	}
	return &setting, nil
}

// GetSettingValue gets setting value with type conversion
func (s *SettingsService) GetSettingValue(key string) (interface{}, error) {
	setting, err := s.GetSetting(key)
	if err != nil {
		return nil, err
	}

	switch setting.Type {
	case "int":
		return strconv.Atoi(setting.Value)
	case "bool":
		return strconv.ParseBool(setting.Value)
	case "float":
		return strconv.ParseFloat(setting.Value, 64)
	case "json":
		var result interface{}
		err := json.Unmarshal([]byte(setting.Value), &result)
		return result, err
	default:
		return setting.Value, nil
	}
}

// GetSettingsByCategory gets all settings in a category
func (s *SettingsService) GetSettingsByCategory(category string) ([]models.SystemSettings, error) {
	var settings []models.SystemSettings
	if err := s.db.Where("category = ?", category).Find(&settings).Error; err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}
	return settings, nil
}

// GetAllSettings gets all settings
func (s *SettingsService) GetAllSettings() ([]models.SystemSettings, error) {
	var settings []models.SystemSettings
	if err := s.db.Order("category, key").Find(&settings).Error; err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}
	return settings, nil
}

// UpdateSetting updates a setting value
func (s *SettingsService) UpdateSetting(key string, value interface{}) error {
	setting, err := s.GetSetting(key)
	if err != nil {
		return err
	}

	// Convert value to string based on type
	var stringValue string
	switch setting.Type {
	case "json":
		bytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON value: %w", err)
		}
		stringValue = string(bytes)
	default:
		stringValue = fmt.Sprintf("%v", value)
	}

	// Validate value based on type
	if err := s.validateSettingValue(setting.Type, stringValue); err != nil {
		return err
	}

	// Update setting
	if err := s.db.Model(setting).Update("value", stringValue).Error; err != nil {
		return fmt.Errorf("failed to update setting: %w", err)
	}

	return nil
}

// CreateSetting creates a new setting
func (s *SettingsService) CreateSetting(setting *models.SystemSettings) error {
	// Validate value
	if err := s.validateSettingValue(setting.Type, setting.Value); err != nil {
		return err
	}

	if err := s.db.Create(setting).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errors.New("setting with this key already exists")
		}
		return fmt.Errorf("failed to create setting: %w", err)
	}

	return nil
}

// DeleteSetting deletes a setting
func (s *SettingsService) DeleteSetting(key string) error {
	result := s.db.Where("key = ?", key).Delete(&models.SystemSettings{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete setting: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("setting not found")
	}
	return nil
}

// GetDatabaseConfig gets database configuration
func (s *SettingsService) GetDatabaseConfig() (map[string]interface{}, error) {
	// Get database stats
	sqlDB, err := s.db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	stats := sqlDB.Stats()

	return map[string]interface{}{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration.String(),
	}, nil
}

// GetOCRConfig gets OCR configuration
func (s *SettingsService) GetOCRConfig() (map[string]interface{}, error) {
	config := make(map[string]interface{})

	// Get OCR related settings
	settings := []string{
		"tesseract_path",
		"ocr_language",
		"screenshot_quality",
		"ocr_confidence_threshold",
	}

	for _, key := range settings {
		if value, err := s.GetSettingValue(key); err == nil {
			config[key] = value
		}
	}

	return config, nil
}

// UpdateOCRConfig updates OCR configuration
func (s *SettingsService) UpdateOCRConfig(config map[string]interface{}) error {
	for key, value := range config {
		if err := s.UpdateSetting(key, value); err != nil {
			return fmt.Errorf("failed to update %s: %w", key, err)
		}
	}
	return nil
}

// GetCheckIntervals gets check interval settings
func (s *SettingsService) GetCheckIntervals() (map[string]interface{}, error) {
	intervals := make(map[string]interface{})

	// Get interval related settings
	settings := []string{
		"check_interval_minutes",
		"max_concurrent_checks",
		"retry_failed_checks",
		"retry_delay_minutes",
	}

	for _, key := range settings {
		if value, err := s.GetSettingValue(key); err == nil {
			intervals[key] = value
		}
	}

	return intervals, nil
}

// validateSettingValue validates setting value based on type
func (s *SettingsService) validateSettingValue(settingType, value string) error {
	switch settingType {
	case "int":
		_, err := strconv.Atoi(value)
		if err != nil {
			return errors.New("value must be a valid integer")
		}
	case "bool":
		_, err := strconv.ParseBool(value)
		if err != nil {
			return errors.New("value must be true or false")
		}
	case "float":
		_, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return errors.New("value must be a valid number")
		}
	case "json":
		var temp interface{}
		if err := json.Unmarshal([]byte(value), &temp); err != nil {
			return errors.New("value must be valid JSON")
		}
	}
	return nil
}

// GetSettingsGroups returns settings grouped by category
func (s *SettingsService) GetSettingsGroups() (map[string][]models.SystemSettings, error) {
	settings, err := s.GetAllSettings()
	if err != nil {
		return nil, err
	}

	groups := make(map[string][]models.SystemSettings)
	for _, setting := range settings {
		groups[setting.Category] = append(groups[setting.Category], setting)
	}

	return groups, nil
}

// ImportSettings imports settings from JSON
func (s *SettingsService) ImportSettings(data []byte) error {
	var settings []models.SystemSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("failed to parse settings: %w", err)
	}

	for _, setting := range settings {
		// Validate setting
		if err := s.validateSettingValue(setting.Type, setting.Value); err != nil {
			return fmt.Errorf("invalid value for %s: %w", setting.Key, err)
		}

		// Update or create setting
		_, err := s.GetSetting(setting.Key)
		if err == nil {
			// Update existing
			if err := s.UpdateSetting(setting.Key, setting.Value); err != nil {
				return fmt.Errorf("failed to update %s: %w", setting.Key, err)
			}
		} else {
			// Create new
			if err := s.CreateSetting(&setting); err != nil {
				return fmt.Errorf("failed to create %s: %w", setting.Key, err)
			}
		}
	}

	return nil
}

// ExportSettings exports all settings to JSON
func (s *SettingsService) ExportSettings() ([]byte, error) {
	settings, err := s.GetAllSettings()
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(settings, "", "  ")
}
