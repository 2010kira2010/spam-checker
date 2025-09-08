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

// PhoneWithResults represents phone with its check results
type PhoneWithResults struct {
	models.PhoneNumber
	LatestResults []models.CheckResult `json:"latest_results"`
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
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "duplicate key") {
			return errors.New("phone number already exists")
		}
		return fmt.Errorf("failed to create phone number: %w", err)
	}

	return nil
}

// GetPhoneByID gets phone by ID with latest check results
func (s *PhoneService) GetPhoneByID(id uint) (*models.PhoneNumber, error) {
	var phone models.PhoneNumber

	// First get the phone
	if err := s.db.First(&phone, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("phone number not found")
		}
		return nil, fmt.Errorf("failed to get phone number: %w", err)
	}

	// Then load latest check results separately
	var checkResults []models.CheckResult
	err := s.db.Where("phone_number_id = ?", id).
		Order("checked_at DESC").
		Limit(10).
		Preload("Service").
		Find(&checkResults).Error

	if err != nil {
		s.log.Errorf("Failed to load check results for phone %d: %v", id, err)
		// Don't fail the whole request if we can't load results
		phone.CheckResults = []models.CheckResult{}
	} else {
		phone.CheckResults = checkResults
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

// ListPhones lists all phones with pagination and latest check results
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

	// Get phones
	if err := query.
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&phones).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list phones: %w", err)
	}

	// For each phone, load the latest check results
	for i := range phones {
		var latestResults []models.CheckResult

		// Get latest result for each service
		subQuery := s.db.Model(&models.CheckResult{}).
			Select("MAX(id) as id").
			Where("phone_number_id = ?", phones[i].ID).
			Group("service_id")

		err := s.db.
			Where("id IN (?)", subQuery).
			Preload("Service").
			Order("checked_at DESC").
			Find(&latestResults).Error

		if err != nil {
			s.log.Errorf("Failed to load check results for phone %d: %v", phones[i].ID, err)
			phones[i].CheckResults = []models.CheckResult{}
		} else {
			phones[i].CheckResults = latestResults
		}
	}

	return phones, total, nil
}

// ListPhonesWithDetails returns phones with additional computed fields
func (s *PhoneService) ListPhonesWithDetails(offset, limit int, search string, isActive *bool) ([]map[string]interface{}, int64, error) {
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

	// Get phones
	if err := query.
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&phones).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list phones: %w", err)
	}

	// Build detailed response
	results := make([]map[string]interface{}, len(phones))

	for i, phone := range phones {
		phoneData := map[string]interface{}{
			"id":          phone.ID,
			"number":      phone.Number,
			"description": phone.Description,
			"is_active":   phone.IsActive,
			"created_by":  phone.CreatedBy,
			"created_at":  phone.CreatedAt,
			"updated_at":  phone.UpdatedAt,
		}

		// Get latest check results with service details
		var checkResults []struct {
			ServiceID     uint   `json:"service_id"`
			ServiceName   string `json:"service_name"`
			ServiceCode   string `json:"service_code"`
			IsSpam        bool   `json:"is_spam"`
			FoundKeywords string `json:"found_keywords"`
			CheckedAt     string `json:"checked_at"`
		}

		err := s.db.Table("check_results").
			Select(`
				check_results.service_id,
				spam_services.name as service_name,
				spam_services.code as service_code,
				check_results.is_spam,
				check_results.found_keywords,
				check_results.checked_at
			`).
			Joins("JOIN spam_services ON spam_services.id = check_results.service_id").
			Where("check_results.phone_number_id = ?", phone.ID).
			Where(`check_results.id IN (
				SELECT MAX(id) FROM check_results 
				WHERE phone_number_id = ? 
				GROUP BY service_id
			)`, phone.ID).
			Order("check_results.checked_at DESC").
			Scan(&checkResults).Error

		if err != nil {
			s.log.Errorf("Failed to get check results for phone %d: %v", phone.ID, err)
			phoneData["check_results"] = []interface{}{}
		} else {
			// Convert to proper format
			formattedResults := make([]map[string]interface{}, len(checkResults))
			for j, result := range checkResults {
				// Parse keywords
				var keywords []string
				if result.FoundKeywords != "" && result.FoundKeywords != "{}" {
					// Handle PostgreSQL array format
					keywordsStr := strings.Trim(result.FoundKeywords, "{}")
					if keywordsStr != "" {
						keywords = strings.Split(keywordsStr, ",")
						// Clean up quotes
						for k := range keywords {
							keywords[k] = strings.Trim(keywords[k], `"`)
						}
					}
				}

				formattedResults[j] = map[string]interface{}{
					"service": map[string]interface{}{
						"id":   result.ServiceID,
						"name": result.ServiceName,
						"code": result.ServiceCode,
					},
					"is_spam":        result.IsSpam,
					"found_keywords": keywords,
					"checked_at":     result.CheckedAt,
				}
			}
			phoneData["check_results"] = formattedResults
		}

		// Get overall spam status
		var spamCount int64
		s.db.Model(&models.CheckResult{}).
			Where("phone_number_id = ? AND is_spam = ?", phone.ID, true).
			Where(`id IN (
				SELECT MAX(id) FROM check_results 
				WHERE phone_number_id = ? 
				GROUP BY service_id
			)`, phone.ID).
			Count(&spamCount)

		phoneData["is_spam"] = spamCount > 0
		phoneData["spam_services_count"] = spamCount

		results[i] = phoneData
	}

	return results, total, nil
}

// UpdatePhone updates phone information
func (s *PhoneService) UpdatePhone(id uint, updates map[string]interface{}) error {
	// Normalize phone number if it's being updated
	if number, ok := updates["number"].(string); ok {
		updates["number"] = s.normalizePhoneNumber(number)
	}

	if err := s.db.Model(&models.PhoneNumber{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "duplicate key") {
			return errors.New("phone number already exists")
		}
		return fmt.Errorf("failed to update phone: %w", err)
	}

	return nil
}

// DeletePhone soft deletes a phone
func (s *PhoneService) DeletePhone(id uint) error {
	// Start transaction
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Delete related check results first
		if err := tx.Where("phone_number_id = ?", id).Delete(&models.CheckResult{}).Error; err != nil {
			return fmt.Errorf("failed to delete check results: %w", err)
		}

		// Delete related statistics
		if err := tx.Where("phone_number_id = ?", id).Delete(&models.Statistics{}).Error; err != nil {
			return fmt.Errorf("failed to delete statistics: %w", err)
		}

		// Delete the phone
		if err := tx.Delete(&models.PhoneNumber{}, id).Error; err != nil {
			return fmt.Errorf("failed to delete phone: %w", err)
		}

		return nil
	})
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
		if col == "number" || col == "phone" || col == "phone_number" || col == "номер" || col == "телефон" {
			numberIdx = i
		} else if col == "description" || col == "desc" || col == "описание" || col == "name" || col == "имя" {
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
			errors = append(errors, fmt.Sprintf("Line %d (%s): %v", lineNum, number, err))
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
	if err := csvWriter.Write([]string{"Number", "Description", "Status", "Last Check", "Is Spam", "Services Checked"}); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Get all phones with pagination
	offset := 0
	limit := 100

	for {
		phones, _, err := s.ListPhonesWithDetails(offset, limit, "", isActive)
		if err != nil {
			return fmt.Errorf("failed to get phones: %w", err)
		}

		if len(phones) == 0 {
			break
		}

		// Write rows
		for _, phoneData := range phones {
			status := "Active"
			if active, ok := phoneData["is_active"].(bool); ok && !active {
				status = "Inactive"
			}

			lastCheck := "Never"
			isSpam := "Unknown"
			servicesChecked := 0

			if results, ok := phoneData["check_results"].([]map[string]interface{}); ok && len(results) > 0 {
				servicesChecked = len(results)

				// Get latest check time
				if checkedAt, ok := results[0]["checked_at"].(string); ok {
					lastCheck = checkedAt
				}

				// Get spam status
				if spamStatus, ok := phoneData["is_spam"].(bool); ok {
					if spamStatus {
						isSpam = "Yes"
					} else {
						isSpam = "No"
					}
				}
			}

			row := []string{
				phoneData["number"].(string),
				phoneData["description"].(string),
				status,
				lastCheck,
				isSpam,
				fmt.Sprintf("%d", servicesChecked),
			}

			if err := csvWriter.Write(row); err != nil {
				return fmt.Errorf("failed to write CSV row: %w", err)
			}
		}

		offset += limit
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
	var checkedPhones int64

	// Total phones
	if err := s.db.Model(&models.PhoneNumber{}).Count(&totalPhones).Error; err != nil {
		return nil, fmt.Errorf("failed to count total phones: %w", err)
	}

	// Active phones
	if err := s.db.Model(&models.PhoneNumber{}).Where("is_active = ?", true).Count(&activePhones).Error; err != nil {
		return nil, fmt.Errorf("failed to count active phones: %w", err)
	}

	// Phones with at least one check
	if err := s.db.Model(&models.PhoneNumber{}).
		Joins("JOIN check_results ON check_results.phone_number_id = phone_numbers.id").
		Distinct("phone_numbers.id").
		Count(&checkedPhones).Error; err != nil {
		return nil, fmt.Errorf("failed to count checked phones: %w", err)
	}

	// Phones marked as spam (at least one service detected spam in latest check)
	query := `
		SELECT COUNT(DISTINCT phone_numbers.id)
		FROM phone_numbers
		JOIN check_results cr1 ON cr1.phone_number_id = phone_numbers.id
		WHERE cr1.is_spam = true
		AND cr1.id IN (
			SELECT MAX(cr2.id)
			FROM check_results cr2
			WHERE cr2.phone_number_id = cr1.phone_number_id
			GROUP BY cr2.service_id
		)
		AND phone_numbers.deleted_at IS NULL
	`

	if err := s.db.Raw(query).Scan(&spamPhones).Error; err != nil {
		return nil, fmt.Errorf("failed to count spam phones: %w", err)
	}

	return map[string]interface{}{
		"total_phones":     totalPhones,
		"active_phones":    activePhones,
		"checked_phones":   checkedPhones,
		"spam_phones":      spamPhones,
		"clean_phones":     checkedPhones - spamPhones,
		"unchecked_phones": totalPhones - checkedPhones,
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

// GetPhonesWithLatestResults gets phones with their latest check results efficiently
func (s *PhoneService) GetPhonesWithLatestResults(phoneIDs []uint) ([]models.PhoneNumber, error) {
	var phones []models.PhoneNumber

	// Get phones
	if err := s.db.Where("id IN ?", phoneIDs).Find(&phones).Error; err != nil {
		return nil, fmt.Errorf("failed to get phones: %w", err)
	}

	// Create map for quick lookup
	phoneMap := make(map[uint]*models.PhoneNumber)
	for i := range phones {
		phoneMap[phones[i].ID] = &phones[i]
		phones[i].CheckResults = []models.CheckResult{}
	}

	// Get latest check results for all phones at once
	var results []models.CheckResult
	subQuery := s.db.Model(&models.CheckResult{}).
		Select("phone_number_id, service_id, MAX(id) as max_id").
		Where("phone_number_id IN ?", phoneIDs).
		Group("phone_number_id, service_id")

	err := s.db.
		Joins("JOIN (?) as latest ON check_results.id = latest.max_id", subQuery).
		Preload("Service").
		Order("checked_at DESC").
		Find(&results).Error

	if err != nil {
		s.log.Errorf("Failed to load check results: %v", err)
		return phones, nil
	}

	// Assign results to phones
	for _, result := range results {
		if phone, exists := phoneMap[result.PhoneNumberID]; exists {
			phone.CheckResults = append(phone.CheckResults, result)
		}
	}

	return phones, nil
}
