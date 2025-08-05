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
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type CheckService struct {
	db         *gorm.DB
	cfg        *config.Config
	adbService *ADBService
}

// CheckResult for concurrent processing
type ConcurrentCheckResult struct {
	PhoneID uint
	Gateway *models.ADBGateway
	Service *models.SpamService
	Error   error
	Result  *models.CheckResult
}

// GetDB returns database instance for handlers
func (s *CheckService) GetDB() *gorm.DB {
	return s.db
}

func NewCheckService(db *gorm.DB, cfg *config.Config) *CheckService {
	return &CheckService{
		db:         db,
		cfg:        cfg,
		adbService: NewADBServiceWithConfig(db, cfg),
	}
}

// CheckPhoneNumber checks a single phone number across all services concurrently
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

	if len(gateways) == 0 {
		return fmt.Errorf("no active gateways available")
	}

	logrus.Infof("Starting concurrent check for phone %s across %d gateways", phone.Number, len(gateways))

	// Create a channel for results
	resultChan := make(chan ConcurrentCheckResult, len(gateways))
	var wg sync.WaitGroup

	// Check on each gateway concurrently
	for _, gateway := range gateways {
		wg.Add(1)
		go func(gw models.ADBGateway) {
			defer wg.Done()

			// Get service info
			var service models.SpamService
			if err := s.db.Where("code = ?", gw.ServiceCode).First(&service).Error; err != nil {
				resultChan <- ConcurrentCheckResult{
					PhoneID: phone.ID,
					Gateway: &gw,
					Error:   fmt.Errorf("service not found: %w", err),
				}
				return
			}

			// Perform check
			result := ConcurrentCheckResult{
				PhoneID: phone.ID,
				Gateway: &gw,
				Service: &service,
			}

			if err := s.checkOnGateway(&phone, &gw); err != nil {
				result.Error = err
				logrus.Errorf("Failed to check phone %s on gateway %s: %v", phone.Number, gw.Name, err)
			} else {
				// Get the created result
				var checkResult models.CheckResult
				if err := s.db.Where("phone_number_id = ? AND service_id = ?", phone.ID, service.ID).
					Order("checked_at DESC").First(&checkResult).Error; err == nil {
					result.Result = &checkResult
				}
			}

			resultChan <- result
		}(gateway)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	successCount := 0
	errorCount := 0
	for result := range resultChan {
		if result.Error != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	logrus.Infof("Concurrent check completed for phone %s: %d successful, %d failed",
		phone.Number, successCount, errorCount)

	if successCount == 0 && errorCount > 0 {
		return fmt.Errorf("all checks failed for phone %s", phone.Number)
	}

	return nil
}

// CheckAllPhones checks all active phone numbers with concurrent processing
func (s *CheckService) CheckAllPhones() error {
	phones, err := NewPhoneService(s.db).GetActivePhones()
	if err != nil {
		return fmt.Errorf("failed to get active phones: %w", err)
	}

	if len(phones) == 0 {
		logrus.Info("No active phones to check")
		return nil
	}

	// Get max concurrent checks setting
	var maxConcurrent int = 3 // default
	if setting, err := NewSettingsService(s.db).GetSettingValue("max_concurrent_checks"); err == nil {
		if val, ok := setting.(int); ok && val > 0 {
			maxConcurrent = val
		}
	}

	logrus.Infof("Starting check for %d phones with max %d concurrent checks", len(phones), maxConcurrent)

	// Create semaphore channel to limit concurrent phone checks
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, phone := range phones {
		wg.Add(1)
		go func(p models.PhoneNumber) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := s.CheckPhoneNumber(p.ID); err != nil {
				logrus.Errorf("Failed to check phone %s: %v", p.Number, err)
			}

			// Small delay between phone checks to avoid overwhelming the system
			time.Sleep(1 * time.Second)
		}(phone)
	}

	wg.Wait()
	logrus.Info("All phone checks completed")

	return nil
}

// checkOnGateway checks phone on specific gateway (unchanged from original)
func (s *CheckService) checkOnGateway(phone *models.PhoneNumber, gateway *models.ADBGateway) error {
	// Get service info
	var service models.SpamService
	if err := s.db.Where("code = ?", gateway.ServiceCode).First(&service).Error; err != nil {
		return fmt.Errorf("service not found: %w", err)
	}

	logrus.Infof("Checking %s on %s (gateway: %s)", phone.Number, service.Name, gateway.Name)

	// Since apps are pre-installed, we just need to ensure the app is running
	appPackage, appActivity := s.getAppInfo(gateway.ServiceCode)
	if appPackage != "" && appActivity != "" {
		// Try to start the app (it may already be running)
		if err := s.adbService.StartApp(gateway.ID, appPackage, appActivity); err != nil {
			logrus.Warnf("Failed to start app (it may already be running): %v", err)
		}
		time.Sleep(2 * time.Second)
	}

	// Simulate incoming call
	logrus.Infof("Simulating incoming call from %s", phone.Number)
	if err := s.adbService.SimulateIncomingCall(gateway.ID, phone.Number); err != nil {
		return fmt.Errorf("failed to simulate incoming call: %w", err)
	}

	// Wait for the service to process the call
	time.Sleep(5 * time.Second)

	// Take screenshot
	screenshot, err := s.adbService.TakeScreenshot(gateway.ID)
	if err != nil {
		logrus.Errorf("Failed to take screenshot: %v", err)
		// Continue with empty screenshot
		screenshot = []byte{}
	}

	// End the call
	if err := s.adbService.EndCall(gateway.ID); err != nil {
		logrus.Warnf("Failed to end call: %v", err)
	}

	// Save screenshot if we got one
	var screenshotPath string
	if len(screenshot) > 0 {
		screenshotPath, err = s.saveScreenshot(screenshot, phone.Number, service.Code)
		if err != nil {
			logrus.Errorf("Failed to save screenshot: %v", err)
			screenshotPath = ""
		}
	}

	// Perform OCR if we have a screenshot
	var ocrText string
	if screenshotPath != "" {
		ocrText, err = s.performOCR(screenshotPath)
		if err != nil {
			logrus.Errorf("Failed to perform OCR: %v", err)
			ocrText = ""
		}
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

	logrus.Infof("Check completed for %s on %s: isSpam=%v, keywords=%v",
		phone.Number, service.Name, isSpam, foundKeywords)

	return nil
}

// CheckPhoneNumbersInBatch checks multiple phone numbers concurrently
func (s *CheckService) CheckPhoneNumbersInBatch(phoneIDs []uint) error {
	if len(phoneIDs) == 0 {
		return nil
	}

	// Get max concurrent checks setting
	var maxConcurrent int = 3
	if setting, err := NewSettingsService(s.db).GetSettingValue("max_concurrent_checks"); err == nil {
		if val, ok := setting.(int); ok && val > 0 {
			maxConcurrent = val
		}
	}

	logrus.Infof("Starting batch check for %d phones with max %d concurrent", len(phoneIDs), maxConcurrent)

	// Create semaphore channel
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	errorChan := make(chan error, len(phoneIDs))

	for _, phoneID := range phoneIDs {
		wg.Add(1)
		go func(id uint) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := s.CheckPhoneNumber(id); err != nil {
				errorChan <- fmt.Errorf("phone %d: %w", id, err)
			}
		}(phoneID)
	}

	// Wait for all checks to complete
	wg.Wait()
	close(errorChan)

	// Collect errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("batch check completed with %d errors", len(errors))
	}

	return nil
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

	// Try to decode and save as PNG
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

// CheckPhoneRealtime checks phone number in real-time with concurrent processing
func (s *CheckService) CheckPhoneRealtime(phoneNumber string) (map[string]interface{}, error) {
	// Normalize phone number
	phoneNumber = NewPhoneService(s.db).normalizePhoneNumber(phoneNumber)

	// Create temporary phone record
	tempPhone := &models.PhoneNumber{
		Number:   phoneNumber,
		IsActive: false, // Don't save for scheduled checks
	}

	// Save temporarily to get ID
	if err := s.db.Create(tempPhone).Error; err != nil {
		return nil, fmt.Errorf("failed to create temporary phone record: %w", err)
	}

	// Ensure cleanup
	defer func() {
		// Delete temporary phone record and its results
		s.db.Where("phone_number_id = ?", tempPhone.ID).Delete(&models.CheckResult{})
		s.db.Delete(tempPhone)
	}()

	// Get active gateways
	gateways, err := s.adbService.GetActiveGateways()
	if err != nil {
		return nil, fmt.Errorf("failed to get active gateways: %w", err)
	}

	results := make(map[string]interface{})
	results["phone_number"] = phoneNumber
	results["checked_at"] = time.Now()

	var serviceResults []map[string]interface{}
	resultChan := make(chan map[string]interface{}, len(gateways))
	var wg sync.WaitGroup

	// Check on each gateway concurrently
	for _, gateway := range gateways {
		wg.Add(1)
		go func(gw models.ADBGateway) {
			defer wg.Done()

			// Get service info
			var service models.SpamService
			if err := s.db.Where("code = ?", gw.ServiceCode).First(&service).Error; err != nil {
				resultChan <- map[string]interface{}{
					"service": "Unknown",
					"error":   err.Error(),
				}
				return
			}

			// Perform check
			if err := s.checkOnGateway(tempPhone, &gw); err != nil {
				resultChan <- map[string]interface{}{
					"service": service.Name,
					"error":   err.Error(),
				}
				return
			}

			// Get result
			var checkResult models.CheckResult
			if err := s.db.Where("phone_number_id = ? AND service_id = ?", tempPhone.ID, service.ID).
				Order("checked_at DESC").First(&checkResult).Error; err == nil {
				resultChan <- map[string]interface{}{
					"service":        service.Name,
					"is_spam":        checkResult.IsSpam,
					"found_keywords": checkResult.FoundKeywords,
				}
			} else {
				resultChan <- map[string]interface{}{
					"service": service.Name,
					"error":   "Result not found",
				}
			}
		}(gateway)
	}

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		serviceResults = append(serviceResults, result)
	}

	results["results"] = serviceResults

	return results, nil
}
