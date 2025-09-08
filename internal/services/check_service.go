package services

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"spam-checker/internal/config"
	"spam-checker/internal/logger"
	"spam-checker/internal/models"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type CheckService struct {
	db               *gorm.DB
	cfg              *config.Config
	adbService       *ADBService
	apiService       *APICheckService
	gatewayLocks     map[uint]*sync.Mutex
	gatewayLocksMu   sync.RWMutex
	gatewayBusy      map[uint]bool // Track gateway busy state in memory
	phoneCheckLocks  map[uint]*sync.Mutex
	phoneCheckMu     sync.RWMutex
	resultWriteMutex sync.Mutex // Global mutex for writing results
	log              *logrus.Entry
}

// CheckResult for concurrent processing
type ConcurrentCheckResult struct {
	PhoneID uint
	Gateway *models.ADBGateway
	Service *models.SpamService
	Error   error
	Result  *models.CheckResult
}

// APICheckResult for concurrent API processing
type APICheckResult struct {
	PhoneID    uint
	APIService *models.APIService
	Service    *models.SpamService
	Error      error
	Result     *models.CheckResult
}

// GetDB returns database instance for handlers
func (s *CheckService) GetDB() *gorm.DB {
	return s.db
}

func NewCheckService(db *gorm.DB, cfg *config.Config) *CheckService {
	return &CheckService{
		db:              db,
		cfg:             cfg,
		adbService:      NewADBServiceWithConfig(db, cfg),
		apiService:      NewAPICheckService(db),
		gatewayLocks:    make(map[uint]*sync.Mutex),
		gatewayBusy:     make(map[uint]bool),
		phoneCheckLocks: make(map[uint]*sync.Mutex),
		log:             logger.WithField("service", "CheckService"),
	}
}

// getGatewayLock returns a lock for specific gateway
func (s *CheckService) getGatewayLock(gatewayID uint) *sync.Mutex {
	s.gatewayLocksMu.Lock()
	defer s.gatewayLocksMu.Unlock()

	if lock, exists := s.gatewayLocks[gatewayID]; exists {
		return lock
	}

	lock := &sync.Mutex{}
	s.gatewayLocks[gatewayID] = lock
	return lock
}

// getPhoneCheckLock returns a lock for specific phone's checks
func (s *CheckService) getPhoneCheckLock(phoneID uint) *sync.Mutex {
	s.phoneCheckMu.Lock()
	defer s.phoneCheckMu.Unlock()

	if lock, exists := s.phoneCheckLocks[phoneID]; exists {
		return lock
	}

	lock := &sync.Mutex{}
	s.phoneCheckLocks[phoneID] = lock
	return lock
}

// tryLockGateway attempts to lock a gateway without blocking
func (s *CheckService) tryLockGateway(gatewayID uint) bool {
	lock := s.getGatewayLock(gatewayID)
	if lock.TryLock() {
		// Mark gateway as busy in memory
		s.gatewayLocksMu.Lock()
		s.gatewayBusy[gatewayID] = true
		s.gatewayLocksMu.Unlock()
		return true
	}
	return false
}

// unlockGateway releases gateway lock
func (s *CheckService) unlockGateway(gatewayID uint) {
	lock := s.getGatewayLock(gatewayID)
	lock.Unlock()

	// Mark gateway as available in memory
	s.gatewayLocksMu.Lock()
	s.gatewayBusy[gatewayID] = false
	s.gatewayLocksMu.Unlock()
}

// isGatewayBusy checks if gateway is busy
func (s *CheckService) isGatewayBusy(gatewayID uint) bool {
	s.gatewayLocksMu.RLock()
	defer s.gatewayLocksMu.RUnlock()
	return s.gatewayBusy[gatewayID]
}

// CheckPhoneNumber checks a single phone number across all services
func (s *CheckService) CheckPhoneNumber(phoneID uint) error {
	log := s.log.WithFields(logrus.Fields{
		"method":  "CheckPhoneNumber",
		"phoneID": phoneID,
	})

	// Get phone number
	var phone models.PhoneNumber
	if err := s.db.First(&phone, phoneID).Error; err != nil {
		return fmt.Errorf("phone not found: %w", err)
	}

	// Get check mode setting
	checkMode := s.getCheckMode()

	log.Infof("Starting check for phone %s with mode: %s", phone.Number, checkMode)

	// Perform checks based on mode
	switch checkMode {
	case models.CheckModeADBOnly:
		return s.checkViaADB(&phone)
	case models.CheckModeAPIOnly:
		return s.checkViaAPI(&phone)
	case models.CheckModeBoth:
		// Check both ADB and API concurrently
		var wg sync.WaitGroup
		var adbErr, apiErr error

		wg.Add(2)
		go func() {
			defer wg.Done()
			adbErr = s.checkViaADB(&phone)
		}()
		go func() {
			defer wg.Done()
			apiErr = s.checkViaAPI(&phone)
		}()

		wg.Wait()

		// Return error only if both failed
		if adbErr != nil && apiErr != nil {
			return fmt.Errorf("both ADB and API checks failed: ADB: %v, API: %v", adbErr, apiErr)
		}
		return nil
	default:
		return fmt.Errorf("unknown check mode: %s", checkMode)
	}
}

// getCheckMode gets the check mode from settings
func (s *CheckService) getCheckMode() models.CheckMode {
	var setting models.SystemSettings
	if err := s.db.Where("key = ?", "check_mode").First(&setting).Error; err != nil {
		// Default to ADB only if setting not found
		return models.CheckModeADBOnly
	}
	return models.CheckMode(setting.Value)
}

// checkViaADB checks phone via ADB gateways with smart concurrency
func (s *CheckService) checkViaADB(phone *models.PhoneNumber) error {
	log := s.log.WithFields(logrus.Fields{
		"method": "checkViaADB",
		"phone":  phone.Number,
	})

	// Get active gateways
	gateways, err := s.adbService.GetActiveGateways()
	if err != nil {
		return fmt.Errorf("failed to get active gateways: %w", err)
	}

	if len(gateways) == 0 {
		return fmt.Errorf("no active ADB gateways available")
	}

	log.Infof("Starting ADB check for phone %s across %d gateways", phone.Number, len(gateways))

	// Create a channel for results
	resultChan := make(chan ConcurrentCheckResult, len(gateways))
	var wg sync.WaitGroup

	// Process each gateway concurrently
	for _, gateway := range gateways {
		wg.Add(1)
		go func(gw models.ADBGateway) {
			defer wg.Done()

			result := ConcurrentCheckResult{
				PhoneID: phone.ID,
				Gateway: &gw,
			}

			// Try to acquire gateway lock without blocking
			if !s.tryLockGateway(gw.ID) {
				log.Debugf("Gateway %s (ID: %d) is busy, skipping", gw.Name, gw.ID)
				result.Error = fmt.Errorf("gateway busy")
				resultChan <- result
				return
			}
			log.Debugf("Successfully locked gateway %s (ID: %d)", gw.Name, gw.ID)
			defer func() {
				s.unlockGateway(gw.ID)
				log.Debugf("Unlocked gateway %s (ID: %d)", gw.Name, gw.ID)
			}()

			// Get service info
			var service models.SpamService
			if err := s.db.Where("code = ?", gw.ServiceCode).First(&service).Error; err != nil {
				result.Error = fmt.Errorf("service not found: %w", err)
				resultChan <- result
				return
			}
			result.Service = &service

			// Perform check
			log.Infof("Checking phone %s on gateway %s", phone.Number, gw.Name)
			if err := s.checkOnGateway(phone, &gw); err != nil {
				result.Error = err
				log.Errorf("Failed to check phone %s on gateway %s: %v", phone.Number, gw.Name, err)
			} else {
				// Get the created result
				var checkResult models.CheckResult
				if err := s.db.Where("phone_number_id = ? AND service_id = ?", phone.ID, service.ID).
					Order("checked_at DESC").First(&checkResult).Error; err == nil {
					result.Result = &checkResult
				}
				log.Infof("Successfully checked phone %s on gateway %s", phone.Number, gw.Name)
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
	busyCount := 0
	for result := range resultChan {
		if result.Error != nil {
			if strings.Contains(result.Error.Error(), "gateway busy") {
				busyCount++
			} else {
				errorCount++
			}
		} else {
			successCount++
		}
	}

	log.Infof("ADB check completed for phone %s: %d successful, %d failed, %d busy",
		phone.Number, successCount, errorCount, busyCount)

	if successCount == 0 && errorCount > 0 {
		return fmt.Errorf("all ADB checks failed for phone %s", phone.Number)
	}

	return nil
}

// checkViaAPI checks phone via API services with proper synchronization
func (s *CheckService) checkViaAPI(phone *models.PhoneNumber) error {
	log := s.log.WithFields(logrus.Fields{
		"method": "checkViaAPI",
		"phone":  phone.Number,
	})

	// Get active API services
	apiServices, err := s.apiService.GetActiveAPIServices()
	if err != nil {
		return fmt.Errorf("failed to get active API services: %w", err)
	}

	if len(apiServices) == 0 {
		return fmt.Errorf("no active API services available")
	}

	log.Infof("Starting API check for phone %s across %d services", phone.Number, len(apiServices))

	// Create a channel for results
	resultChan := make(chan APICheckResult, len(apiServices))
	var wg sync.WaitGroup

	// Check on each API service concurrently
	for _, apiService := range apiServices {
		wg.Add(1)
		go func(api models.APIService) {
			defer wg.Done()

			result := APICheckResult{
				PhoneID:    phone.ID,
				APIService: &api,
			}

			// Get service info
			var service models.SpamService
			if err := s.db.Where("code = ?", api.ServiceCode).First(&service).Error; err != nil {
				result.Error = fmt.Errorf("service not found: %w", err)
				resultChan <- result
				return
			}
			result.Service = &service

			// Perform check
			log.Infof("Checking phone %s via API %s", phone.Number, api.Name)
			checkResult, err := s.apiService.CheckPhoneViaAPI(phone, &api)
			if err != nil {
				result.Error = err
				log.Errorf("Failed to check phone %s via API %s: %v", phone.Number, api.Name, err)
			} else {
				result.Result = checkResult
				// Update statistics with proper locking
				s.updateStatistics(phone.ID, service.ID, checkResult.IsSpam)
				log.Infof("Successfully checked phone %s via API %s", phone.Number, api.Name)
			}

			resultChan <- result
		}(apiService)
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

	log.Infof("API check completed for phone %s: %d successful, %d failed",
		phone.Number, successCount, errorCount)

	if successCount == 0 && errorCount > 0 {
		return fmt.Errorf("all API checks failed for phone %s", phone.Number)
	}

	return nil
}

// CheckAllPhones checks all active phone numbers with intelligent concurrency
func (s *CheckService) CheckAllPhones() error {
	log := s.log.WithFields(logrus.Fields{
		"method": "CheckAllPhones",
	})

	phones, err := NewPhoneService(s.db).GetActivePhones()
	if err != nil {
		return fmt.Errorf("failed to get active phones: %w", err)
	}

	if len(phones) == 0 {
		log.Info("No active phones to check")
		return nil
	}

	// Get max concurrent checks setting
	var maxConcurrent int = 3 // default
	if setting, err := NewSettingsService(s.db).GetSettingValue("max_concurrent_checks"); err == nil {
		if val, ok := setting.(int); ok && val > 0 {
			maxConcurrent = val
		}
	}

	log.Infof("Starting check for %d phones with max %d concurrent checks", len(phones), maxConcurrent)

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

			log.Infof("Starting check for phone: %s", p.Number)
			if err := s.CheckPhoneNumber(p.ID); err != nil {
				log.Errorf("Failed to check phone %s: %v", p.Number, err)
			} else {
				log.Infof("Completed check for phone: %s", p.Number)
			}
		}(phone)
	}

	wg.Wait()
	log.Info("All phone checks completed")

	return nil
}

// checkOnGateway checks phone on specific gateway with proper synchronization
func (s *CheckService) checkOnGateway(phone *models.PhoneNumber, gateway *models.ADBGateway) error {
	log := s.log.WithFields(logrus.Fields{
		"method":  "checkOnGateway",
		"phone":   phone.Number,
		"gateway": gateway.Name,
	})

	// Get service info
	var service models.SpamService
	if err := s.db.Where("code = ?", gateway.ServiceCode).First(&service).Error; err != nil {
		return fmt.Errorf("service not found: %w", err)
	}

	log.Infof("Checking %s on %s (gateway: %s)", phone.Number, service.Name, gateway.Name)

	// Don't update gateway status in DB, use in-memory locking instead
	// The gateway lock already prevents concurrent usage

	// Since apps are pre-installed, we just need to ensure the app is running
	appPackage, appActivity := s.getAppInfo(gateway.ServiceCode)
	if appPackage != "" && appActivity != "" {
		// Try to start the app (it may already be running)
		if err := s.adbService.StartApp(gateway.ID, appPackage, appActivity); err != nil {
			log.Warnf("Failed to start app (it may already be running): %v", err)
		}
		time.Sleep(2 * time.Second)
	}

	// Simulate incoming call
	log.Infof("Simulating incoming call from %s", phone.Number)
	if err := s.adbService.SimulateIncomingCall(gateway.ID, phone.Number); err != nil {
		return fmt.Errorf("failed to simulate incoming call: %w", err)
	}

	// Wait for the service to process the call
	time.Sleep(5 * time.Second)

	// Take screenshot
	screenshot, err := s.adbService.TakeScreenshot(gateway.ID)
	if err != nil {
		log.Errorf("Failed to take screenshot: %v", err)
		// Continue with empty screenshot
		screenshot = []byte{}
	}

	// End the call
	if err := s.adbService.EndCall(gateway.ID, onlyDigits(phone.Number)); err != nil {
		log.Warnf("Failed to end call: %v", err)
	}

	// Save screenshot if we got one
	var screenshotPath string
	if len(screenshot) > 0 {
		screenshotPath, err = s.saveScreenshot(screenshot, phone.Number, service.Code)
		if err != nil {
			log.Errorf("Failed to save screenshot: %v", err)
			screenshotPath = ""
		}
	}

	// Perform OCR if we have a screenshot
	var ocrText string
	if screenshotPath != "" {
		ocrText, err = s.performOCR(screenshotPath)
		if err != nil {
			log.Errorf("Failed to perform OCR: %v", err)
			ocrText = ""
		}
	}

	// Check for spam keywords
	isSpam, foundKeywords := s.checkForSpamKeywords(ocrText, service.ID)

	// Save result with proper synchronization
	result := &models.CheckResult{
		PhoneNumberID: phone.ID,
		ServiceID:     service.ID,
		IsSpam:        isSpam,
		FoundKeywords: models.StringArray(foundKeywords),
		Screenshot:    screenshotPath,
		RawText:       ocrText,
		CheckedAt:     time.Now(),
	}

	// Use global mutex for writing results to prevent race conditions
	s.resultWriteMutex.Lock()
	err = s.db.Create(result).Error
	s.resultWriteMutex.Unlock()

	if err != nil {
		return fmt.Errorf("failed to save check result: %w", err)
	}

	// Update statistics with proper synchronization
	s.updateStatistics(phone.ID, service.ID, isSpam)

	log.Infof("Check completed for %s on %s: isSpam=%v, keywords=%v",
		phone.Number, service.Name, isSpam, foundKeywords)

	return nil
}

// CheckPhoneRealtime checks phone number in real-time
func (s *CheckService) CheckPhoneRealtime(phoneNumber string) (map[string]interface{}, error) {
	log := s.log.WithFields(logrus.Fields{
		"method": "CheckPhoneRealtime",
		"phone":  phoneNumber,
	})

	// Normalize phone number
	phoneNumber = NewPhoneService(s.db).normalizePhoneNumber(phoneNumber)

	// Check if phone already exists
	var existingPhone models.PhoneNumber
	err := s.db.Where("number = ?", phoneNumber).First(&existingPhone).Error

	if err == nil {
		// Phone exists - check if we have recent results
		var recentResults []models.CheckResult
		err = s.db.Where("phone_number_id = ?", existingPhone.ID).
			Order("checked_at DESC").
			Limit(10).
			Preload("Service").
			Find(&recentResults).Error

		if err == nil && len(recentResults) > 0 {
			// Check if results are fresh (less than 1 hour old)
			latestCheck := recentResults[0].CheckedAt
			if time.Since(latestCheck) < time.Hour {
				// Return cached results
				results := make(map[string]interface{})
				results["phone_number"] = phoneNumber
				results["checked_at"] = latestCheck
				results["cached"] = true

				var serviceResults []map[string]interface{}
				for _, result := range recentResults {
					serviceResults = append(serviceResults, map[string]interface{}{
						"service":        result.Service.Name,
						"is_spam":        result.IsSpam,
						"found_keywords": []string(result.FoundKeywords),
						"checked_at":     result.CheckedAt,
					})
				}
				results["results"] = serviceResults

				log.Infof("Returning cached results for phone %s", phoneNumber)
				return results, nil
			}
		}

		// Results are old or don't exist - perform new check
		log.Infof("Phone %s exists but results are old, performing new check", phoneNumber)
		return s.performRealtimeCheck(&existingPhone)
	}

	// Phone doesn't exist - create temporary phone for realtime check
	tempPhone := &models.PhoneNumber{
		Number:      phoneNumber,
		Description: "Realtime check",
		IsActive:    false, // Don't include in scheduled checks
		CreatedBy:   1,     // System user ID
	}

	// Save phone record
	if err := s.db.Create(tempPhone).Error; err != nil {
		return nil, fmt.Errorf("failed to create phone record: %w", err)
	}

	// Perform check
	result, checkErr := s.performRealtimeCheck(tempPhone)

	// If check failed and phone is temporary, clean it up
	if checkErr != nil && !tempPhone.IsActive {
		// Delete check results and phone
		s.db.Where("phone_number_id = ?", tempPhone.ID).Delete(&models.CheckResult{})
		s.db.Delete(tempPhone)
	}

	return result, checkErr
}

// performRealtimeCheck performs actual realtime check for a phone with priority
func (s *CheckService) performRealtimeCheck(phone *models.PhoneNumber) (map[string]interface{}, error) {
	//log := s.log.WithFields(logrus.Fields{
	//	"method": "performRealtimeCheck",
	//	"phone":  phone.Number,
	//})

	// Use phone check lock to ensure all checks for this phone are synchronized
	phoneCheckLock := s.getPhoneCheckLock(phone.ID)
	phoneCheckLock.Lock()
	defer phoneCheckLock.Unlock()

	results := make(map[string]interface{})
	results["phone_number"] = phone.Number
	results["checked_at"] = time.Now()
	results["cached"] = false

	var serviceResults []map[string]interface{}
	var serviceResultsMutex sync.Mutex
	var wg sync.WaitGroup

	checkMode := s.getCheckMode()

	// Check via ADB if enabled
	if checkMode == models.CheckModeADBOnly || checkMode == models.CheckModeBoth {
		// Get active gateways
		gateways, err := s.adbService.GetActiveGateways()
		if err != nil && checkMode == models.CheckModeADBOnly {
			return nil, fmt.Errorf("failed to get active gateways: %w", err)
		}

		for _, gateway := range gateways {
			wg.Add(1)
			go func(gw models.ADBGateway) {
				defer wg.Done()

				// Try to acquire gateway lock with high priority (immediate)
				if !s.tryLockGateway(gw.ID) {
					// For realtime check, wait a bit and try again
					time.Sleep(100 * time.Millisecond)
					if !s.tryLockGateway(gw.ID) {
						serviceResultsMutex.Lock()
						serviceResults = append(serviceResults, map[string]interface{}{
							"service": gw.ServiceCode,
							"error":   "Gateway is busy",
						})
						serviceResultsMutex.Unlock()
						return
					}
				}
				defer s.unlockGateway(gw.ID)

				// Get service info
				var service models.SpamService
				if err := s.db.Where("code = ?", gw.ServiceCode).First(&service).Error; err != nil {
					serviceResultsMutex.Lock()
					serviceResults = append(serviceResults, map[string]interface{}{
						"service": "Unknown",
						"error":   err.Error(),
					})
					serviceResultsMutex.Unlock()
					return
				}

				// Perform check
				if err := s.checkOnGateway(phone, &gw); err != nil {
					serviceResultsMutex.Lock()
					serviceResults = append(serviceResults, map[string]interface{}{
						"service": service.Name,
						"error":   err.Error(),
					})
					serviceResultsMutex.Unlock()
				} else {
					// Get result
					var checkResult models.CheckResult
					if err := s.db.Where("phone_number_id = ? AND service_id = ?", phone.ID, service.ID).
						Order("checked_at DESC").First(&checkResult).Error; err == nil {
						serviceResultsMutex.Lock()
						serviceResults = append(serviceResults, map[string]interface{}{
							"service":        service.Name,
							"is_spam":        checkResult.IsSpam,
							"found_keywords": []string(checkResult.FoundKeywords),
							"checked_at":     checkResult.CheckedAt,
							"type":           "adb",
						})
						serviceResultsMutex.Unlock()
					}
				}
			}(gateway)
		}
	}

	// Check via API if enabled
	if checkMode == models.CheckModeAPIOnly || checkMode == models.CheckModeBoth {
		// Get active API services
		apiServices, err := s.apiService.GetActiveAPIServices()
		if err != nil && checkMode == models.CheckModeAPIOnly {
			wg.Wait()
			return nil, fmt.Errorf("failed to get active API services: %w", err)
		}

		for _, apiService := range apiServices {
			wg.Add(1)
			go func(api models.APIService) {
				defer wg.Done()

				// Get service info
				var service models.SpamService
				if err := s.db.Where("code = ?", api.ServiceCode).First(&service).Error; err != nil {
					serviceResultsMutex.Lock()
					serviceResults = append(serviceResults, map[string]interface{}{
						"service": "Unknown",
						"error":   err.Error(),
					})
					serviceResultsMutex.Unlock()
					return
				}

				// Perform API check
				checkResult, err := s.apiService.CheckPhoneViaAPI(phone, &api)
				if err != nil {
					serviceResultsMutex.Lock()
					serviceResults = append(serviceResults, map[string]interface{}{
						"service": service.Name,
						"error":   err.Error(),
					})
					serviceResultsMutex.Unlock()
				} else {
					// Update statistics
					s.updateStatistics(phone.ID, service.ID, checkResult.IsSpam)

					serviceResultsMutex.Lock()
					serviceResults = append(serviceResults, map[string]interface{}{
						"service":        service.Name,
						"is_spam":        checkResult.IsSpam,
						"found_keywords": []string(checkResult.FoundKeywords),
						"checked_at":     checkResult.CheckedAt,
						"type":           "api",
					})
					serviceResultsMutex.Unlock()
				}
			}(apiService)
		}
	}

	// Wait for all checks to complete
	wg.Wait()

	results["results"] = serviceResults
	return results, nil
}

// CheckPhoneNumbersInBatch checks multiple phone numbers efficiently
func (s *CheckService) CheckPhoneNumbersInBatch(phoneIDs []uint) error {
	log := s.log.WithFields(logrus.Fields{
		"method": "CheckPhoneNumbersInBatch",
		"count":  len(phoneIDs),
	})

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

	log.Infof("Starting batch check for %d phones with max %d concurrent", len(phoneIDs), maxConcurrent)

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

// updateStatistics updates check statistics with proper locking
func (s *CheckService) updateStatistics(phoneID, serviceID uint, isSpam bool) {
	// Use result write mutex to ensure statistics are updated atomically
	s.resultWriteMutex.Lock()
	defer s.resultWriteMutex.Unlock()

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
	log := s.log.WithFields(logrus.Fields{
		"method": "checkForSpamKeywords",
	})

	text = strings.ToLower(text)
	var foundKeywords []string

	// Get keywords
	var keywords []models.SpamKeyword
	query := s.db.Where("is_active = ?", true)
	query = query.Where("service_id IS NULL OR service_id = ?", serviceID)

	if err := query.Find(&keywords).Error; err != nil {
		log.Errorf("Failed to get spam keywords: %v", err)
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

// GetGatewayStatuses returns current status of all gateways
func (s *CheckService) GetGatewayStatuses() ([]map[string]interface{}, error) {
	gateways, err := s.adbService.ListGateways()
	if err != nil {
		return nil, err
	}

	statuses := make([]map[string]interface{}, len(gateways))
	for i, gateway := range gateways {
		// Check if gateway is busy
		isBusy := s.isGatewayBusy(gateway.ID)

		// Determine actual status
		actualStatus := gateway.Status
		if isBusy && gateway.Status == "online" {
			actualStatus = "checking"
		}

		statuses[i] = map[string]interface{}{
			"id":        gateway.ID,
			"name":      gateway.Name,
			"status":    actualStatus,
			"is_locked": isBusy,
			"service":   gateway.ServiceCode,
		}
	}

	return statuses, nil
}

func onlyDigits(input string) string {
	re := regexp.MustCompile(`\D`)
	return re.ReplaceAllString(input, "")
}
