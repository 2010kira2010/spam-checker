package services

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"spam-checker/internal/config"
	"spam-checker/internal/models"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type CheckService struct {
	db         *gorm.DB
	cfg        *config.Config
	adbService *ADBService
}

// GetDB returns database instance for handlers
func (s *CheckService) GetDB() *gorm.DB {
	return s.db
}

func NewCheckService(db *gorm.DB, cfg *config.Config) *CheckService {
	return &CheckService{
		db:         db,
		cfg:        cfg,
		adbService: NewADBService(db),
	}
}

// CheckPhoneNumber checks a single phone number across all services
func (s *CheckService) CheckPhoneNumber(phoneID uint) error {
	// Get phone number
	var phone models.PhoneNumber
	if err := s.db.First(&phone, phoneID).Error; err != nil {
		return fmt.Errorf("phone not found: %w", err)
	}

	// Get active gateways
	gateways, err := s.adbService.GetActiveGateways()
	if err != nil {
		return fmt.Errorf("failed to get active gateways: %w", err)
	}

	// Check on each gateway
	for _, gateway := range gateways {
		if err := s.checkOnGateway(&phone, &gateway); err != nil {
			logrus.Errorf("Failed to check phone %s on gateway %s: %v", phone.Number, gateway.Name, err)
			continue
		}
	}

	return nil
}

// CheckAllPhones checks all active phone numbers
func (s *CheckService) CheckAllPhones() error {
	phones, err := NewPhoneService(s.db).GetActivePhones()
	if err != nil {
		return fmt.Errorf("failed to get active phones: %w", err)
	}

	logrus.Infof("Starting check for %d phones", len(phones))

	for _, phone := range phones {
		if err := s.CheckPhoneNumber(phone.ID); err != nil {
			logrus.Errorf("Failed to check phone %s: %v", phone.Number, err)
			continue
		}

		// Small delay between checks
		time.Sleep(2 * time.Second)
	}

	return nil
}

// checkOnGateway checks phone on specific gateway
func (s *CheckService) checkOnGateway(phone *models.PhoneNumber, gateway *models.ADBGateway) error {
	// Get service info
	var service models.SpamService
	if err := s.db.Where("code = ?", gateway.ServiceCode).First(&service).Error; err != nil {
		return fmt.Errorf("service not found: %w", err)
	}

	logrus.Infof("Checking %s on %s", phone.Number, service.Name)

	// Connect to ADB
	adbCmd := fmt.Sprintf("adb connect %s:%d", gateway.Host, gateway.Port)
	if err := exec.Command("sh", "-c", adbCmd).Run(); err != nil {
		return fmt.Errorf("failed to connect to ADB: %w", err)
	}

	// Open app based on service
	appPackage, appActivity := s.getAppInfo(gateway.ServiceCode)
	openCmd := fmt.Sprintf("adb -s %s:%d shell am start -n %s/%s",
		gateway.Host, gateway.Port, appPackage, appActivity)
	if err := exec.Command("sh", "-c", openCmd).Run(); err != nil {
		return fmt.Errorf("failed to open app: %w", err)
	}

	time.Sleep(3 * time.Second)

	// Clear previous search
	clearCmd := fmt.Sprintf("adb -s %s:%d shell input keyevent KEYCODE_CLEAR", gateway.Host, gateway.Port)
	exec.Command("sh", "-c", clearCmd).Run()

	// Input phone number
	inputCmd := fmt.Sprintf("adb -s %s:%d shell input text %s", gateway.Host, gateway.Port, phone.Number)
	if err := exec.Command("sh", "-c", inputCmd).Run(); err != nil {
		return fmt.Errorf("failed to input phone number: %w", err)
	}

	// Press search
	searchCmd := fmt.Sprintf("adb -s %s:%d shell input keyevent KEYCODE_ENTER", gateway.Host, gateway.Port)
	if err := exec.Command("sh", "-c", searchCmd).Run(); err != nil {
		return fmt.Errorf("failed to press search: %w", err)
	}

	// Wait for results
	time.Sleep(5 * time.Second)

	// Take screenshot
	screenshot, err := s.takeScreenshot(gateway)
	if err != nil {
		return fmt.Errorf("failed to take screenshot: %w", err)
	}

	// Save screenshot
	screenshotPath, err := s.saveScreenshot(screenshot, phone.Number, service.Code)
	if err != nil {
		return fmt.Errorf("failed to save screenshot: %w", err)
	}

	// Perform OCR
	ocrText, err := s.performOCR(screenshotPath)
	if err != nil {
		return fmt.Errorf("failed to perform OCR: %w", err)
	}

	// Check for spam keywords
	isSpam, foundKeywords := s.checkForSpamKeywords(ocrText, service.ID)

	// Save result
	result := &models.CheckResult{
		PhoneNumberID: phone.ID,
		ServiceID:     service.ID,
		IsSpam:        isSpam,
		FoundKeywords: foundKeywords,
		Screenshot:    screenshotPath,
		RawText:       ocrText,
		CheckedAt:     time.Now(),
	}

	if err := s.db.Create(result).Error; err != nil {
		return fmt.Errorf("failed to save check result: %w", err)
	}

	// Update statistics
	s.updateStatistics(phone.ID, service.ID, isSpam)

	return nil
}

// takeScreenshot takes screenshot from Android device
func (s *CheckService) takeScreenshot(gateway *models.ADBGateway) ([]byte, error) {
	// Take screenshot
	screenshotCmd := fmt.Sprintf("adb -s %s:%d shell screencap -p", gateway.Host, gateway.Port)
	output, err := exec.Command("sh", "-c", screenshotCmd).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	// Convert output (remove carriage returns on Windows)
	screenshot := bytes.Replace(output, []byte("\r\n"), []byte("\n"), -1)

	return screenshot, nil
}

// saveScreenshot saves screenshot to file
func (s *CheckService) saveScreenshot(data []byte, phoneNumber, serviceCode string) (string, error) {
	// Create screenshots directory
	dir := filepath.Join("screenshots", serviceCode)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate filename
	filename := fmt.Sprintf("%s_%s_%d.png", phoneNumber, serviceCode, time.Now().Unix())
	path := filepath.Join(dir, filename)

	// Decode and save image
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		// If decoding fails, save raw data
		if err := os.WriteFile(path, data, 0644); err != nil {
			return "", fmt.Errorf("failed to save screenshot: %w", err)
		}
		return path, nil
	}

	// Save as PNG
	file, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return "", fmt.Errorf("failed to encode image: %w", err)
	}

	return path, nil
}

// performOCR performs OCR on screenshot
func (s *CheckService) performOCR(imagePath string) (string, error) {
	// Prepare Tesseract command
	cmd := exec.Command(s.cfg.OCR.TesseractPath, imagePath, "stdout", "-l", s.cfg.OCR.Language)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("OCR failed: %w", err)
	}

	return string(output), nil
}

// checkForSpamKeywords checks if text contains spam keywords
func (s *CheckService) checkForSpamKeywords(text string, serviceID uint) (bool, []string) {
	text = strings.ToLower(text)
	var foundKeywords []string

	// Get keywords
	var keywords []models.SpamKeyword
	query := s.db.Where("is_active = ?", true)
	query = query.Where("service_id IS NULL OR service_id = ?", serviceID)

	if err := query.Find(&keywords).Error; err != nil {
		logrus.Errorf("Failed to get spam keywords: %v", err)
		return false, foundKeywords
	}

	// Check each keyword
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword.Keyword)) {
			foundKeywords = append(foundKeywords, keyword.Keyword)
		}
	}

	return len(foundKeywords) > 0, foundKeywords
}

// updateStatistics updates check statistics
func (s *CheckService) updateStatistics(phoneID, serviceID uint, isSpam bool) {
	var stats models.Statistics

	// Try to find existing statistics
	err := s.db.Where("phone_number_id = ? AND service_id = ?", phoneID, serviceID).First(&stats).Error

	if err == gorm.ErrRecordNotFound {
		// Create new statistics
		stats = models.Statistics{
			PhoneNumberID: phoneID,
			ServiceID:     serviceID,
			TotalChecks:   1,
			LastCheckDate: time.Now(),
		}
		if isSpam {
			stats.SpamCount = 1
			now := time.Now()
			stats.FirstSpamDate = &now
		}
		s.db.Create(&stats)
	} else if err == nil {
		// Update existing statistics
		stats.TotalChecks++
		stats.LastCheckDate = time.Now()
		if isSpam {
			stats.SpamCount++
			if stats.FirstSpamDate == nil {
				now := time.Now()
				stats.FirstSpamDate = &now
			}
		}
		s.db.Save(&stats)
	}
}

// getAppInfo returns package and activity for service
func (s *CheckService) getAppInfo(serviceCode string) (string, string) {
	switch serviceCode {
	case "yandex_aon":
		return "ru.yandex.whocalls", "ru.yandex.whocalls.MainActivity"
	case "kaspersky":
		return "com.kaspersky.whocalls", "com.kaspersky.whocalls.MainActivity"
	case "getcontact":
		return "app.source.getcontact", "app.source.getcontact.MainActivity"
	default:
		return "", ""
	}
}

// GetCheckResults gets check results with filters
func (s *CheckService) GetCheckResults(phoneID uint, serviceID uint, limit int) ([]models.CheckResult, error) {
	var results []models.CheckResult

	query := s.db.Preload("Service")

	if phoneID > 0 {
		query = query.Where("phone_number_id = ?", phoneID)
	}

	if serviceID > 0 {
		query = query.Where("service_id = ?", serviceID)
	}

	if err := query.Order("checked_at DESC").Limit(limit).Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get check results: %w", err)
	}

	return results, nil
}

// GetLatestResults gets latest results for all phones
func (s *CheckService) GetLatestResults() ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	query := `
		SELECT DISTINCT ON (cr.phone_number_id, cr.service_id)
			pn.id as phone_id,
			pn.number as phone_number,
			pn.description,
			ss.id as service_id,
			ss.name as service_name,
			cr.is_spam,
			cr.found_keywords,
			cr.checked_at
		FROM check_results cr
		JOIN phone_numbers pn ON pn.id = cr.phone_number_id
		JOIN spam_services ss ON ss.id = cr.service_id
		WHERE pn.deleted_at IS NULL
		ORDER BY cr.phone_number_id, cr.service_id, cr.checked_at DESC
	`

	if err := s.db.Raw(query).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get latest results: %w", err)
	}

	return results, nil
}

// CheckPhoneRealtime checks phone number in real-time
func (s *CheckService) CheckPhoneRealtime(phoneNumber string) (map[string]interface{}, error) {
	// Normalize phone number
	phoneNumber = NewPhoneService(s.db).normalizePhoneNumber(phoneNumber)

	// Create temporary phone record
	tempPhone := &models.PhoneNumber{
		Number:   phoneNumber,
		IsActive: false, // Don't save for scheduled checks
	}

	// Get active gateways
	gateways, err := s.adbService.GetActiveGateways()
	if err != nil {
		return nil, fmt.Errorf("failed to get active gateways: %w", err)
	}

	results := make(map[string]interface{})
	results["phone_number"] = phoneNumber
	results["checked_at"] = time.Now()

	var serviceResults []map[string]interface{}

	// Check on each gateway
	for _, gateway := range gateways {
		// Get service info
		var service models.SpamService
		if err := s.db.Where("code = ?", gateway.ServiceCode).First(&service).Error; err != nil {
			continue
		}

		// Perform check
		if err := s.checkOnGateway(tempPhone, &gateway); err != nil {
			serviceResults = append(serviceResults, map[string]interface{}{
				"service": service.Name,
				"error":   err.Error(),
			})
			continue
		}

		// Get result
		var checkResult models.CheckResult
		if err := s.db.Where("phone_number_id = ? AND service_id = ?", tempPhone.ID, service.ID).
			Order("checked_at DESC").First(&checkResult).Error; err == nil {
			serviceResults = append(serviceResults, map[string]interface{}{
				"service":        service.Name,
				"is_spam":        checkResult.IsSpam,
				"found_keywords": checkResult.FoundKeywords,
			})
		}
	}

	results["results"] = serviceResults

	return results, nil
}
