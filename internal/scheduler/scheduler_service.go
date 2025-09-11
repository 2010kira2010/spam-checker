package scheduler

import (
	"fmt"
	"spam-checker/internal/config"
	"spam-checker/internal/logger"
	"spam-checker/internal/models"
	"spam-checker/internal/services"
	"strconv"
	"strings"
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
	defaultIntervalJob  *gocron.Job // Job for default interval from settings
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
	}
}

// Start starts the scheduler
func (s *CheckScheduler) Start() {
	log := s.log.WithFields(logrus.Fields{
		"method": "Start",
	})
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

	// Reload schedules every minute to pick up changes
	s.scheduler.Every(1).Minutes().Do(func() {
		s.reloadSchedules()
	})

	log.Info("Check scheduler started successfully")
}

// Stop stops the scheduler
func (s *CheckScheduler) Stop() {
	log := s.log.WithFields(logrus.Fields{
		"method": "Stop",
	})
	log.Info("Stopping check scheduler...")
	s.scheduler.Clear()
	log.Info("Check scheduler stopped")
}

// startDefaultIntervalCheck starts the default interval check based on settings
func (s *CheckScheduler) startDefaultIntervalCheck() {
	log := s.log.WithFields(logrus.Fields{
		"method": "startDefaultIntervalCheck",
	})

	// Get check interval from settings
	var setting models.SystemSettings
	if err := s.db.Where("key = ?", "check_interval_minutes").First(&setting).Error; err != nil {
		log.Warnf("Failed to get check_interval_minutes setting, using default 60 minutes")
		// Default to 60 minutes if setting not found
		job := s.scheduler.Every(60).Minutes()
		job.Do(s.runDefaultCheck)
		s.defaultIntervalJob = job
		return
	}

	intervalMinutes, err := strconv.Atoi(setting.Value)
	if err != nil || intervalMinutes <= 0 {
		log.Warnf("Invalid check_interval_minutes value: %s, using default 60 minutes", setting.Value)
		intervalMinutes = 60
	}

	log.Infof("Starting default interval check every %d minutes", intervalMinutes)

	// Remove old job if exists
	if s.defaultIntervalJob != nil {
		s.scheduler.Remove(s.defaultIntervalJob)
	}

	// Create new job
	job := s.scheduler.Every(uint64(intervalMinutes)).Minutes()
	job.Do(s.runDefaultCheck)
	s.defaultIntervalJob = job
}

// runDefaultCheck runs the default interval check
func (s *CheckScheduler) runDefaultCheck() {
	log := s.log.WithFields(logrus.Fields{
		"method": "runDefaultCheck",
	})

	log.Info("Running default interval check")

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

	// Check each phone
	for _, phone := range phones {
		if err := s.checkService.CheckPhoneNumber(phone.ID); err != nil {
			log.Errorf("Failed to check phone %s: %v", phone.Number, err)
			continue
		}

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

		// Small delay between checks
		time.Sleep(2 * time.Second)
	}

	// Send notification if spam found
	if spamCount > 0 {
		s.sendNotification(spamCount, len(phones), results)
	}

	log.Infof("Default interval check completed. Checked %d phones, found %d spam", len(phones), spamCount)
}

// reloadSchedules reloads schedules from database
func (s *CheckScheduler) reloadSchedules() {
	log := s.log.WithFields(logrus.Fields{
		"method": "reloadSchedules",
	})

	// Check if check_interval_minutes has changed
	var setting models.SystemSettings
	if err := s.db.Where("key = ?", "check_interval_minutes").First(&setting).Error; err == nil {
		intervalMinutes, err := strconv.Atoi(setting.Value)
		if err == nil && intervalMinutes > 0 {
			// Restart the default interval check with new interval
			// Since we can't easily check the current interval, we'll just restart it
			s.startDefaultIntervalCheck()
		}
	}

	// Reload custom schedules
	var schedules []models.CheckSchedule
	if err := s.db.Find(&schedules).Error; err != nil {
		log.Errorf("Failed to load schedules: %v", err)
		return
	}

	// Check for new or updated schedules
	for _, schedule := range schedules {
		if _, exists := s.jobs[schedule.ID]; !exists && schedule.IsActive {
			// New schedule
			if err := s.AddSchedule(&schedule); err != nil {
				log.Errorf("Failed to add schedule %s: %v", schedule.Name, err)
			}
		} else if !schedule.IsActive && s.jobs[schedule.ID] != nil {
			// Deactivated schedule
			s.RemoveSchedule(schedule.ID)
		}
	}

	// Remove deleted schedules
	for scheduleID := range s.jobs {
		found := false
		for _, schedule := range schedules {
			if schedule.ID == scheduleID {
				found = true
				break
			}
		}
		if !found {
			s.RemoveSchedule(scheduleID)
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
		"method": "RemoveSchedule",
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

// runScheduledCheck runs a scheduled check
func (s *CheckScheduler) runScheduledCheck(scheduleID uint) {
	log := s.log.WithFields(logrus.Fields{
		"method":     "runScheduledCheck",
		"scheduleID": scheduleID,
	})

	log.Infof("Running scheduled check ID: %d", scheduleID)

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

	// Check each phone
	for _, phone := range phones {
		if err := s.checkService.CheckPhoneNumber(phone.ID); err != nil {
			log.Errorf("Failed to check phone %s: %v", phone.Number, err)
			continue
		}

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

		// Small delay between checks
		time.Sleep(2 * time.Second)
	}

	// Send notification if spam found
	if spamCount > 0 {
		s.sendNotification(spamCount, len(phones), results)
	}

	// Update next run time
	if job, exists := s.jobs[scheduleID]; exists {
		nextRun := job.NextScheduledTime()
		s.db.Model(&models.CheckSchedule{}).Where("id = ?", scheduleID).Update("next_run", &nextRun)
	}

	log.Infof("Scheduled check completed. Checked %d phones, found %d spam", len(phones), spamCount)
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
		// Parse standard cron format
		// Format: "minute hour day month weekday"
		parts := strings.Fields(expr)

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

// GetScheduleStatus gets status of all schedules
func (s *CheckScheduler) GetScheduleStatus() []map[string]interface{} {
	var schedules []models.CheckSchedule
	if err := s.db.Find(&schedules).Error; err != nil {
		return nil
	}

	status := make([]map[string]interface{}, 0, len(schedules))

	// Add default interval check status
	var setting models.SystemSettings
	intervalMinutes := 60 // default
	if err := s.db.Where("key = ?", "check_interval_minutes").First(&setting).Error; err == nil {
		if val, err := strconv.Atoi(setting.Value); err == nil && val > 0 {
			intervalMinutes = val
		}
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
