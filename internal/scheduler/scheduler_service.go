package scheduler

import (
	"fmt"
	"spam-checker/internal/config"
	"spam-checker/internal/logger"
	"spam-checker/internal/models"
	"spam-checker/internal/services"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jasonlvhit/gocron"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type CheckScheduler struct {
	scheduler           *gocron.Scheduler
	checkService        *services.CheckService
	phoneService        *services.PhoneService
	notificationService *services.NotificationService
	db                  *gorm.DB
	jobs                map[uint]*gocron.Job
	cfg                 *config.Config
	log                 *logrus.Entry
	defaultIntervalJob  *gocron.Job
	currentInterval     int
	isRunning           bool
	runningMutex        sync.RWMutex
	stopChan            chan struct{}

	// Global check mutex to prevent concurrent checks
	checkMutex       sync.Mutex
	isCheckingNow    bool
	lastCheckTime    time.Time
	minCheckInterval time.Duration // Minimum time between checks
}

func NewCheckScheduler(db *gorm.DB, checkService *services.CheckService, phoneService *services.PhoneService, notificationService *services.NotificationService, cfg *config.Config) *CheckScheduler {
	return &CheckScheduler{
		scheduler:           gocron.NewScheduler(),
		checkService:        checkService,
		phoneService:        phoneService,
		notificationService: notificationService,
		db:                  db,
		jobs:                make(map[uint]*gocron.Job),
		cfg:                 cfg,
		log:                 logger.WithField("service", "CheckScheduler"),
		currentInterval:     -1,
		isRunning:           false,
		stopChan:            make(chan struct{}),
		isCheckingNow:       false,
		minCheckInterval:    5 * time.Minute, // Minimum 5 minutes between checks
	}
}

// Start starts the scheduler
func (s *CheckScheduler) Start() {
	log := s.log.WithFields(logrus.Fields{
		"method": "Start",
	})

	s.runningMutex.Lock()
	if s.isRunning {
		s.runningMutex.Unlock()
		log.Warn("Scheduler is already running")
		return
	}
	s.isRunning = true
	s.runningMutex.Unlock()

	log.Info("Starting check scheduler...")

	// Load schedules from database
	s.loadSchedules()

	// Start default interval check based on settings
	s.startDefaultIntervalCheck()

	// Start scheduler in background
	go s.scheduler.Start()

	// Monitor gateway statuses every 5 minutes
	s.scheduler.Every(5).Minutes().Do(func() {
		adbService := services.NewADBService(s.db, s.cfg)
		if err := adbService.UpdateAllGatewayStatuses(); err != nil {
			log.Errorf("Failed to update gateway statuses: %v", err)
		}
	})

	// Check for configuration changes every minute
	s.scheduler.Every(1).Minutes().Do(func() {
		s.checkForConfigurationChanges()
	})

	log.Info("Check scheduler started successfully")
}

// Stop stops the scheduler
func (s *CheckScheduler) Stop() {
	log := s.log.WithFields(logrus.Fields{
		"method": "Stop",
	})

	s.runningMutex.Lock()
	defer s.runningMutex.Unlock()

	if !s.isRunning {
		log.Warn("Scheduler is not running")
		return
	}

	log.Info("Stopping check scheduler...")

	// Signal stop
	close(s.stopChan)

	// Clear all jobs
	s.scheduler.Clear()

	// Reset state
	s.isRunning = false
	s.currentInterval = -1
	s.defaultIntervalJob = nil
	s.jobs = make(map[uint]*gocron.Job)
	s.isCheckingNow = false

	log.Info("Check scheduler stopped")
}

// canStartCheck checks if we can start a new check
func (s *CheckScheduler) canStartCheck() bool {
	s.checkMutex.Lock()
	defer s.checkMutex.Unlock()

	// Check if already checking
	if s.isCheckingNow {
		s.log.Warn("Check already in progress, skipping")
		return false
	}

	// Check minimum interval
	if time.Since(s.lastCheckTime) < s.minCheckInterval {
		s.log.Warnf("Too soon since last check (%v ago), skipping", time.Since(s.lastCheckTime))
		return false
	}

	// Mark as checking
	s.isCheckingNow = true
	s.lastCheckTime = time.Now()
	return true
}

// markCheckComplete marks check as complete
func (s *CheckScheduler) markCheckComplete() {
	s.checkMutex.Lock()
	defer s.checkMutex.Unlock()
	s.isCheckingNow = false
}

// runDefaultCheck runs the default interval check
func (s *CheckScheduler) runDefaultCheck() {
	log := s.log.WithFields(logrus.Fields{
		"method": "runDefaultCheck",
	})

	// Check if we can start
	if !s.canStartCheck() {
		return
	}
	defer s.markCheckComplete()

	log.Info("Starting default interval check")

	// Perform the check with unified method
	s.performPhoneCheck("default", 0)
}

// runScheduledCheck runs a scheduled check
func (s *CheckScheduler) runScheduledCheck(scheduleID uint) {
	log := s.log.WithFields(logrus.Fields{
		"method":     "runScheduledCheck",
		"scheduleID": scheduleID,
	})

	// Check if we can start
	if !s.canStartCheck() {
		return
	}
	defer s.markCheckComplete()

	log.Infof("Starting scheduled check ID: %d", scheduleID)

	// Update last run time
	now := time.Now()
	if err := s.db.Model(&models.CheckSchedule{}).Where("id = ?", scheduleID).Update("last_run", &now).Error; err != nil {
		log.Errorf("Failed to update last run time: %v", err)
	}

	// Perform the check with unified method
	s.performPhoneCheck("scheduled", scheduleID)

	// Update next run time
	if job, exists := s.jobs[scheduleID]; exists {
		nextRun := job.NextScheduledTime()
		s.db.Model(&models.CheckSchedule{}).Where("id = ?", scheduleID).Update("next_run", &nextRun)
	}
}

// performPhoneCheck performs the actual phone checking with proper result aggregation
func (s *CheckScheduler) performPhoneCheck(checkType string, scheduleID uint) {
	log := s.log.WithFields(logrus.Fields{
		"method":     "performPhoneCheck",
		"checkType":  checkType,
		"scheduleID": scheduleID,
	})

	startTime := time.Now()

	// Get active phones
	phones, err := s.phoneService.GetActivePhones()
	if err != nil {
		log.Errorf("Failed to get active phones: %v", err)
		return
	}

	if len(phones) == 0 {
		log.Info("No active phones to check")
		return
	}

	log.Infof("Starting check for %d phones", len(phones))

	// Track all results for single notification
	allResults := make(map[uint][]models.CheckResult)
	totalSpamCount := 0
	successCount := 0
	var checkErrors []error

	// Check each phone sequentially to avoid conflicts
	for _, phone := range phones {
		// Check if we're stopping
		select {
		case <-s.stopChan:
			log.Info("Scheduler stopping, aborting check")
			return
		default:
		}

		// Perform check with timeout
		checkDone := make(chan error, 1)
		go func(p models.PhoneNumber) {
			checkDone <- s.checkService.CheckPhoneNumber(p.ID)
		}(phone)

		select {
		case err := <-checkDone:
			if err != nil {
				// Check if it's a "already checking" error
				if strings.Contains(err.Error(), "already being checked") {
					log.Debugf("Phone %s is already being checked by another process", phone.Number)
					// Don't count this as an error
				} else {
					log.Errorf("Failed to check phone %s: %v", phone.Number, err)
					checkErrors = append(checkErrors, err)
				}
			} else {
				successCount++
				// Get latest results for this phone
				checkResults, err := s.checkService.GetCheckResults(phone.ID, 0, 10)
				if err == nil && len(checkResults) > 0 {
					allResults[phone.ID] = checkResults

					// Check if any service detected spam
					for _, result := range checkResults {
						if result.IsSpam {
							totalSpamCount++
							break // Count each phone only once
						}
					}
				}
			}
		case <-time.After(30 * time.Second):
			log.Warnf("Check timeout for phone %s", phone.Number)
			checkErrors = append(checkErrors, fmt.Errorf("timeout checking phone %s", phone.Number))
		case <-s.stopChan:
			log.Info("Scheduler stopping, aborting check")
			return
		}

		// Small delay between checks
		time.Sleep(1 * time.Second)
	}

	// Calculate duration
	duration := time.Since(startTime)

	// Log summary
	log.Infof("%s check completed in %v. Checked %d phones, found %d spam, %d succeeded, %d errors",
		checkType, duration, len(phones), totalSpamCount, successCount, len(checkErrors))

	// Send single consolidated notification if spam found
	if totalSpamCount > 0 {
		s.sendConsolidatedNotification(checkType, scheduleID, totalSpamCount, len(phones), allResults)
	}
}

// sendConsolidatedNotification sends a single notification with all results
func (s *CheckScheduler) sendConsolidatedNotification(checkType string, scheduleID uint, spamCount, totalCount int, results map[uint][]models.CheckResult) {
	log := s.log.WithFields(logrus.Fields{
		"method": "sendConsolidatedNotification",
	})

	// Build notification message
	var title string
	if checkType == "scheduled" && scheduleID > 0 {
		var schedule models.CheckSchedule
		if err := s.db.First(&schedule, scheduleID).Error; err == nil {
			title = fmt.Sprintf("📋 %s Results", schedule.Name)
		} else {
			title = "📋 Результат проверки по расписанию"
		}
	} else {
		title = "🔍 Результат проверки"
	}

	message := fmt.Sprintf(
		"%s\n\n"+
			"Всего проверенных номеров: %d\n"+
			"Обнаружено спама: %d\n"+
			"Чистые: %d\n",
		title, totalCount, spamCount, totalCount-spamCount,
	)

	// Group spam results by service
	serviceSpamMap := make(map[string][]string)

	for phoneID, checkResults := range results {
		for _, result := range checkResults {
			if result.IsSpam {
				var phone models.PhoneNumber
				if err := s.db.First(&phone, phoneID).Error; err == nil {
					serviceName := result.Service.Name
					if serviceName == "" {
						// Load service if not preloaded
						var service models.SpamService
						if err := s.db.First(&service, result.ServiceID).Error; err == nil {
							serviceName = service.Name
						}
					}

					keywords := []string(result.FoundKeywords)
					phoneInfo := fmt.Sprintf("%s: %v", phone.Number, keywords)
					serviceSpamMap[serviceName] = append(serviceSpamMap[serviceName], phoneInfo)
				}
			}
		}
	}

	// Add spam details grouped by service
	if len(serviceSpamMap) > 0 {
		message += "\n⚠️🚨 Обнаружение спама по сервисам:\n"
		for serviceName, phones := range serviceSpamMap {
			message += fmt.Sprintf("\n📱 %s:\n", serviceName)
			for _, phoneInfo := range phones {
				message += fmt.Sprintf("  • %s\n", phoneInfo)
			}
		}
	}

	// Send notification
	if err := s.notificationService.SendNotification(title, message); err != nil {
		log.Errorf("Failed to send notification: %v", err)
	} else {
		log.Info("Notification sent successfully")
	}
}

// checkForConfigurationChanges checks if configuration has changed and reloads if necessary
func (s *CheckScheduler) checkForConfigurationChanges() {
	log := s.log.WithFields(logrus.Fields{
		"method": "checkForConfigurationChanges",
	})

	// Check if check_interval_minutes has changed
	var setting models.SystemSettings
	if err := s.db.Where("key = ?", "check_interval_minutes").First(&setting).Error; err == nil {
		intervalMinutes, err := strconv.Atoi(setting.Value)
		if err == nil && intervalMinutes > 0 {
			// Only restart if interval actually changed
			if intervalMinutes != s.currentInterval {
				log.Infof("Check interval changed from %d to %d minutes", s.currentInterval, intervalMinutes)
				s.updateDefaultIntervalCheck(intervalMinutes)
			}
		}
	}

	// Reload custom schedules
	s.reloadCustomSchedules()
}

// startDefaultIntervalCheck starts the default interval check based on settings
func (s *CheckScheduler) startDefaultIntervalCheck() {
	log := s.log.WithFields(logrus.Fields{
		"method": "startDefaultIntervalCheck",
	})

	// Get check interval from settings
	var setting models.SystemSettings
	intervalMinutes := 60 // Default value

	if err := s.db.Where("key = ?", "check_interval_minutes").First(&setting).Error; err != nil {
		log.Warnf("Failed to get check_interval_minutes setting, using default 60 minutes")
	} else {
		if val, err := strconv.Atoi(setting.Value); err == nil && val > 0 {
			intervalMinutes = val
		} else {
			log.Warnf("Invalid check_interval_minutes value: %s, using default 60 minutes", setting.Value)
		}
	}

	s.updateDefaultIntervalCheck(intervalMinutes)
}

// updateDefaultIntervalCheck updates the default interval check job
func (s *CheckScheduler) updateDefaultIntervalCheck(intervalMinutes int) {
	log := s.log.WithFields(logrus.Fields{
		"method":   "updateDefaultIntervalCheck",
		"interval": intervalMinutes,
	})

	// Remove old job if exists
	if s.defaultIntervalJob != nil {
		s.scheduler.Remove(s.defaultIntervalJob)
		s.defaultIntervalJob = nil
	}

	// Update current interval
	s.currentInterval = intervalMinutes

	// Update minimum check interval to be at least 1/4 of the interval
	minInterval := time.Duration(intervalMinutes/4) * time.Minute
	if minInterval < 5*time.Minute {
		minInterval = 5 * time.Minute
	}
	s.minCheckInterval = minInterval

	log.Infof("Setting default interval check to every %d minutes (min interval: %v)", intervalMinutes, minInterval)

	// Create new job
	job := s.scheduler.Every(uint64(intervalMinutes)).Minutes()
	job.Do(s.runDefaultCheck)
	s.defaultIntervalJob = job
}

// loadSchedules loads schedules from database
func (s *CheckScheduler) loadSchedules() {
	log := s.log.WithFields(logrus.Fields{
		"method": "loadSchedules",
	})

	var schedules []models.CheckSchedule
	if err := s.db.Where("is_active = ?", true).Find(&schedules).Error; err != nil {
		log.Errorf("Failed to load schedules: %v", err)
		return
	}

	log.Infof("Loading %d active schedules", len(schedules))

	for _, schedule := range schedules {
		if err := s.AddSchedule(&schedule); err != nil {
			log.Errorf("Failed to add schedule %s: %v", schedule.Name, err)
		}
	}
}

// AddSchedule adds a new schedule
func (s *CheckScheduler) AddSchedule(schedule *models.CheckSchedule) error {
	log := s.log.WithFields(logrus.Fields{
		"method":     "AddSchedule",
		"schedule":   schedule.Name,
		"expression": schedule.CronExpression,
	})

	// Remove existing job if any
	s.RemoveSchedule(schedule.ID)

	// Parse cron expression and create job
	job, err := s.parseCronExpression(schedule.CronExpression)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	// Set job function
	job.Do(s.runScheduledCheck, schedule.ID)

	// Store job reference
	s.jobs[schedule.ID] = job

	// Update next run time
	nextRun := job.NextScheduledTime()
	s.db.Model(schedule).Update("next_run", &nextRun)

	log.Infof("Added schedule: %s (%s)", schedule.Name, schedule.CronExpression)

	return nil
}

// RemoveSchedule removes a schedule
func (s *CheckScheduler) RemoveSchedule(scheduleID uint) {
	log := s.log.WithFields(logrus.Fields{
		"method":     "RemoveSchedule",
		"scheduleID": scheduleID,
	})

	if job, exists := s.jobs[scheduleID]; exists {
		s.scheduler.Remove(job)
		delete(s.jobs, scheduleID)
		log.Infof("Removed schedule ID: %d", scheduleID)
	}
}

// reloadCustomSchedules reloads custom schedules from database
func (s *CheckScheduler) reloadCustomSchedules() {
	log := s.log.WithFields(logrus.Fields{
		"method": "reloadCustomSchedules",
	})

	// Get all schedules from database
	var schedules []models.CheckSchedule
	if err := s.db.Find(&schedules).Error; err != nil {
		log.Errorf("Failed to load schedules: %v", err)
		return
	}

	// Track which schedules are in DB
	schedulesInDB := make(map[uint]bool)

	// Check for new or updated schedules
	for _, schedule := range schedules {
		schedulesInDB[schedule.ID] = true

		if schedule.IsActive {
			// Check if schedule already exists and is running
			if _, exists := s.jobs[schedule.ID]; !exists {
				// New active schedule - add it
				if err := s.AddSchedule(&schedule); err != nil {
					log.Errorf("Failed to add schedule %s: %v", schedule.Name, err)
				} else {
					log.Infof("Added new schedule: %s", schedule.Name)
				}
			}
		} else {
			// Schedule is inactive
			if _, exists := s.jobs[schedule.ID]; exists {
				// Was active, now inactive - remove it
				s.RemoveSchedule(schedule.ID)
				log.Infof("Deactivated schedule: %s", schedule.Name)
			}
		}
	}

	// Remove deleted schedules
	for scheduleID := range s.jobs {
		if !schedulesInDB[scheduleID] {
			s.RemoveSchedule(scheduleID)
			log.Infof("Removed deleted schedule ID: %d", scheduleID)
		}
	}
}

// parseCronExpression parses cron expression to gocron job
func (s *CheckScheduler) parseCronExpression(expr string) (*gocron.Job, error) {
	// Common patterns
	switch expr {
	case "@hourly":
		return s.scheduler.Every(1).Hour(), nil
	case "@daily":
		return s.scheduler.Every(1).Day().At("09:00"), nil
	case "@weekly":
		return s.scheduler.Every(1).Week().At("09:00"), nil
	case "@monthly":
		return s.scheduler.Every(30).Days().At("09:00"), nil
	default:
		// Parse standard cron format
		parts := strings.Fields(expr)

		if len(parts) >= 5 {
			minute := parts[0]
			hour := parts[1]

			// Daily at specific time
			if minute != "*" && hour != "*" && parts[2] == "*" && parts[3] == "*" && parts[4] == "*" {
				m, _ := strconv.Atoi(minute)
				h, _ := strconv.Atoi(hour)
				timeStr := fmt.Sprintf("%02d:%02d", h, m)
				return s.scheduler.Every(1).Day().At(timeStr), nil
			}

			// Every N hours
			if strings.HasPrefix(hour, "*/") {
				intervalStr := strings.TrimPrefix(hour, "*/")
				if interval, err := strconv.Atoi(intervalStr); err == nil && interval > 0 {
					return s.scheduler.Every(uint64(interval)).Hours(), nil
				}
			}

			// Every N minutes
			if strings.HasPrefix(minute, "*/") && hour == "*" {
				intervalStr := strings.TrimPrefix(minute, "*/")
				if interval, err := strconv.Atoi(intervalStr); err == nil && interval > 0 {
					return s.scheduler.Every(uint64(interval)).Minutes(), nil
				}
			}
		}

		// Default to every hour if can't parse
		s.log.Warnf("Could not parse cron expression '%s', defaulting to hourly", expr)
		return s.scheduler.Every(1).Hour(), nil
	}
}

// GetScheduleStatus gets status of all schedules
func (s *CheckScheduler) GetScheduleStatus() []map[string]interface{} {
	var schedules []models.CheckSchedule
	if err := s.db.Find(&schedules).Error; err != nil {
		return nil
	}

	status := make([]map[string]interface{}, 0, len(schedules))

	// Add default interval check status
	intervalMinutes := s.currentInterval
	if intervalMinutes <= 0 {
		intervalMinutes = 60
	}

	status = append(status, map[string]interface{}{
		"id":         0,
		"name":       "Default Interval Check",
		"expression": fmt.Sprintf("Every %d minutes", intervalMinutes),
		"is_active":  s.defaultIntervalJob != nil,
		"last_run":   s.lastCheckTime,
		"next_run":   nil,
		"is_running": s.isCheckingNow,
		"is_default": true,
	})

	// Add custom schedules
	for _, schedule := range schedules {
		item := map[string]interface{}{
			"id":         schedule.ID,
			"name":       schedule.Name,
			"expression": schedule.CronExpression,
			"is_active":  schedule.IsActive,
			"last_run":   schedule.LastRun,
			"next_run":   schedule.NextRun,
			"is_default": false,
		}

		if _, exists := s.jobs[schedule.ID]; exists {
			item["is_running"] = true
		} else {
			item["is_running"] = false
		}

		status = append(status, item)
	}

	return status
}

// IsRunning returns whether the scheduler is running
func (s *CheckScheduler) IsRunning() bool {
	s.runningMutex.RLock()
	defer s.runningMutex.RUnlock()
	return s.isRunning
}
