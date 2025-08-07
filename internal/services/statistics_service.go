package services

import (
	"fmt"
	"spam-checker/internal/models"
	"time"

	"gorm.io/gorm"
)

type StatisticsService struct {
	db *gorm.DB
}

func NewStatisticsService(db *gorm.DB) *StatisticsService {
	return &StatisticsService{db: db}
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
	if totalChecks > 0 {
		stats["spam_rate"] = float64(spamDetections) / float64(totalChecks) * 100
	} else {
		stats["spam_rate"] = 0
	}

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

	query := `
		SELECT 
			DATE(checked_at) as date,
			COUNT(*) as total_checks,
			SUM(CASE WHEN is_spam THEN 1 ELSE 0 END) as spam_count,
			SUM(CASE WHEN NOT is_spam THEN 1 ELSE 0 END) as clean_count
		FROM check_results
		WHERE checked_at >= ? AND checked_at <= ?
		GROUP BY DATE(checked_at)
		ORDER BY date ASC
	`

	var results []struct {
		Date        time.Time `gorm:"column:date"`
		TotalChecks int       `gorm:"column:total_checks"`
		SpamCount   int       `gorm:"column:spam_count"`
		CleanCount  int       `gorm:"column:clean_count"`
	}

	if err := s.db.Raw(query, startDate, endDate).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get time series stats: %w", err)
	}

	// Convert to map format
	stats := make([]map[string]interface{}, len(results))
	for i, result := range results {
		stats[i] = map[string]interface{}{
			"date":         result.Date.Format("2006-01-02"),
			"total_checks": result.TotalChecks,
			"spam_count":   result.SpamCount,
			"clean_count":  result.CleanCount,
			"spam_rate":    float64(result.SpamCount) / float64(result.TotalChecks) * 100,
		}
	}

	return stats, nil
}

// GetServiceStats gets statistics by service
func (s *StatisticsService) GetServiceStats() ([]map[string]interface{}, error) {
	query := `
		SELECT 
			ss.id,
			ss.name,
			ss.code,
			COUNT(cr.id) as total_checks,
			SUM(CASE WHEN cr.is_spam THEN 1 ELSE 0 END) as spam_count,
			AVG(CASE WHEN cr.is_spam THEN 1 ELSE 0 END) * 100 as spam_rate
		FROM spam_services ss
		LEFT JOIN check_results cr ON cr.service_id = ss.id
		GROUP BY ss.id, ss.name, ss.code
		ORDER BY total_checks DESC
	`

	var results []struct {
		ID          uint    `gorm:"column:id"`
		Name        string  `gorm:"column:name"`
		Code        string  `gorm:"column:code"`
		TotalChecks int     `gorm:"column:total_checks"`
		SpamCount   int     `gorm:"column:spam_count"`
		SpamRate    float64 `gorm:"column:spam_rate"`
	}

	if err := s.db.Raw(query).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get service stats: %w", err)
	}

	// Convert to map format
	stats := make([]map[string]interface{}, len(results))
	for i, result := range results {
		stats[i] = map[string]interface{}{
			"service_id":   result.ID,
			"service_name": result.Name,
			"service_code": result.Code,
			"total_checks": result.TotalChecks,
			"spam_count":   result.SpamCount,
			"spam_rate":    result.SpamRate,
		}
	}

	return stats, nil
}

// GetTopSpamKeywords gets most common spam keywords
func (s *StatisticsService) GetTopSpamKeywords(limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			keyword,
			COUNT(*) as occurrence_count
		FROM (
			SELECT unnest(found_keywords) as keyword
			FROM check_results
			WHERE is_spam = true AND array_length(found_keywords, 1) > 0
		) keywords
		GROUP BY keyword
		ORDER BY occurrence_count DESC
		LIMIT ?
	`

	var results []struct {
		Keyword         string `gorm:"column:keyword"`
		OccurrenceCount int    `gorm:"column:occurrence_count"`
	}

	if err := s.db.Raw(query, limit).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get top keywords: %w", err)
	}

	// Convert to map format
	keywords := make([]map[string]interface{}, len(results))
	for i, result := range results {
		keywords[i] = map[string]interface{}{
			"keyword": result.Keyword,
			"count":   result.OccurrenceCount,
		}
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
		history[i] = map[string]interface{}{
			"checked_at":     result.CheckedAt,
			"service_name":   result.Service.Name,
			"is_spam":        result.IsSpam,
			"found_keywords": result.FoundKeywords,
		}
	}

	return history, nil
}

// GetSpamTrends gets spam trends over time
func (s *StatisticsService) GetSpamTrends(interval string) ([]map[string]interface{}, error) {
	var groupBy string
	var dateFormat string

	switch interval {
	case "hourly":
		groupBy = "DATE_TRUNC('hour', checked_at)"
		dateFormat = "2006-01-02 15:00"
	case "daily":
		groupBy = "DATE(checked_at)"
		dateFormat = "2006-01-02"
	case "weekly":
		groupBy = "DATE_TRUNC('week', checked_at)"
		dateFormat = "2006-01-02"
	case "monthly":
		groupBy = "DATE_TRUNC('month', checked_at)"
		dateFormat = "2006-01"
	default:
		groupBy = "DATE(checked_at)"
		dateFormat = "2006-01-02"
	}

	query := fmt.Sprintf(`
		SELECT 
			%s as period,
			COUNT(*) as total_checks,
			SUM(CASE WHEN is_spam THEN 1 ELSE 0 END) as spam_count,
			AVG(CASE WHEN is_spam THEN 1 ELSE 0 END) * 100 as spam_rate
		FROM check_results
		WHERE checked_at >= NOW() - INTERVAL '30 days'
		GROUP BY period
		ORDER BY period ASC
	`, groupBy)

	var results []struct {
		Period      time.Time `gorm:"column:period"`
		TotalChecks int       `gorm:"column:total_checks"`
		SpamCount   int       `gorm:"column:spam_count"`
		SpamRate    float64   `gorm:"column:spam_rate"`
	}

	if err := s.db.Raw(query).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get spam trends: %w", err)
	}

	trends := make([]map[string]interface{}, len(results))
	for i, result := range results {
		trends[i] = map[string]interface{}{
			"period":       result.Period.Format(dateFormat),
			"total_checks": result.TotalChecks,
			"spam_count":   result.SpamCount,
			"spam_rate":    result.SpamRate,
		}
	}

	return trends, nil
}

// GetRecentSpamDetections gets recent spam detections
func (s *StatisticsService) GetRecentSpamDetections(limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			pn.number,
			pn.description,
			cr.checked_at,
			ss.name as service_name,
			cr.found_keywords
		FROM check_results cr
		JOIN phone_numbers pn ON pn.id = cr.phone_number_id
		JOIN spam_services ss ON ss.id = cr.service_id
		WHERE cr.is_spam = true
		ORDER BY cr.checked_at DESC
		LIMIT ?
	`

	var results []struct {
		Number        string             `gorm:"column:number"`
		Description   string             `gorm:"column:description"`
		CheckedAt     time.Time          `gorm:"column:checked_at"`
		ServiceName   string             `gorm:"column:service_name"`
		FoundKeywords models.StringArray `gorm:"column:found_keywords;type:text[]"`
	}

	if err := s.db.Raw(query, limit).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get recent spam detections: %w", err)
	}

	detections := make([]map[string]interface{}, len(results))
	for i, result := range results {
		detections[i] = map[string]interface{}{
			"phone_number":   result.Number,
			"description":    result.Description,
			"checked_at":     result.CheckedAt,
			"service_name":   result.ServiceName,
			"found_keywords": []string(result.FoundKeywords), // Convert back to []string for JSON
		}
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
