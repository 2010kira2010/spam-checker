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
	currentInterval     int // Track current interval to avoid recreating
	isRunning           bool
	runningMutex        sync.Mutex
	stopChan            chan struct{}
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
		currentInterval:     -1, // Initialize with invalid value
		isRunning:           false,
		stopChan:            make(chan struct{}),
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

	log.Info("Check scheduler stopped")
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

	// Reload custom schedules (check for changes)
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

	log.Infof("Setting default interval check to every %d minutes", intervalMinutes)

	// Create new job
	job := s.scheduler.Every(uint64(intervalMinutes)).Minutes()
	job.Do(s.runDefaultCheckSafe) // Use safe wrapper
	s.defaultIntervalJob = job
}

// runDefaultCheckSafe is a wrapper that prevents concurrent execution
func (s *CheckScheduler) runDefaultCheckSafe() {
	// Use a non-blocking check to prevent duplicate runs
	select {
	case <-s.stopChan:
		// Scheduler is stopping
		return
	default:
		// Continue with check
	}

	// Delegate to actual check method
	s.runDefaultCheck()
}

// runDefaultCheck runs the default interval check
func (s *CheckScheduler) runDefaultCheck() {
	log := s.log.WithFields(logrus.Fields{
		"method": "runDefaultCheck",
	})

	log.Info("Starting default interval check")
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

	// Track results for notification
	results := make(map[uint][]models.CheckResult)
	spamCount := 0
	var checkErrors []error

	// Check each phone with a timeout
	checkTimeout := 30 * time.Second // Timeout per phone
	for _, phone := range phones {
		// Check if we're stopping
		select {
		case <-s.stopChan:
			log.Info("Scheduler stopping, aborting default check")
			return
		default:
		}

		// Create a channel for the check result
		done := make(chan error, 1)

		go func(p models.PhoneNumber) {
			done <- s.checkService.CheckPhoneNumber(p.ID)
		}(phone)

		// Wait for check with timeout
		select {
		case err := <-done:
			if err != nil {
				log.Errorf("Failed to check phone %s: %v", phone.Number, err)
				checkErrors = append(checkErrors, err)
			} else {
				// Get latest results
				checkResults, err := s.checkService.GetCheckResults(phone.ID, 0, 3)
				if err == nil && len(checkResults) > 0 {
					results[phone.ID] = checkResults
					for _, result := range checkResults {
						if result.IsSpam {
							spamCount++
							break
						}
					}
				}
			}
		case <-time.After(checkTimeout):
			log.Warnf("Check timeout for phone %s", phone.Number)
			checkErrors = append(checkErrors, fmt.Errorf("timeout checking phone %s", phone.Number))
		case <-s.stopChan:
			log.Info("Scheduler stopping, aborting default check")
			return
		}

		// Small delay between checks to avoid overload
		time.Sleep(2 * time.Second)
	}

	// Calculate duration
	duration := time.Since(startTime)

	// Send notification if spam found
	if spamCount > 0 {
		s.sendNotification(spamCount, len(phones), results)
	}

	log.Infof("Default interval check completed in %v. Checked %d phones, found %d spam, %d errors",
		duration, len(phones), spamCount, len(checkErrors))
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
			// If it exists and is running, leave it as is
		} else {
			// Schedule is inactive
			if _, exists := s.jobs[schedule.ID]; exists {
				// Was active, now inactive - remove it
				s.RemoveSchedule(schedule.ID)
				log.Infof("Deactivated schedule: %s", schedule.Name)
			}
		}
	}

	// Remove deleted schedules (those not in DB anymore)
	for scheduleID := range s.jobs {
		if !schedulesInDB[scheduleID] {
			s.RemoveSchedule(scheduleID)
			log.Infof("Removed deleted schedule ID: %d", scheduleID)
		}
	}
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

	// Set job function with safe wrapper
	job.Do(s.runScheduledCheckSafe, schedule.ID)

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

// UpdateSchedule updates an existing schedule
func (s *CheckScheduler) UpdateSchedule(schedule *models.CheckSchedule) error {
	if schedule.IsActive {
		return s.AddSchedule(schedule)
	} else {
		s.RemoveSchedule(schedule.ID)
		return nil
	}
}

// runScheduledCheckSafe is a wrapper that prevents concurrent execution
func (s *CheckScheduler) runScheduledCheckSafe(scheduleID uint) {
	// Check if scheduler is stopping
	select {
	case <-s.stopChan:
		return
	default:
	}

	s.runScheduledCheck(scheduleID)
}

// runScheduledCheck runs a scheduled check
func (s *CheckScheduler) runScheduledCheck(scheduleID uint) {
	log := s.log.WithFields(logrus.Fields{
		"method":     "runScheduledCheck",
		"scheduleID": scheduleID,
	})

	log.Infof("Starting scheduled check ID: %d", scheduleID)
	startTime := time.Now()

	// Update last run time
	now := time.Now()
	if err := s.db.Model(&models.CheckSchedule{}).Where("id = ?", scheduleID).Update("last_run", &now).Error; err != nil {
		log.Errorf("Failed to update last run time: %v", err)
	}

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

	// Track results for notification
	results := make(map[uint][]models.CheckResult)
	spamCount := 0
	var checkErrors []error

	// Check each phone with timeout
	checkTimeout := 30 * time.Second
	for _, phone := range phones {
		// Check if we're stopping
		select {
		case <-s.stopChan:
			log.Info("Scheduler stopping, aborting scheduled check")
			return
		default:
		}

		// Create a channel for the check result
		done := make(chan error, 1)

		go func(p models.PhoneNumber) {
			done <- s.checkService.CheckPhoneNumber(p.ID)
		}(phone)

		// Wait for check with timeout
		select {
		case err := <-done:
			if err != nil {
				log.Errorf("Failed to check phone %s: %v", phone.Number, err)
				checkErrors = append(checkErrors, err)
			} else {
				// Get latest results
				checkResults, err := s.checkService.GetCheckResults(phone.ID, 0, 3)
				if err == nil && len(checkResults) > 0 {
					results[phone.ID] = checkResults
					for _, result := range checkResults {
						if result.IsSpam {
							spamCount++
							break
						}
					}
				}
			}
		case <-time.After(checkTimeout):
			log.Warnf("Check timeout for phone %s", phone.Number)
			checkErrors = append(checkErrors, fmt.Errorf("timeout checking phone %s", phone.Number))
		case <-s.stopChan:
			log.Info("Scheduler stopping, aborting scheduled check")
			return
		}

		// Small delay between checks
		time.Sleep(2 * time.Second)
	}

	// Calculate duration
	duration := time.Since(startTime)

	// Send notification if spam found
	if spamCount > 0 {
		s.sendNotification(spamCount, len(phones), results)
	}

	// Update next run time
	if job, exists := s.jobs[scheduleID]; exists {
		nextRun := job.NextScheduledTime()
		s.db.Model(&models.CheckSchedule{}).Where("id = ?", scheduleID).Update("next_run", &nextRun)
	}

	log.Infof("Scheduled check completed in %v. Checked %d phones, found %d spam, %d errors",
		duration, len(phones), spamCount, len(checkErrors))
}

// sendNotification sends notification about check results
func (s *CheckScheduler) sendNotification(spamCount, totalCount int, results map[uint][]models.CheckResult) {
	log := s.log.WithFields(logrus.Fields{
		"method": "sendNotification",
	})

	message := fmt.Sprintf(
		"🔍 Check Results\n\n"+
			"Total phones checked: %d\n"+
			"Spam detected: %d\n"+
			"Clean: %d\n",
		totalCount, spamCount, totalCount-spamCount,
	)

	// Add details about spam phones
	if spamCount > 0 {
		message += "\n⚠️ Spam Numbers:\n"
		for phoneID, checkResults := range results {
			for _, result := range checkResults {
				if result.IsSpam {
					var phone models.PhoneNumber
					if err := s.db.First(&phone, phoneID).Error; err == nil {
						message += fmt.Sprintf("• %s (%s): %v\n",
							phone.Number,
							result.Service.Name,
							result.FoundKeywords,
						)
					}
					break
				}
			}
		}
	}

	// Send to all active notification channels
	if err := s.notificationService.SendNotification("Check Results", message); err != nil {
		log.Errorf("Failed to send notification: %v", err)
	}
}

// parseCronExpression parses cron expression to gocron job
func (s *CheckScheduler) parseCronExpression(expr string) (*gocron.Job, error) {
	log := s.log.WithFields(logrus.Fields{
		"method":     "parseCronExpression",
		"expression": expr,
	})

	// Common patterns
	switch expr {
	case "@hourly":
		return s.scheduler.Every(1).Hour(), nil
	case "@daily":
		return s.scheduler.Every(1).Day().At("09:00"), nil
	case "@weekly":
		return s.scheduler.Every(1).Week().At("09:00"), nil
	case "@monthly":
		// gocron doesn't support monthly directly, use 30 days
		return s.scheduler.Every(30).Days().At("09:00"), nil
	default:
		// Parse standard cron format and custom formats
		parts := strings.Fields(expr)

		// Check for weekly schedule with time (e.g., "WEEKLY:1,3,5:14:30" - Monday, Wednesday, Friday at 14:30)
		if strings.HasPrefix(expr, "WEEKLY:") {
			return s.parseWeeklySchedule(expr)
		}

		// Check for daily schedule with time (e.g., "DAILY:14:30")
		if strings.HasPrefix(expr, "DAILY:") {
			return s.parseDailySchedule(expr)
		}

		if len(parts) < 5 {
			// Try to parse simple formats
			if strings.HasPrefix(expr, "*/") {
				// Every N minutes/hours format
				if strings.Contains(expr, "* * * *") {
					// Extract interval
					intervalStr := strings.TrimPrefix(strings.Split(expr, " ")[1], "*/")
					if interval, err := strconv.Atoi(intervalStr); err == nil && interval > 0 {
						log.Infof("Parsed as every %d hours", interval)
						return s.scheduler.Every(uint64(interval)).Hours(), nil
					}
				} else if strings.Contains(expr, "* * *") {
					// Minutes format
					intervalStr := strings.TrimPrefix(strings.Split(expr, " ")[0], "*/")
					if interval, err := strconv.Atoi(intervalStr); err == nil && interval > 0 {
						log.Infof("Parsed as every %d minutes", interval)
						return s.scheduler.Every(uint64(interval)).Minutes(), nil
					}
				}
			}
		}

		// Handle specific cron patterns
		minute := parts[0]
		hour := parts[1]

		// Daily at specific time
		if minute != "*" && hour != "*" && len(parts) >= 5 && parts[2] == "*" && parts[3] == "*" && parts[4] == "*" {
			// Format: "30 14 * * *" - daily at 14:30
			m, _ := strconv.Atoi(minute)
			h, _ := strconv.Atoi(hour)
			timeStr := fmt.Sprintf("%02d:%02d", h, m)
			log.Infof("Parsed as daily at %s", timeStr)
			return s.scheduler.Every(1).Day().At(timeStr), nil
		}

		// Weekly at specific day and time
		if minute != "*" && hour != "*" && len(parts) >= 5 && parts[2] == "*" && parts[3] == "*" && parts[4] != "*" {
			// Format: "30 14 * * 1" - every Monday at 14:30
			m, _ := strconv.Atoi(minute)
			h, _ := strconv.Atoi(hour)
			dayOfWeek, _ := strconv.Atoi(parts[4])
			timeStr := fmt.Sprintf("%02d:%02d", h, m)

			return s.parseWeekdaySchedule(dayOfWeek, timeStr)
		}

		// Every N hours at specific minute
		if minute != "*" && strings.HasPrefix(hour, "*/") {
			// Format: "0 */6 * * *" - every 6 hours at minute 0
			intervalStr := strings.TrimPrefix(hour, "*/")
			if interval, err := strconv.Atoi(intervalStr); err == nil && interval > 0 {
				log.Infof("Parsed as every %d hours", interval)
				return s.scheduler.Every(uint64(interval)).Hours(), nil
			}
		}

		// Every N minutes
		if strings.HasPrefix(minute, "*/") && hour == "*" {
			// Format: "*/30 * * * *" - every 30 minutes
			intervalStr := strings.TrimPrefix(minute, "*/")
			if interval, err := strconv.Atoi(intervalStr); err == nil && interval > 0 {
				log.Infof("Parsed as every %d minutes", interval)
				return s.scheduler.Every(uint64(interval)).Minutes(), nil
			}
		}

		// Specific hour every day
		if minute == "0" && hour != "*" && !strings.Contains(hour, "/") {
			// Format: "0 14 * * *" - daily at 14:00
			h, _ := strconv.Atoi(hour)
			timeStr := fmt.Sprintf("%02d:00", h)
			log.Infof("Parsed as daily at %s", timeStr)
			return s.scheduler.Every(1).Day().At(timeStr), nil
		}

		log.Warnf("Could not parse cron expression '%s', defaulting to hourly", expr)
		// Default to every hour if can't parse
		return s.scheduler.Every(1).Hour(), nil
	}
}

// parseWeeklySchedule parses weekly schedule format
// Format: "WEEKLY:1,3,5:14:30" - Monday(1), Wednesday(3), Friday(5) at 14:30
func (s *CheckScheduler) parseWeeklySchedule(expr string) (*gocron.Job, error) {
	parts := strings.Split(strings.TrimPrefix(expr, "WEEKLY:"), ":")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid weekly schedule format: %s", expr)
	}

	daysStr := parts[0]
	hour := parts[1]
	minute := parts[2]

	// Parse time
	h, err := strconv.Atoi(hour)
	if err != nil || h < 0 || h > 23 {
		return nil, fmt.Errorf("invalid hour in weekly schedule: %s", hour)
	}

	m, err := strconv.Atoi(minute)
	if err != nil || m < 0 || m > 59 {
		return nil, fmt.Errorf("invalid minute in weekly schedule: %s", minute)
	}

	timeStr := fmt.Sprintf("%02d:%02d", h, m)

	// Parse days
	daysParts := strings.Split(daysStr, ",")
	if len(daysParts) == 0 {
		return nil, fmt.Errorf("no days specified in weekly schedule")
	}

	// For gocron, we need to create separate jobs for each day
	// We'll use the first day and note that gocron has limitations here
	firstDay, err := strconv.Atoi(daysParts[0])
	if err != nil || firstDay < 0 || firstDay > 6 {
		return nil, fmt.Errorf("invalid day in weekly schedule: %s", daysParts[0])
	}

	return s.parseWeekdaySchedule(firstDay, timeStr)
}

// parseDailySchedule parses daily schedule format
// Format: "DAILY:14:30" - Every day at 14:30
func (s *CheckScheduler) parseDailySchedule(expr string) (*gocron.Job, error) {
	parts := strings.Split(strings.TrimPrefix(expr, "DAILY:"), ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid daily schedule format: %s", expr)
	}

	hour := parts[0]
	minute := parts[1]

	h, err := strconv.Atoi(hour)
	if err != nil || h < 0 || h > 23 {
		return nil, fmt.Errorf("invalid hour in daily schedule: %s", hour)
	}

	m, err := strconv.Atoi(minute)
	if err != nil || m < 0 || m > 59 {
		return nil, fmt.Errorf("invalid minute in daily schedule: %s", minute)
	}

	timeStr := fmt.Sprintf("%02d:%02d", h, m)
	s.log.Infof("Parsed as daily at %s", timeStr)

	return s.scheduler.Every(1).Day().At(timeStr), nil
}

// parseWeekdaySchedule creates a job for specific weekday and time
func (s *CheckScheduler) parseWeekdaySchedule(dayOfWeek int, timeStr string) (*gocron.Job, error) {
	// Map cron day (0-6, 0=Sunday) to gocron day
	var job *gocron.Job
	switch dayOfWeek {
	case 0:
		job = s.scheduler.Every(1).Sunday().At(timeStr)
	case 1:
		job = s.scheduler.Every(1).Monday().At(timeStr)
	case 2:
		job = s.scheduler.Every(1).Tuesday().At(timeStr)
	case 3:
		job = s.scheduler.Every(1).Wednesday().At(timeStr)
	case 4:
		job = s.scheduler.Every(1).Thursday().At(timeStr)
	case 5:
		job = s.scheduler.Every(1).Friday().At(timeStr)
	case 6:
		job = s.scheduler.Every(1).Saturday().At(timeStr)
	default:
		return nil, fmt.Errorf("invalid day of week: %d", dayOfWeek)
	}

	s.log.Infof("Parsed as weekly on day %d at %s", dayOfWeek, timeStr)
	return job, nil
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
		intervalMinutes = 60 // default
	}

	status = append(status, map[string]interface{}{
		"id":         0,
		"name":       "Default Interval Check",
		"expression": fmt.Sprintf("Every %d minutes", intervalMinutes),
		"is_active":  s.defaultIntervalJob != nil,
		"last_run":   nil,
		"next_run":   nil,
		"is_running": s.defaultIntervalJob != nil,
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

		// Check if job exists
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
	s.runningMutex.Lock()
	defer s.runningMutex.Unlock()
	return s.isRunning
}
