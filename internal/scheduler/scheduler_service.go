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

	// Fixed: Single check control with proper timing
	checkMutex       sync.Mutex
	isCheckingNow    bool
	lastCheckTime    time.Time
	nextCheckTime    time.Time // Track when next check should occur
	minCheckInterval time.Duration
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
		minCheckInterval:    5 * time.Minute,
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

// canStartCheck checks if we can start a new check with improved timing logic
// This is primarily used for the default interval check, not scheduled checks
func (s *CheckScheduler) canStartCheck() bool {
	s.checkMutex.Lock()
	defer s.checkMutex.Unlock()

	now := time.Now()

	// Check if already checking
	if s.isCheckingNow {
		s.log.WithFields(logrus.Fields{
			"last_check": s.lastCheckTime.Format("15:04:05"),
			"time_since": time.Since(s.lastCheckTime),
		}).Warn("Check already in progress, skipping")
		return false
	}

	// Check if enough time has passed since last check
	timeSinceLastCheck := now.Sub(s.lastCheckTime)
	if timeSinceLastCheck < s.minCheckInterval {
		s.log.WithFields(logrus.Fields{
			"time_since":   timeSinceLastCheck,
			"min_interval": s.minCheckInterval,
			"next_allowed": s.lastCheckTime.Add(s.minCheckInterval).Format("15:04:05"),
		}).Warn("Too soon since last check, skipping")
		return false
	}

	// Check if we're before the scheduled next check time (for default interval check only)
	if !s.nextCheckTime.IsZero() && now.Before(s.nextCheckTime) {
		s.log.WithFields(logrus.Fields{
			"next_scheduled": s.nextCheckTime.Format("15:04:05"),
			"current_time":   now.Format("15:04:05"),
		}).Debug("Not yet time for next check")
		return false
	}

	// Mark as checking
	s.isCheckingNow = true
	s.lastCheckTime = now

	// Calculate next check time based on current interval (for default check)
	if s.currentInterval > 0 {
		s.nextCheckTime = now.Add(time.Duration(s.currentInterval) * time.Minute)
		s.log.WithFields(logrus.Fields{
			"next_check": s.nextCheckTime.Format("15:04:05"),
			"interval":   s.currentInterval,
		}).Info("Next default check scheduled")
	}

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

	// Get schedule details for logging
	var schedule models.CheckSchedule
	if err := s.db.First(&schedule, scheduleID).Error; err == nil {
		log = log.WithFields(logrus.Fields{
			"scheduleName": schedule.Name,
			"expression":   schedule.CronExpression,
		})
	}

	// For scheduled checks, we don't use canStartCheck() because
	// they should run independently of the default interval check
	s.checkMutex.Lock()
	if s.isCheckingNow {
		s.checkMutex.Unlock()
		log.Warn("Another check is already in progress, skipping scheduled check")
		return
	}

	// Mark as checking
	s.isCheckingNow = true
	s.lastCheckTime = time.Now()
	s.checkMutex.Unlock()

	defer func() {
		s.checkMutex.Lock()
		s.isCheckingNow = false
		s.checkMutex.Unlock()
	}()

	log.Infof("Starting scheduled check ID: %d (%s)", scheduleID, schedule.Name)

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
		log.Infof("Scheduled check completed. Next run scheduled for: %s", nextRun.Format("2006-01-02 15:04:05"))
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
	allResults := make(map[uint]*PhoneCheckSummary)
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
				// Check if it's a "already checking" error - don't count as error
				if strings.Contains(err.Error(), "already being checked") {
					log.Debugf("Phone %s is already being checked by another process", phone.Number)
				} else {
					log.Errorf("Failed to check phone %s: %v", phone.Number, err)
					checkErrors = append(checkErrors, err)
				}
			} else {
				successCount++
				// Get latest results for this phone
				summary := s.getPhoneSummary(phone.ID)
				if summary != nil {
					allResults[phone.ID] = summary
					if summary.IsSpam {
						totalSpamCount++
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

		// Small delay between checks to avoid overwhelming the system
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

// PhoneCheckSummary holds summary of check results for a phone
type PhoneCheckSummary struct {
	PhoneNumber string
	IsSpam      bool
	Services    map[string]*ServiceResult
}

// ServiceResult holds result for a specific service
type ServiceResult struct {
	IsSpam   bool
	Keywords []string
}

// getPhoneSummary gets summary of latest check results for a phone
func (s *CheckScheduler) getPhoneSummary(phoneID uint) *PhoneCheckSummary {
	// Get phone details
	var phone models.PhoneNumber
	if err := s.db.First(&phone, phoneID).Error; err != nil {
		return nil
	}

	summary := &PhoneCheckSummary{
		PhoneNumber: phone.Number,
		Services:    make(map[string]*ServiceResult),
	}

	// Get latest check results grouped by service
	var results []models.CheckResult
	subQuery := s.db.Model(&models.CheckResult{}).
		Select("MAX(id) as id").
		Where("phone_number_id = ?", phoneID).
		Group("service_id")

	err := s.db.
		Where("id IN (?)", subQuery).
		Preload("Service").
		Find(&results).Error

	if err != nil {
		s.log.Errorf("Failed to get check results for phone %d: %v", phoneID, err)
		return summary
	}

	// Process results
	for _, result := range results {
		serviceName := result.Service.Name
		if serviceName == "" {
			continue
		}

		summary.Services[serviceName] = &ServiceResult{
			IsSpam:   result.IsSpam,
			Keywords: []string(result.FoundKeywords),
		}

		if result.IsSpam {
			summary.IsSpam = true
		}
	}

	return summary
}

// sendConsolidatedNotification sends a single notification with all results
func (s *CheckScheduler) sendConsolidatedNotification(checkType string, scheduleID uint, spamCount, totalCount int, results map[uint]*PhoneCheckSummary) {
	log := s.log.WithFields(logrus.Fields{
		"method": "sendConsolidatedNotification",
	})

	// Check if notifications are enabled
	var enableNotificationsSetting models.SystemSettings
	enableNotifications := true // Default to true

	if err := s.db.Where("key = ?", "enable_notifications").First(&enableNotificationsSetting).Error; err == nil {
		if enableNotificationsSetting.Value == "false" || enableNotificationsSetting.Value == "0" {
			enableNotifications = false
		}
	}

	if !enableNotifications {
		log.Debug("Notifications are disabled in settings")
		return
	}

	// Check if notifications for spam detection are enabled
	var notifyOnSpamSetting models.SystemSettings
	notifyOnSpam := true // Default to true

	if err := s.db.Where("key = ?", "notify_on_spam_detection").First(&notifyOnSpamSetting).Error; err == nil {
		if notifyOnSpamSetting.Value == "false" || notifyOnSpamSetting.Value == "0" {
			notifyOnSpam = false
		}
	}

	if !notifyOnSpam {
		log.Debug("Spam detection notifications are disabled")
		return
	}

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

	for _, summary := range results {
		if !summary.IsSpam {
			continue
		}

		for serviceName, result := range summary.Services {
			if result.IsSpam {
				phoneInfo := fmt.Sprintf("%s: %v", summary.PhoneNumber, result.Keywords)
				serviceSpamMap[serviceName] = append(serviceSpamMap[serviceName], phoneInfo)
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

	// Send notification with error handling
	if err := s.notificationService.SendNotification(title, message); err != nil {
		// Check if it's a critical error or just a temporary issue
		if strings.Contains(err.Error(), "all notifications failed") {
			log.Errorf("All notification channels failed: %v", err)
		} else if strings.Contains(err.Error(), "config issue") {
			log.Warnf("Notification configuration issue: %v", err)
		} else {
			log.Warnf("Some notifications may have failed: %v", err)
		}

		// Don't fail the entire check process because of notification errors
		// The check results are already saved in the database
	} else {
		log.Info("Notification sent successfully")
	}
}

// Helper function to check if we should send notifications for this check type
func (s *CheckScheduler) shouldSendNotification(checkType string, scheduleID uint) bool {
	// Check global notification setting
	var enableNotificationsSetting models.SystemSettings
	if err := s.db.Where("key = ?", "enable_notifications").First(&enableNotificationsSetting).Error; err == nil {
		if enableNotificationsSetting.Value == "false" || enableNotificationsSetting.Value == "0" {
			return false
		}
	}

	// Check specific settings for check type
	switch checkType {
	case "default":
		var notifyDefaultSetting models.SystemSettings
		if err := s.db.Where("key = ?", "notify_default_checks").First(&notifyDefaultSetting).Error; err == nil {
			return notifyDefaultSetting.Value != "false" && notifyDefaultSetting.Value != "0"
		}
	case "scheduled":
		// Could check per-schedule notification settings here if needed
		return true
	}

	return true
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

	// Set next check time
	s.checkMutex.Lock()
	s.nextCheckTime = time.Now().Add(time.Duration(intervalMinutes) * time.Minute)
	s.checkMutex.Unlock()

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

	log.Infof("Added schedule: %s (%s), next run: %s",
		schedule.Name, schedule.CronExpression, nextRun.Format("2006-01-02 15:04:05"))

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

	log.Debugf("Found %d schedules in database", len(schedules))

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
			} else {
				log.Debugf("Schedule %s already active", schedule.Name)
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

	// Log current active schedules
	log.Infof("Active schedules: %d", len(s.jobs))
	for id, job := range s.jobs {
		var schedule models.CheckSchedule
		if err := s.db.First(&schedule, id).Error; err == nil {
			nextRun := job.NextScheduledTime()
			log.Debugf("  - %s: next run at %s", schedule.Name, nextRun.Format("2006-01-02 15:04:05"))
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
		// Check for custom formats first
		if strings.HasPrefix(expr, "WEEKLY:") {
			// Format: WEEKLY:DAY:HH:MM (e.g., WEEKLY:5:16:30 = Friday at 16:30)
			parts := strings.Split(expr, ":")
			if len(parts) == 4 {
				day, dayErr := strconv.Atoi(parts[1])
				hour, hourErr := strconv.Atoi(parts[2])
				minute, minuteErr := strconv.Atoi(parts[3])

				if dayErr == nil && hourErr == nil && minuteErr == nil &&
					day >= 0 && day <= 7 && hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59 {

					timeStr := fmt.Sprintf("%02d:%02d", hour, minute)
					var job *gocron.Job

					switch day {
					case 0, 7:
						job = s.scheduler.Every(1).Sunday()
					case 1:
						job = s.scheduler.Every(1).Monday()
					case 2:
						job = s.scheduler.Every(1).Tuesday()
					case 3:
						job = s.scheduler.Every(1).Wednesday()
					case 4:
						job = s.scheduler.Every(1).Thursday()
					case 5:
						job = s.scheduler.Every(1).Friday()
					case 6:
						job = s.scheduler.Every(1).Saturday()
					}

					if job != nil {
						job = job.At(timeStr)
						s.log.Debugf("Creating job: Weekly on day %d at %s", day, timeStr)
						return job, nil
					}
				}
			}
		} else if strings.HasPrefix(expr, "DAILY:") {
			// Format: DAILY:HH:MM (e.g., DAILY:14:30)
			parts := strings.Split(expr, ":")
			if len(parts) == 3 {
				hour, hourErr := strconv.Atoi(parts[1])
				minute, minuteErr := strconv.Atoi(parts[2])

				if hourErr == nil && minuteErr == nil &&
					hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59 {

					timeStr := fmt.Sprintf("%02d:%02d", hour, minute)
					s.log.Debugf("Creating job: Daily at %s", timeStr)
					return s.scheduler.Every(1).Day().At(timeStr), nil
				}
			}
		} else if strings.HasPrefix(expr, "INTERVAL:") {
			// Format: INTERVAL:MINUTES (e.g., INTERVAL:10 = every 10 minutes)
			parts := strings.Split(expr, ":")
			if len(parts) == 2 {
				minutes, err := strconv.Atoi(parts[1])
				if err == nil && minutes > 0 {
					s.log.Debugf("Creating job: Every %d minutes", minutes)
					return s.scheduler.Every(uint64(minutes)).Minutes(), nil
				}
			}
		}

		// Parse standard cron format
		parts := strings.Fields(expr)

		if len(parts) >= 5 {
			minute := parts[0]
			hour := parts[1]
			dayOfMonth := parts[2]
			month := parts[3]
			dayOfWeek := parts[4]

			// Every N minutes (e.g., */10 * * * *)
			if strings.HasPrefix(minute, "*/") && hour == "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
				intervalStr := strings.TrimPrefix(minute, "*/")
				if interval, err := strconv.Atoi(intervalStr); err == nil && interval > 0 {
					s.log.Debugf("Creating job: Every %d minutes", interval)
					return s.scheduler.Every(uint64(interval)).Minutes(), nil
				}
			}

			// Every N hours (e.g., 0 */6 * * *)
			if minute == "0" && strings.HasPrefix(hour, "*/") && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
				intervalStr := strings.TrimPrefix(hour, "*/")
				if interval, err := strconv.Atoi(intervalStr); err == nil && interval > 0 {
					s.log.Debugf("Creating job: Every %d hours", interval)
					return s.scheduler.Every(uint64(interval)).Hours(), nil
				}
			}

			// Specific minute every hour (e.g., 30 * * * *)
			if minute != "*" && !strings.Contains(minute, "/") && hour == "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
				if m, err := strconv.Atoi(minute); err == nil && m >= 0 && m <= 59 {
					s.log.Debugf("Creating job: At minute %d of every hour", m)
					// gocron doesn't support "at minute X of every hour" directly, so we use hourly
					return s.scheduler.Every(1).Hour(), nil
				}
			}

			// Daily at specific time (e.g., 30 14 * * *)
			if minute != "*" && hour != "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
				m, mErr := strconv.Atoi(minute)
				h, hErr := strconv.Atoi(hour)
				if mErr == nil && hErr == nil && m >= 0 && m <= 59 && h >= 0 && h <= 23 {
					timeStr := fmt.Sprintf("%02d:%02d", h, m)
					s.log.Debugf("Creating job: Daily at %s", timeStr)
					return s.scheduler.Every(1).Day().At(timeStr), nil
				}
			}

			// Weekly on specific day (simplified - just check if dayOfWeek is a number)
			if dayOfWeek != "*" && dayOfMonth == "*" {
				if dow, err := strconv.Atoi(dayOfWeek); err == nil && dow >= 0 && dow <= 7 {
					// Map cron day (0-7, where 0 and 7 are Sunday) to gocron
					var job *gocron.Job
					switch dow {
					case 0, 7:
						job = s.scheduler.Every(1).Sunday()
					case 1:
						job = s.scheduler.Every(1).Monday()
					case 2:
						job = s.scheduler.Every(1).Tuesday()
					case 3:
						job = s.scheduler.Every(1).Wednesday()
					case 4:
						job = s.scheduler.Every(1).Thursday()
					case 5:
						job = s.scheduler.Every(1).Friday()
					case 6:
						job = s.scheduler.Every(1).Saturday()
					}

					if job != nil {
						// Set time if specified
						if minute != "*" && hour != "*" {
							m, _ := strconv.Atoi(minute)
							h, _ := strconv.Atoi(hour)
							timeStr := fmt.Sprintf("%02d:%02d", h, m)
							job = job.At(timeStr)
						}
						s.log.Debugf("Creating job: Weekly on day %d", dow)
						return job, nil
					}
				}
			}

			// Every N minutes with offset (e.g., 5,15,25,35,45,55 * * * * for every 10 minutes starting at 5)
			if strings.Contains(minute, ",") && hour == "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
				// Try to detect pattern
				minutes := strings.Split(minute, ",")
				if len(minutes) >= 2 {
					// Check if it's a regular interval
					firstMin, _ := strconv.Atoi(minutes[0])
					secondMin, _ := strconv.Atoi(minutes[1])
					interval := secondMin - firstMin

					isRegular := true
					for i := 1; i < len(minutes)-1; i++ {
						curr, _ := strconv.Atoi(minutes[i])
						next, _ := strconv.Atoi(minutes[i+1])
						if next-curr != interval {
							isRegular = false
							break
						}
					}

					if isRegular && interval > 0 {
						s.log.Debugf("Creating job: Every %d minutes (starting at minute %d)", interval, firstMin)
						return s.scheduler.Every(uint64(interval)).Minutes(), nil
					}
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

	s.checkMutex.Lock()
	nextCheck := s.nextCheckTime
	lastCheck := s.lastCheckTime
	isChecking := s.isCheckingNow
	s.checkMutex.Unlock()

	status = append(status, map[string]interface{}{
		"id":         0,
		"name":       "Default Interval Check",
		"expression": fmt.Sprintf("Every %d minutes", intervalMinutes),
		"is_active":  s.defaultIntervalJob != nil,
		"last_run":   lastCheck,
		"next_run":   nextCheck,
		"is_running": isChecking,
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
