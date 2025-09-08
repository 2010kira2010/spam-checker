package services

import (
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"spam-checker/internal/logger"
	"spam-checker/internal/models"
	"strings"

	"gorm.io/gorm"
)

type PhoneService struct {
	db  *gorm.DB
	log *logrus.Entry
}

func NewPhoneService(db *gorm.DB) *PhoneService {
	return &PhoneService{
		db:  db,
		log: logger.WithField("service", "PhoneService"),
	}
}

// CreatePhone creates a new phone number
func (s *PhoneService) CreatePhone(phone *models.PhoneNumber) error {
	// Normalize phone number
	phone.Number = s.normalizePhoneNumber(phone.Number)

	if err := s.db.Create(phone).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errors.New("phone number already exists")
		}
		return fmt.Errorf("failed to create phone number: %w", err)
	}

	return nil
}

// GetPhoneByID gets phone by ID
func (s *PhoneService) GetPhoneByID(id uint) (*models.PhoneNumber, error) {
	var phone models.PhoneNumber
	if err := s.db.Preload("CheckResults.Service").First(&phone, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("phone number not found")
		}
		return nil, fmt.Errorf("failed to get phone number: %w", err)
	}
	return &phone, nil
}

// GetPhoneByNumber gets phone by number
func (s *PhoneService) GetPhoneByNumber(number string) (*models.PhoneNumber, error) {
	number = s.normalizePhoneNumber(number)
	var phone models.PhoneNumber
	if err := s.db.Where("number = ?", number).First(&phone).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("phone number not found")
		}
		return nil, fmt.Errorf("failed to get phone number: %w", err)
	}
	return &phone, nil
}

// ListPhones lists all phones with pagination
func (s *PhoneService) ListPhones(offset, limit int, search string, isActive *bool) ([]models.PhoneNumber, int64, error) {
	var phones []models.PhoneNumber
	var total int64

	query := s.db.Model(&models.PhoneNumber{})

	// Apply filters
	if search != "" {
		search = "%" + search + "%"
		query = query.Where("number LIKE ? OR description LIKE ?", search, search)
	}

	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count phones: %w", err)
	}

	// Get phones with latest check results
	if err := query.
		Preload("CheckResults", func(db *gorm.DB) *gorm.DB {
			return db.Order("checked_at DESC").Limit(1)
		}).
		Preload("CheckResults.Service").
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&phones).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list phones: %w", err)
	}

	return phones, total, nil
}

// UpdatePhone updates phone information
func (s *PhoneService) UpdatePhone(id uint, updates map[string]interface{}) error {
	// Normalize phone number if it's being updated
	if number, ok := updates["number"].(string); ok {
		updates["number"] = s.normalizePhoneNumber(number)
	}

	if err := s.db.Model(&models.PhoneNumber{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errors.New("phone number already exists")
		}
		return fmt.Errorf("failed to update phone: %w", err)
	}

	return nil
}

// DeletePhone soft deletes a phone
func (s *PhoneService) DeletePhone(id uint) error {
	if err := s.db.Delete(&models.PhoneNumber{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete phone: %w", err)
	}
	return nil
}

// ImportPhones imports phones from CSV
func (s *PhoneService) ImportPhones(reader io.Reader, userID uint) (int, []string, error) {
	csvReader := csv.NewReader(reader)

	// Read header
	header, err := csvReader.Read()
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Find column indices
	numberIdx := -1
	descriptionIdx := -1
	for i, col := range header {
		col = strings.ToLower(strings.TrimSpace(col))
		if col == "number" || col == "phone" || col == "phone_number" || col == "номер" {
			numberIdx = i
		} else if col == "description" || col == "desc" || col == "описание" {
			descriptionIdx = i
		}
	}

	if numberIdx == -1 {
		return 0, nil, errors.New("phone number column not found in CSV")
	}

	imported := 0
	var errors []string

	// Read rows
	for lineNum := 2; ; lineNum++ {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errors = append(errors, fmt.Sprintf("Line %d: %v", lineNum, err))
			continue
		}

		if len(record) <= numberIdx {
			errors = append(errors, fmt.Sprintf("Line %d: insufficient columns", lineNum))
			continue
		}

		number := strings.TrimSpace(record[numberIdx])
		if number == "" {
			errors = append(errors, fmt.Sprintf("Line %d: empty phone number", lineNum))
			continue
		}

		description := ""
		if descriptionIdx != -1 && len(record) > descriptionIdx {
			description = strings.TrimSpace(record[descriptionIdx])
		}

		phone := &models.PhoneNumber{
			Number:      number,
			Description: description,
			CreatedBy:   userID,
			IsActive:    true,
		}

		if err := s.CreatePhone(phone); err != nil {
			errors = append(errors, fmt.Sprintf("Line %d: %v", lineNum, err))
			continue
		}

		imported++
	}

	return imported, errors, nil
}

// ExportPhones exports phones to CSV
func (s *PhoneService) ExportPhones(writer io.Writer, isActive *bool) error {
	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	// Write header
	if err := csvWriter.Write([]string{"Number", "Description", "Status", "Last Check", "Is Spam"}); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Get all phones
	query := s.db.Model(&models.PhoneNumber{})
	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}

	var phones []models.PhoneNumber
	if err := query.
		Preload("CheckResults", func(db *gorm.DB) *gorm.DB {
			return db.Order("checked_at DESC").Limit(1)
		}).
		Find(&phones).Error; err != nil {
		return fmt.Errorf("failed to get phones: %w", err)
	}

	// Write rows
	for _, phone := range phones {
		status := "Active"
		if !phone.IsActive {
			status = "Inactive"
		}

		lastCheck := "Never"
		isSpam := "Unknown"
		if len(phone.CheckResults) > 0 {
			lastCheck = phone.CheckResults[0].CheckedAt.Format("2006-01-02 15:04:05")
			if phone.CheckResults[0].IsSpam {
				isSpam = "Yes"
			} else {
				isSpam = "No"
			}
		}

		row := []string{
			phone.Number,
			phone.Description,
			status,
			lastCheck,
			isSpam,
		}

		if err := csvWriter.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// GetActivePhones gets all active phones for checking
func (s *PhoneService) GetActivePhones() ([]models.PhoneNumber, error) {
	var phones []models.PhoneNumber
	if err := s.db.Where("is_active = ?", true).Find(&phones).Error; err != nil {
		return nil, fmt.Errorf("failed to get active phones: %w", err)
	}
	return phones, nil
}

// GetPhoneStats gets phone statistics
func (s *PhoneService) GetPhoneStats() (map[string]interface{}, error) {
	var totalPhones int64
	var activePhones int64
	var spamPhones int64

	// Total phones
	if err := s.db.Model(&models.PhoneNumber{}).Count(&totalPhones).Error; err != nil {
		return nil, fmt.Errorf("failed to count total phones: %w", err)
	}

	// Active phones
	if err := s.db.Model(&models.PhoneNumber{}).Where("is_active = ?", true).Count(&activePhones).Error; err != nil {
		return nil, fmt.Errorf("failed to count active phones: %w", err)
	}

	// Phones marked as spam (at least one service)
	if err := s.db.Model(&models.PhoneNumber{}).
		Joins("JOIN check_results ON check_results.phone_number_id = phone_numbers.id").
		Where("check_results.is_spam = ? AND check_results.id IN (SELECT MAX(id) FROM check_results GROUP BY phone_number_id, service_id)", true).
		Distinct("phone_numbers.id").
		Count(&spamPhones).Error; err != nil {
		return nil, fmt.Errorf("failed to count spam phones: %w", err)
	}

	return map[string]interface{}{
		"total_phones":  totalPhones,
		"active_phones": activePhones,
		"spam_phones":   spamPhones,
		"clean_phones":  totalPhones - spamPhones,
	}, nil
}

// normalizePhoneNumber normalizes phone number format
func (s *PhoneService) normalizePhoneNumber(number string) string {
	// Remove all non-digit characters
	number = strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, number)

	// Add country code if missing (assuming Russia)
	if len(number) == 10 {
		number = "7" + number
	} else if len(number) == 11 && number[0] == '8' {
		number = "7" + number[1:]
	}

	return number
}
