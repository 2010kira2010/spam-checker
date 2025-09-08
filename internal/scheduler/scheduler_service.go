package scheduler

import (
	"fmt"
	"spam-checker/internal/config"
	"spam-checker/internal/logger"
	"spam-checker/internal/models"
	"spam-checker/internal/services"
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

	// Start scheduler in background
	go s.scheduler.Start()

	// Monitor gateway statuses every 5 minutes
	s.scheduler.Every(5).Minutes().Do(func() {
		adbService := services.NewADBService(s.db, s.cfg)
		if err := adbService.UpdateAllGatewayStatuses(); err != nil {
			log.Errorf("Failed to update gateway statuses: %v", err)
		}
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

	for _, schedule := range schedules {
		if err := s.AddSchedule(&schedule); err != nil {
			log.Errorf("Failed to add schedule %s: %v", schedule.Name, err)
		}
	}
}

// AddSchedule adds a new schedule
func (s *CheckScheduler) AddSchedule(schedule *models.CheckSchedule) error {
	log := s.log.WithFields(logrus.Fields{
		"method": "AddSchedule",
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
		"method": "runScheduledCheck",
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
	// Common patterns
	switch expr {
	case "@hourly":
		return s.scheduler.Every(1).Hour(), nil
	case "@daily":
		return s.scheduler.Every(1).Day().At("09:00"), nil
	case "@weekly":
		return s.scheduler.Every(1).Week().At("09:00"), nil
	default:
		// Parse standard cron format (simplified)
		// Format: "0 */6 * * *" (every 6 hours)
		if expr == "0 */6 * * *" {
			return s.scheduler.Every(6).Hours(), nil
		} else if expr == "0 */12 * * *" {
			return s.scheduler.Every(12).Hours(), nil
		} else if expr == "0 0 * * *" {
			return s.scheduler.Every(1).Day().At("00:00"), nil
		}

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
	for _, schedule := range schedules {
		item := map[string]interface{}{
			"id":         schedule.ID,
			"name":       schedule.Name,
			"expression": schedule.CronExpression,
			"is_active":  schedule.IsActive,
			"last_run":   schedule.LastRun,
			"next_run":   schedule.NextRun,
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
