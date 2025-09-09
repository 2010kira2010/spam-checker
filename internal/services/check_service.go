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
	gatewayBusy      map[uint]bool
	phoneCheckLocks  map[uint]*sync.Mutex
	phoneCheckMu     sync.RWMutex
	resultWriteMutex sync.Mutex
	log              *logrus.Entry

	// New fields for better concurrency control
	gatewayQueue   map[uint]chan struct{} // Queue for each gateway
	gatewayQueueMu sync.RWMutex
	maxRetries     int
	retryDelay     time.Duration
}

// CheckTask represents a task for checking phone on specific gateway/service
type CheckTask struct {
	PhoneID   uint
	Phone     *models.PhoneNumber
	GatewayID uint
	ServiceID uint
	Retry     int
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

func NewCheckService(db *gorm.DB, cfg *config.Config) *CheckService {
	service := &CheckService{
		db:              db,
		cfg:             cfg,
		adbService:      NewADBServiceWithConfig(db, cfg),
		apiService:      NewAPICheckService(db),
		gatewayLocks:    make(map[uint]*sync.Mutex),
		gatewayBusy:     make(map[uint]bool),
		phoneCheckLocks: make(map[uint]*sync.Mutex),
		gatewayQueue:    make(map[uint]chan struct{}),
		log:             logger.WithField("service", "CheckService"),
		maxRetries:      3,
		retryDelay:      2 * time.Second,
	}

	// Initialize gateway queues
	service.initGatewayQueues()

	return service
}

// initGatewayQueues initializes queue channels for each gateway
func (s *CheckService) initGatewayQueues() {
	gateways, err := s.adbService.ListGateways()
	if err != nil {
		s.log.Errorf("Failed to initialize gateway queues: %v", err)
		return
	}

	s.gatewayQueueMu.Lock()
	defer s.gatewayQueueMu.Unlock()

	for _, gateway := range gateways {
		// Create a buffered channel that acts as a semaphore (1 = only one task at a time)
		s.gatewayQueue[gateway.ID] = make(chan struct{}, 1)
	}
}

// getGatewayQueue returns or creates a queue for gateway
func (s *CheckService) getGatewayQueue(gatewayID uint) chan struct{} {
	s.gatewayQueueMu.RLock()
	if queue, exists := s.gatewayQueue[gatewayID]; exists {
		s.gatewayQueueMu.RUnlock()
		return queue
	}
	s.gatewayQueueMu.RUnlock()

	// Create new queue if doesn't exist
	s.gatewayQueueMu.Lock()
	defer s.gatewayQueueMu.Unlock()

	if queue, exists := s.gatewayQueue[gatewayID]; exists {
		return queue
	}

	queue := make(chan struct{}, 1)
	s.gatewayQueue[gatewayID] = queue
	return queue
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

	// Use phone-level lock to prevent duplicate concurrent checks
	phoneCheckLock := s.getPhoneCheckLock(phoneID)
	phoneCheckLock.Lock()
	defer phoneCheckLock.Unlock()

	// Get check mode setting
	checkMode := s.getCheckMode()

	log.Infof("Starting check for phone %s with mode: %s", phone.Number, checkMode)

	// Create error channel to collect errors
	errChan := make(chan error, 2)
	var wg sync.WaitGroup

	// Perform checks based on mode
	switch checkMode {
	case models.CheckModeADBOnly:
		return s.checkViaADBWithRetry(&phone)

	case models.CheckModeAPIOnly:
		return s.checkViaAPIWithRetry(&phone)

	case models.CheckModeBoth:
		// Check both ADB and API concurrently
		wg.Add(2)

		go func() {
			defer wg.Done()
			if err := s.checkViaADBWithRetry(&phone); err != nil {
				errChan <- fmt.Errorf("ADB: %w", err)
			}
		}()

		go func() {
			defer wg.Done()
			if err := s.checkViaAPIWithRetry(&phone); err != nil {
				errChan <- fmt.Errorf("API: %w", err)
			}
		}()

		wg.Wait()
		close(errChan)

		// Collect errors
		var errors []error
		for err := range errChan {
			errors = append(errors, err)
		}

		// Return error only if both failed
		if len(errors) == 2 {
			return fmt.Errorf("both checks failed: %v", errors)
		}

		return nil

	default:
		return fmt.Errorf("unknown check mode: %s", checkMode)
	}
}

// checkViaADBWithRetry checks phone via ADB with retry logic
func (s *CheckService) checkViaADBWithRetry(phone *models.PhoneNumber) error {
	log := s.log.WithFields(logrus.Fields{
		"method": "checkViaADBWithRetry",
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

	// Create task channels
	taskChan := make(chan CheckTask, len(gateways))
	resultChan := make(chan ConcurrentCheckResult, len(gateways))

	// Worker pool size (limit concurrent checks)
	maxWorkers := 5
	if len(gateways) < maxWorkers {
		maxWorkers = len(gateways)
	}

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go s.adbCheckWorker(taskChan, resultChan, &wg)
	}

	// Create tasks for each gateway
	for _, gateway := range gateways {
		task := CheckTask{
			PhoneID:   phone.ID,
			Phone:     phone,
			GatewayID: gateway.ID,
			ServiceID: 0, // Will be resolved in worker
			Retry:     0,
		}
		taskChan <- task
	}
	close(taskChan)

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	successCount := 0
	errorCount := 0
	var lastError error

	for result := range resultChan {
		if result.Error != nil {
			errorCount++
			lastError = result.Error
			log.Errorf("Check failed on gateway %s: %v", result.Gateway.Name, result.Error)
		} else {
			successCount++
			log.Infof("Check succeeded on gateway %s", result.Gateway.Name)
		}
	}

	log.Infof("ADB check completed for phone %s: %d successful, %d failed",
		phone.Number, successCount, errorCount)

	if successCount == 0 && errorCount > 0 {
		return fmt.Errorf("all ADB checks failed: %v", lastError)
	}

	return nil
}

// adbCheckWorker processes ADB check tasks
func (s *CheckService) adbCheckWorker(taskChan <-chan CheckTask, resultChan chan<- ConcurrentCheckResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for task := range taskChan {
		result := ConcurrentCheckResult{
			PhoneID: task.PhoneID,
		}

		// Get gateway
		gateway, err := s.adbService.GetGatewayByID(task.GatewayID)
		if err != nil {
			result.Error = fmt.Errorf("failed to get gateway: %w", err)
			resultChan <- result
			continue
		}
		result.Gateway = gateway

		// Get service info
		var service models.SpamService
		if err := s.db.Where("code = ?", gateway.ServiceCode).First(&service).Error; err != nil {
			result.Error = fmt.Errorf("service not found: %w", err)
			resultChan <- result
			continue
		}
		result.Service = &service

		// Try to perform check with retries
		err = s.checkOnGatewayWithRetry(task.Phone, gateway, &service, task.Retry)
		if err != nil {
			result.Error = err
		} else {
			// Get the created result
			var checkResult models.CheckResult
			if err := s.db.Where("phone_number_id = ? AND service_id = ?", task.Phone.ID, service.ID).
				Order("checked_at DESC").First(&checkResult).Error; err == nil {
				result.Result = &checkResult
			}
		}

		resultChan <- result
	}
}

// checkOnGatewayWithRetry performs check on gateway with retry logic
func (s *CheckService) checkOnGatewayWithRetry(phone *models.PhoneNumber, gateway *models.ADBGateway, service *models.SpamService, currentRetry int) error {
	log := s.log.WithFields(logrus.Fields{
		"method":  "checkOnGatewayWithRetry",
		"phone":   phone.Number,
		"gateway": gateway.Name,
		"retry":   currentRetry,
	})

	// Get gateway queue (acts as a semaphore)
	queue := s.getGatewayQueue(gateway.ID)

	// Try to acquire slot with timeout
	maxWaitTime := 30 * time.Second
	if currentRetry > 0 {
		maxWaitTime = 10 * time.Second // Shorter wait on retries
	}

	select {
	case queue <- struct{}{}:
		// Successfully acquired slot
		defer func() { <-queue }() // Release slot when done

		log.Infof("Acquired gateway %s for checking %s", gateway.Name, phone.Number)

		// Perform the actual check
		err := s.performGatewayCheck(phone, gateway, service)
		if err != nil {
			// Check if we should retry
			if currentRetry < s.maxRetries && s.isRetryableError(err) {
				log.Warnf("Check failed on gateway %s, will retry (attempt %d/%d): %v",
					gateway.Name, currentRetry+1, s.maxRetries, err)

				time.Sleep(s.retryDelay)
				return s.checkOnGatewayWithRetry(phone, gateway, service, currentRetry+1)
			}
			return err
		}

		return nil

	case <-time.After(maxWaitTime):
		// Timeout waiting for gateway
		if currentRetry < s.maxRetries {
			log.Warnf("Timeout waiting for gateway %s, retrying (attempt %d/%d)",
				gateway.Name, currentRetry+1, s.maxRetries)

			time.Sleep(s.retryDelay)
			return s.checkOnGatewayWithRetry(phone, gateway, service, currentRetry+1)
		}

		return fmt.Errorf("gateway %s is busy after %d retries", gateway.Name, s.maxRetries)
	}
}

// performGatewayCheck performs the actual check on gateway
func (s *CheckService) performGatewayCheck(phone *models.PhoneNumber, gateway *models.ADBGateway, service *models.SpamService) error {
	log := s.log.WithFields(logrus.Fields{
		"method":  "performGatewayCheck",
		"phone":   phone.Number,
		"gateway": gateway.Name,
	})

	// Ensure app is running
	appPackage, appActivity := s.getAppInfo(gateway.ServiceCode)
	if appPackage != "" && appActivity != "" {
		if err := s.adbService.StartApp(gateway.ID, appPackage, appActivity); err != nil {
			log.Warnf("Failed to start app: %v", err)
		}
		time.Sleep(2 * time.Second)
	}

	// Simulate incoming call
	log.Infof("Simulating incoming call from %s", phone.Number)
	if err := s.adbService.SimulateIncomingCall(gateway.ID, phone.Number); err != nil {
		return fmt.Errorf("failed to simulate incoming call: %w", err)
	}

	// Wait for the service to process
	time.Sleep(5 * time.Second)

	// Take screenshot
	screenshot, err := s.adbService.TakeScreenshot(gateway.ID)
	if err != nil {
		log.Errorf("Failed to take screenshot: %v", err)
		screenshot = []byte{}
	}

	// End the call
	if err := s.adbService.EndCall(gateway.ID, onlyDigits(phone.Number)); err != nil {
		log.Warnf("Failed to end call: %v", err)
	}

	// Process and save results
	return s.processCheckResult(phone, service, screenshot)
}

// processCheckResult processes and saves check result
func (s *CheckService) processCheckResult(phone *models.PhoneNumber, service *models.SpamService, screenshot []byte) error {
	log := s.log.WithFields(logrus.Fields{
		"method":  "processCheckResult",
		"phone":   phone.Number,
		"service": service.Name,
	})

	// Save screenshot
	var screenshotPath string
	if len(screenshot) > 0 {
		var err error
		screenshotPath, err = s.saveScreenshot(screenshot, phone.Number, service.Code)
		if err != nil {
			log.Errorf("Failed to save screenshot: %v", err)
		}
	}

	// Perform OCR
	var ocrText string
	if screenshotPath != "" {
		var err error
		ocrText, err = s.performOCR(screenshotPath)
		if err != nil {
			log.Errorf("Failed to perform OCR: %v", err)
		}
	}

	// Check for spam keywords
	isSpam, foundKeywords := s.checkForSpamKeywords(ocrText, service.ID)

	// Create result
	result := &models.CheckResult{
		PhoneNumberID: phone.ID,
		ServiceID:     service.ID,
		IsSpam:        isSpam,
		FoundKeywords: models.StringArray(foundKeywords),
		Screenshot:    screenshotPath,
		RawText:       ocrText,
		CheckedAt:     time.Now(),
	}

	// Use transaction to ensure atomic write
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Save result
		if err := tx.Create(result).Error; err != nil {
			return fmt.Errorf("failed to save check result: %w", err)
		}

		// Update statistics
		return s.updateStatisticsInTx(tx, phone.ID, service.ID, isSpam)
	})

	if err != nil {
		return err
	}

	log.Infof("Check completed for %s on %s: isSpam=%v, keywords=%v",
		phone.Number, service.Name, isSpam, foundKeywords)

	return nil
}

// checkViaAPIWithRetry checks phone via API with retry logic
func (s *CheckService) checkViaAPIWithRetry(phone *models.PhoneNumber) error {
	log := s.log.WithFields(logrus.Fields{
		"method": "checkViaAPIWithRetry",
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

	// Create result channel
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

			// Perform check with retries
			var checkResult *models.CheckResult
			var lastErr error

			for retry := 0; retry <= s.maxRetries; retry++ {
				log.Infof("Checking phone %s via API %s (attempt %d/%d)",
					phone.Number, api.Name, retry+1, s.maxRetries+1)

				checkResult, err = s.apiService.CheckPhoneViaAPI(phone, &api)
				if err != nil {
					lastErr = err
					if retry < s.maxRetries && s.isRetryableError(err) {
						log.Warnf("API check failed, retrying: %v", err)
						time.Sleep(s.retryDelay)
						continue
					}
					result.Error = err
					break
				} else {
					result.Result = checkResult

					// Get service info after successful check (it should be created by CheckPhoneViaAPI if needed)
					var service models.SpamService
					if err := s.db.Where("code = ?", api.ServiceCode).First(&service).Error; err == nil {
						result.Service = &service

						// Update statistics in transaction
						s.db.Transaction(func(tx *gorm.DB) error {
							return s.updateStatisticsInTx(tx, phone.ID, service.ID, checkResult.IsSpam)
						})
					} else {
						log.Warnf("Failed to get service after check: %v", err)
					}

					// Log extracted data for debugging
					if checkResult.RawText != "" {
						log.Debugf("API %s extracted text: %s", api.Name, checkResult.RawText)
					}
					if len(checkResult.FoundKeywords) > 0 {
						log.Debugf("API %s found keywords: %v", api.Name, []string(checkResult.FoundKeywords))
					}

					break
				}
			}

			// If all retries failed, set the last error
			if checkResult == nil && lastErr != nil {
				result.Error = lastErr
			}

			resultChan <- result
		}(apiService)
	}

	// Close channel when all done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	successCount := 0
	errorCount := 0
	var lastError error
	hasSpamDetection := false

	for result := range resultChan {
		if result.Error != nil {
			errorCount++
			lastError = result.Error
			log.Errorf("API check failed for %s: %v", result.APIService.Name, result.Error)
		} else {
			successCount++
			log.Infof("API check succeeded for %s", result.APIService.Name)

			// Check if any service detected spam
			if result.Result != nil && result.Result.IsSpam {
				hasSpamDetection = true
			}
		}
	}

	log.Infof("API check completed for phone %s: %d successful, %d failed, spam detected: %v",
		phone.Number, successCount, errorCount, hasSpamDetection)

	if successCount == 0 && errorCount > 0 {
		return fmt.Errorf("all API checks failed: %v", lastError)
	}

	return nil
}

// isRetryableError determines if an error should trigger a retry
func (s *CheckService) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// List of error patterns that should trigger retry
	retryablePatterns := []string{
		"gateway busy",
		"timeout",
		"device not responding",
		"failed to take screenshot",
		"failed to simulate call",
		"connection refused",
		"deadline exceeded",
		"temporary failure",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}

	return false
}

// updateStatisticsInTx updates statistics within a transaction
func (s *CheckService) updateStatisticsInTx(tx *gorm.DB, phoneID, serviceID uint, isSpam bool) error {
	var stats models.Statistics

	// Try to find existing statistics
	err := tx.Where("phone_number_id = ? AND service_id = ?", phoneID, serviceID).First(&stats).Error

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
		return tx.Create(&stats).Error
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
		return tx.Save(&stats).Error
	}

	return err
}

// CheckAllPhones checks all active phone numbers with proper queue management
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

	// Get max concurrent phone checks setting
	var maxConcurrent int = 3
	if setting, err := NewSettingsService(s.db).GetSettingValue("max_concurrent_checks"); err == nil {
		if val, ok := setting.(int); ok && val > 0 {
			maxConcurrent = val
		}
	}

	log.Infof("Starting check for %d phones with max %d concurrent checks", len(phones), maxConcurrent)

	// Create work channel
	workChan := make(chan models.PhoneNumber, len(phones))
	for _, phone := range phones {
		workChan <- phone
	}
	close(workChan)

	// Create worker pool
	var wg sync.WaitGroup
	errorChan := make(chan error, len(phones))

	for i := 0; i < maxConcurrent; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for phone := range workChan {
				log.Infof("[Worker %d] Starting check for phone: %s", workerID, phone.Number)

				if err := s.CheckPhoneNumber(phone.ID); err != nil {
					errorChan <- fmt.Errorf("phone %s: %w", phone.Number, err)
					log.Errorf("[Worker %d] Failed to check phone %s: %v", workerID, phone.Number, err)
				} else {
					log.Infof("[Worker %d] Completed check for phone: %s", workerID, phone.Number)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Collect errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		log.Warnf("Completed with %d errors out of %d phones", len(errors), len(phones))
		// Don't return error - partial success is OK
	}

	log.Info("All phone checks completed")
	return nil
}

// GetDB returns database instance
func (s *CheckService) GetDB() *gorm.DB {
	return s.db
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

// Helper methods
func (s *CheckService) getCheckMode() models.CheckMode {
	var setting models.SystemSettings
	if err := s.db.Where("key = ?", "check_mode").First(&setting).Error; err != nil {
		return models.CheckModeADBOnly
	}
	return models.CheckMode(setting.Value)
}

func (s *CheckService) saveScreenshot(data []byte, phoneNumber, serviceCode string) (string, error) {
	dir := filepath.Join("screenshots", serviceCode)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	filename := fmt.Sprintf("%s_%s_%d.png", phoneNumber, serviceCode, time.Now().Unix())
	path := filepath.Join(dir, filename)

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		if err := os.WriteFile(path, data, 0644); err != nil {
			return "", fmt.Errorf("failed to save screenshot: %w", err)
		}
		return path, nil
	}

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

func (s *CheckService) performOCR(imagePath string) (string, error) {
	cmd := exec.Command(s.cfg.OCR.TesseractPath, imagePath, "stdout", "-l", s.cfg.OCR.Language)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("OCR failed: %w", err)
	}
	return string(output), nil
}

func (s *CheckService) checkForSpamKeywords(text string, serviceID uint) (bool, []string) {
	text = strings.ToLower(text)
	var foundKeywords []string

	var keywords []models.SpamKeyword
	query := s.db.Where("is_active = ?", true)
	query = query.Where("service_id IS NULL OR service_id = ?", serviceID)

	if err := query.Find(&keywords).Error; err != nil {
		s.log.Errorf("Failed to get spam keywords: %v", err)
		return false, foundKeywords
	}

	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword.Keyword)) {
			foundKeywords = append(foundKeywords, keyword.Keyword)
		}
	}

	return len(foundKeywords) > 0, foundKeywords
}

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

func onlyDigits(input string) string {
	re := regexp.MustCompile(`\D`)
	return re.ReplaceAllString(input, "")
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
					serviceResult := map[string]interface{}{
						"service":        result.Service.Name,
						"is_spam":        result.IsSpam,
						"found_keywords": []string(result.FoundKeywords),
						"checked_at":     result.CheckedAt,
					}

					// Add source information
					if result.RawResponse != "" {
						serviceResult["source"] = "api"
						if result.RawText != "" {
							serviceResult["extracted_text"] = result.RawText
						}
					} else if result.Screenshot != "" {
						serviceResult["source"] = "adb"
						if result.RawText != "" {
							serviceResult["ocr_text"] = result.RawText
						}
					}

					serviceResults = append(serviceResults, serviceResult)
				}
				results["results"] = serviceResults

				log.Infof("Returning cached results for phone %s", phoneNumber)
				return results, nil
			}
		}

		// Results are old or don't exist - perform new check
		log.Infof("Phone %s exists but results are old, performing new check", phoneNumber)
		if err := s.CheckPhoneNumber(existingPhone.ID); err != nil {
			return nil, fmt.Errorf("failed to check phone: %w", err)
		}
		return s.getPhoneResults(&existingPhone)
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
	checkErr := s.CheckPhoneNumber(tempPhone.ID)

	// Get results
	results, _ := s.getPhoneResults(tempPhone)

	// If check failed and phone is temporary, clean it up
	if checkErr != nil && !tempPhone.IsActive {
		// Delete check results and phone
		s.db.Where("phone_number_id = ?", tempPhone.ID).Delete(&models.CheckResult{})
		s.db.Delete(tempPhone)
	}

	return results, checkErr
}

func (s *CheckService) getPhoneResults(phone *models.PhoneNumber) (map[string]interface{}, error) {
	results := make(map[string]interface{})
	results["phone_number"] = phone.Number
	results["checked_at"] = time.Now()
	results["cached"] = false

	var checkResults []models.CheckResult
	err := s.db.Where("phone_number_id = ?", phone.ID).
		Order("checked_at DESC").
		Preload("Service").
		Find(&checkResults).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get results: %w", err)
	}

	var serviceResults []map[string]interface{}
	for _, result := range checkResults {
		serviceResult := map[string]interface{}{
			"service":        result.Service.Name,
			"is_spam":        result.IsSpam,
			"found_keywords": []string(result.FoundKeywords),
			"checked_at":     result.CheckedAt,
		}

		// Add extracted text if available (from API response)
		if result.RawText != "" && result.RawResponse != "" {
			// This means it's an API result with extracted data
			serviceResult["extracted_text"] = result.RawText
			serviceResult["source"] = "api"
		} else if result.Screenshot != "" {
			// This is an ADB/screenshot result
			serviceResult["source"] = "adb"
			if result.RawText != "" {
				serviceResult["ocr_text"] = result.RawText
			}
		}

		serviceResults = append(serviceResults, serviceResult)
	}
	results["results"] = serviceResults

	return results, nil
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
		// Check queue status
		queue := s.getGatewayQueue(gateway.ID)
		queueLen := len(queue)
		isBusy := queueLen > 0

		// Determine actual status
		actualStatus := gateway.Status
		if isBusy && gateway.Status == "online" {
			actualStatus = "checking"
		}

		statuses[i] = map[string]interface{}{
			"id":         gateway.ID,
			"name":       gateway.Name,
			"status":     actualStatus,
			"is_locked":  isBusy,
			"queue_size": queueLen,
			"service":    gateway.ServiceCode,
		}
	}

	return statuses, nil
}
