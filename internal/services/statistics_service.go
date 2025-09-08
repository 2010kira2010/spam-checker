package services

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"spam-checker/internal/logger"
	"spam-checker/internal/models"
	"time"

	"gorm.io/gorm"
)

type StatisticsService struct {
	db  *gorm.DB
	log *logrus.Entry
}

func NewStatisticsService(db *gorm.DB) *StatisticsService {
	return &StatisticsService{
		db:  db,
		log: logger.WithField("service", "StatisticsService"),
	}
}

// GetOverviewStats gets general overview statistics
func (s *StatisticsService) GetOverviewStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total phones
	var totalPhones int64
	if err := s.db.Model(&models.PhoneNumber{}).Count(&totalPhones).Error; err != nil {
		return nil, fmt.Errorf("failed to count phones: %w", err)
	}
	stats["total_phones"] = totalPhones

	// Active phones
	var activePhones int64
	if err := s.db.Model(&models.PhoneNumber{}).Where("is_active = ?", true).Count(&activePhones).Error; err != nil {
		return nil, fmt.Errorf("failed to count active phones: %w", err)
	}
	stats["active_phones"] = activePhones

	// Total checks
	var totalChecks int64
	if err := s.db.Model(&models.CheckResult{}).Count(&totalChecks).Error; err != nil {
		return nil, fmt.Errorf("failed to count checks: %w", err)
	}
	stats["total_checks"] = totalChecks

	// Spam detections
	var spamDetections int64
	if err := s.db.Model(&models.CheckResult{}).Where("is_spam = ?", true).Count(&spamDetections).Error; err != nil {
		return nil, fmt.Errorf("failed to count spam detections: %w", err)
	}
	stats["spam_detections"] = spamDetections

	// Calculate spam rate
	spamRate := float64(0)
	if totalChecks > 0 {
		spamRate = float64(spamDetections) / float64(totalChecks) * 100
	}
	stats["spam_rate"] = spamRate

	// Services stats
	var services []models.SpamService
	if err := s.db.Find(&services).Error; err != nil {
		return nil, fmt.Errorf("failed to get services: %w", err)
	}
	stats["total_services"] = len(services)

	// Active gateways
	var activeGateways int64
	if err := s.db.Model(&models.ADBGateway{}).Where("status = ?", "online").Count(&activeGateways).Error; err != nil {
		return nil, fmt.Errorf("failed to count active gateways: %w", err)
	}
	stats["active_gateways"] = activeGateways

	return stats, nil
}

// GetTimeSeriesStats gets statistics for time series charts
func (s *StatisticsService) GetTimeSeriesStats(days int) ([]map[string]interface{}, error) {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	// Get all check results in the date range
	var results []models.CheckResult
	if err := s.db.Where("checked_at >= ? AND checked_at <= ?", startDate, endDate).Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get check results: %w", err)
	}

	// Group by date manually
	dailyStats := make(map[string]map[string]int)

	for _, result := range results {
		dateKey := result.CheckedAt.Format("2006-01-02")

		if dailyStats[dateKey] == nil {
			dailyStats[dateKey] = map[string]int{
				"total_checks": 0,
				"spam_count":   0,
				"clean_count":  0,
			}
		}

		dailyStats[dateKey]["total_checks"]++
		if result.IsSpam {
			dailyStats[dateKey]["spam_count"]++
		} else {
			dailyStats[dateKey]["clean_count"]++
		}
	}

	// Convert to sorted array
	stats := make([]map[string]interface{}, 0)

	// Generate all dates in range
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateKey := d.Format("2006-01-02")

		dayData := dailyStats[dateKey]
		if dayData == nil {
			// No data for this day
			stats = append(stats, map[string]interface{}{
				"date":         dateKey,
				"total_checks": 0,
				"spam_count":   0,
				"clean_count":  0,
				"spam_rate":    float64(0),
			})
		} else {
			spamRate := float64(0)
			if dayData["total_checks"] > 0 {
				spamRate = float64(dayData["spam_count"]) / float64(dayData["total_checks"]) * 100
			}

			stats = append(stats, map[string]interface{}{
				"date":         dateKey,
				"total_checks": dayData["total_checks"],
				"spam_count":   dayData["spam_count"],
				"clean_count":  dayData["clean_count"],
				"spam_rate":    spamRate,
			})
		}
	}

	return stats, nil
}

// GetServiceStats gets statistics by service
func (s *StatisticsService) GetServiceStats() ([]map[string]interface{}, error) {
	var services []models.SpamService
	if err := s.db.Find(&services).Error; err != nil {
		return nil, fmt.Errorf("failed to get services: %w", err)
	}

	stats := make([]map[string]interface{}, 0)

	for _, service := range services {
		var totalChecks int64
		var spamCount int64

		// Count total checks for this service
		if err := s.db.Model(&models.CheckResult{}).Where("service_id = ?", service.ID).Count(&totalChecks).Error; err != nil {
			return nil, fmt.Errorf("failed to count checks for service %s: %w", service.Name, err)
		}

		// Count spam detections for this service
		if err := s.db.Model(&models.CheckResult{}).Where("service_id = ? AND is_spam = ?", service.ID, true).Count(&spamCount).Error; err != nil {
			return nil, fmt.Errorf("failed to count spam for service %s: %w", service.Name, err)
		}

		spamRate := float64(0)
		if totalChecks > 0 {
			spamRate = float64(spamCount) / float64(totalChecks) * 100
		}

		stats = append(stats, map[string]interface{}{
			"service_id":   service.ID,
			"service_name": service.Name,
			"service_code": service.Code,
			"total_checks": totalChecks,
			"spam_count":   spamCount,
			"spam_rate":    spamRate,
		})
	}

	return stats, nil
}

// GetTopSpamKeywords gets most common spam keywords
func (s *StatisticsService) GetTopSpamKeywords(limit int) ([]map[string]interface{}, error) {
	// Get all spam results with keywords
	var spamResults []models.CheckResult
	if err := s.db.Where("is_spam = ? AND found_keywords IS NOT NULL", true).Find(&spamResults).Error; err != nil {
		return nil, fmt.Errorf("failed to get spam results: %w", err)
	}

	// Count keyword occurrences
	keywordCount := make(map[string]int)
	for _, result := range spamResults {
		// Convert StringArray to []string
		keywords := []string(result.FoundKeywords)
		for _, keyword := range keywords {
			if keyword != "" {
				keywordCount[keyword]++
			}
		}
	}

	// Sort keywords by count
	type kv struct {
		Keyword string
		Count   int
	}

	var sorted []kv
	for k, v := range keywordCount {
		sorted = append(sorted, kv{k, v})
	}

	// Manual sort
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Count > sorted[i].Count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Take top N keywords
	keywords := make([]map[string]interface{}, 0)
	for i := 0; i < len(sorted) && i < limit; i++ {
		keywords = append(keywords, map[string]interface{}{
			"keyword": sorted[i].Keyword,
			"count":   sorted[i].Count,
		})
	}

	return keywords, nil
}

// GetPhoneSpamHistory gets spam detection history for specific phone
func (s *StatisticsService) GetPhoneSpamHistory(phoneID uint) ([]map[string]interface{}, error) {
	var results []models.CheckResult

	err := s.db.
		Where("phone_number_id = ?", phoneID).
		Order("checked_at DESC").
		Limit(100).
		Preload("Service").
		Find(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get phone history: %w", err)
	}

	history := make([]map[string]interface{}, len(results))
	for i, result := range results {
		// Convert StringArray to []string for JSON serialization
		keywords := []string(result.FoundKeywords)

		history[i] = map[string]interface{}{
			"checked_at":     result.CheckedAt,
			"service_name":   result.Service.Name,
			"is_spam":        result.IsSpam,
			"found_keywords": keywords,
		}
	}

	return history, nil
}

// GetSpamTrends gets spam trends over time
func (s *StatisticsService) GetSpamTrends(interval string) ([]map[string]interface{}, error) {
	// Calculate date range based on interval
	endDate := time.Now()
	var startDate time.Time
	var groupByFormat string

	switch interval {
	case "hourly":
		startDate = endDate.Add(-24 * time.Hour)
		groupByFormat = "2006-01-02 15:00"
	case "daily":
		startDate = endDate.AddDate(0, 0, -30)
		groupByFormat = "2006-01-02"
	case "weekly":
		startDate = endDate.AddDate(0, 0, -90)
		groupByFormat = "2006-01-02" // Will group by week manually
	case "monthly":
		startDate = endDate.AddDate(-1, 0, 0)
		groupByFormat = "2006-01"
	default:
		startDate = endDate.AddDate(0, 0, -30)
		groupByFormat = "2006-01-02"
	}

	// Get all check results in date range
	var results []models.CheckResult
	if err := s.db.Where("checked_at >= ? AND checked_at <= ?", startDate, endDate).Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get check results: %w", err)
	}

	// Group by period
	periodStats := make(map[string]map[string]int)

	for _, result := range results {
		var periodKey string

		if interval == "weekly" {
			// Get start of week (Monday)
			year, week := result.CheckedAt.ISOWeek()
			periodKey = fmt.Sprintf("%d-W%02d", year, week)
		} else {
			periodKey = result.CheckedAt.Format(groupByFormat)
		}

		if periodStats[periodKey] == nil {
			periodStats[periodKey] = map[string]int{
				"total_checks": 0,
				"spam_count":   0,
			}
		}

		periodStats[periodKey]["total_checks"]++
		if result.IsSpam {
			periodStats[periodKey]["spam_count"]++
		}
	}

	// Convert to sorted array
	trends := make([]map[string]interface{}, 0)

	for period, data := range periodStats {
		spamRate := float64(0)
		if data["total_checks"] > 0 {
			spamRate = float64(data["spam_count"]) / float64(data["total_checks"]) * 100
		}

		trends = append(trends, map[string]interface{}{
			"period":       period,
			"total_checks": data["total_checks"],
			"spam_count":   data["spam_count"],
			"spam_rate":    spamRate,
		})
	}

	// Sort by period
	for i := 0; i < len(trends); i++ {
		for j := i + 1; j < len(trends); j++ {
			if trends[i]["period"].(string) > trends[j]["period"].(string) {
				trends[i], trends[j] = trends[j], trends[i]
			}
		}
	}

	return trends, nil
}

// GetRecentSpamDetections gets recent spam detections
func (s *StatisticsService) GetRecentSpamDetections(limit int) ([]map[string]interface{}, error) {
	var results []models.CheckResult

	err := s.db.
		Where("is_spam = ?", true).
		Order("checked_at DESC").
		Limit(limit).
		Preload("Service").
		Preload("PhoneNumber").
		Find(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get recent spam detections: %w", err)
	}

	detections := make([]map[string]interface{}, 0)
	for _, result := range results {
		// Convert StringArray to []string for JSON serialization
		keywords := []string(result.FoundKeywords)

		detection := map[string]interface{}{
			"phone_number":   result.PhoneNumber.Number,
			"description":    result.PhoneNumber.Description,
			"checked_at":     result.CheckedAt,
			"service_name":   result.Service.Name,
			"found_keywords": keywords,
		}
		detections = append(detections, detection)
	}

	return detections, nil
}

// GetPhoneStatistics gets detailed statistics for a specific phone
func (s *StatisticsService) GetPhoneStatistics(phoneID uint) (*models.Statistics, error) {
	var stats models.Statistics

	err := s.db.
		Where("phone_number_id = ?", phoneID).
		Preload("Service").
		First(&stats).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get phone statistics: %w", err)
	}

	return &stats, nil
}

// GetDashboardStats gets statistics specifically for dashboard
func (s *StatisticsService) GetDashboardStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get phone statistics
	phoneStats, err := NewPhoneService(s.db).GetPhoneStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get phone stats: %w", err)
	}

	for k, v := range phoneStats {
		stats[k] = v
	}

	// Get check statistics for today
	today := time.Now().Truncate(24 * time.Hour)

	var todayChecks int64
	if err := s.db.Model(&models.CheckResult{}).Where("checked_at >= ?", today).Count(&todayChecks).Error; err != nil {
		return nil, fmt.Errorf("failed to count today's checks: %w", err)
	}
	stats["today_checks"] = todayChecks

	var todaySpam int64
	if err := s.db.Model(&models.CheckResult{}).Where("checked_at >= ? AND is_spam = ?", today, true).Count(&todaySpam).Error; err != nil {
		return nil, fmt.Errorf("failed to count today's spam: %w", err)
	}
	stats["today_spam"] = todaySpam

	return stats, nil
}
