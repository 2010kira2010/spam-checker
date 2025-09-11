package services

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"spam-checker/internal/logger"
	"spam-checker/internal/models"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type AsteriskService struct {
	db              *gorm.DB
	log             *logrus.Entry
	allocationMutex sync.Mutex
	rng             *rand.Rand
}

// AllocationMetadata stores additional information about allocation
type AllocationMetadata struct {
	UserAgent string `json:"user_agent,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Source    string `json:"source,omitempty"`
}

// CleanNumberResponse represents the response for clean number request
type CleanNumberResponse struct {
	Number       string    `json:"number"`
	PhoneID      uint      `json:"phone_id"`
	Description  string    `json:"description,omitempty"`
	AllocatedAt  time.Time `json:"allocated_at"`
	AllocationID uint      `json:"allocation_id"`
}

func NewAsteriskService(db *gorm.DB) *AsteriskService {
	return &AsteriskService{
		db:  db,
		log: logger.WithField("service", "AsteriskService"),
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GetCleanNumber returns a clean (non-spam) phone number with load balancing
func (s *AsteriskService) GetCleanNumber(clientIP string, purpose string, metadata *AllocationMetadata) (*CleanNumberResponse, error) {
	s.allocationMutex.Lock()
	defer s.allocationMutex.Unlock()

	log := s.log.WithFields(logrus.Fields{
		"method":   "GetCleanNumber",
		"clientIP": clientIP,
		"purpose":  purpose,
	})

	// Get all active clean numbers with their usage stats
	cleanNumbers, err := s.getCleanNumbersWithStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get clean numbers: %w", err)
	}

	if len(cleanNumbers) == 0 {
		return nil, fmt.Errorf("no clean numbers available")
	}

	// Select number using weighted random selection based on usage
	selectedNumber := s.selectNumberWithLoadBalancing(cleanNumbers)
	if selectedNumber == nil {
		return nil, fmt.Errorf("failed to select number")
	}

	// Record allocation
	allocation := &models.NumberAllocation{
		PhoneNumberID: selectedNumber.PhoneNumberID,
		AllocatedTo:   clientIP,
		Purpose:       purpose,
		AllocatedAt:   time.Now(),
	}

	// Add metadata if provided
	if metadata != nil {
		metadataJSON, _ := json.Marshal(metadata)
		allocation.Metadata = string(metadataJSON)
	}

	if err := s.db.Create(allocation).Error; err != nil {
		return nil, fmt.Errorf("failed to record allocation: %w", err)
	}

	// Get full phone details
	var phone models.PhoneNumber
	if err := s.db.First(&phone, selectedNumber.PhoneNumberID).Error; err != nil {
		return nil, fmt.Errorf("failed to get phone details: %w", err)
	}

	log.Infof("Allocated number %s (ID: %d) to %s", phone.Number, phone.ID, clientIP)

	return &CleanNumberResponse{
		Number:       phone.Number,
		PhoneID:      phone.ID,
		Description:  phone.Description,
		AllocatedAt:  allocation.AllocatedAt,
		AllocationID: allocation.ID,
	}, nil
}

// getCleanNumbersWithStats gets all clean active numbers with usage statistics
func (s *AsteriskService) getCleanNumbersWithStats() ([]models.PhoneNumberUsageStats, error) {
	// SQL query to get clean numbers with usage stats
	query := `
		WITH latest_checks AS (
			SELECT DISTINCT ON (phone_number_id, service_id)
				phone_number_id,
				service_id,
				is_spam,
				checked_at
			FROM check_results
			ORDER BY phone_number_id, service_id, checked_at DESC
		),
		spam_status AS (
			SELECT 
				phone_number_id,
				BOOL_OR(is_spam) as has_spam
			FROM latest_checks
			GROUP BY phone_number_id
		),
		daily_allocations AS (
			SELECT 
				phone_number_id,
				COUNT(*) as count
			FROM number_allocations
			WHERE allocated_at >= CURRENT_DATE
			GROUP BY phone_number_id
		),
		total_allocations AS (
			SELECT 
				phone_number_id,
				COUNT(*) as count,
				MAX(allocated_at) as last_allocated
			FROM number_allocations
			GROUP BY phone_number_id
		)
		SELECT 
			pn.id as phone_number_id,
			pn.number,
			COALESCE(ta.count, 0) as total_allocations,
			ta.last_allocated as last_allocated_at,
			COALESCE(da.count, 0) as daily_allocations,
			COALESCE(NOT ss.has_spam, true) as is_clean
		FROM phone_numbers pn
		LEFT JOIN spam_status ss ON ss.phone_number_id = pn.id
		LEFT JOIN total_allocations ta ON ta.phone_number_id = pn.id
		LEFT JOIN daily_allocations da ON da.phone_number_id = pn.id
		WHERE pn.is_active = true
			AND pn.deleted_at IS NULL
			AND (ss.has_spam IS NULL OR ss.has_spam = false)
		ORDER BY pn.id
	`

	var stats []models.PhoneNumberUsageStats
	if err := s.db.Raw(query).Scan(&stats).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// selectNumberWithLoadBalancing selects a number using weighted random selection
func (s *AsteriskService) selectNumberWithLoadBalancing(numbers []models.PhoneNumberUsageStats) *models.PhoneNumberUsageStats {
	if len(numbers) == 0 {
		return nil
	}

	// If only one number, return it
	if len(numbers) == 1 {
		return &numbers[0]
	}

	// Calculate weights for each number
	weights := make([]float64, len(numbers))
	totalWeight := 0.0

	// Find the maximum allocations to normalize weights
	maxAllocations := int64(0)
	for _, num := range numbers {
		if num.TotalAllocations > maxAllocations {
			maxAllocations = num.TotalAllocations
		}
	}

	// Calculate weights (inverse of allocation count for load balancing)
	for i, num := range numbers {
		// Base weight - higher for numbers with fewer allocations
		weight := 1.0
		if maxAllocations > 0 {
			// Normalize allocation count and invert (fewer allocations = higher weight)
			normalizedAlloc := float64(num.TotalAllocations) / float64(maxAllocations+1)
			weight = 1.0 - normalizedAlloc + 0.1 // Add 0.1 to ensure non-zero weight
		}

		// Boost weight for numbers not used today
		if num.DailyAllocations == 0 {
			weight *= 2.0
		}

		// Boost weight for numbers not used recently
		if num.LastAllocatedAt == nil {
			weight *= 3.0
		} else {
			hoursSinceLastUse := time.Since(*num.LastAllocatedAt).Hours()
			if hoursSinceLastUse > 24 {
				weight *= 2.0
			} else if hoursSinceLastUse > 1 {
				weight *= 1.5
			}
		}

		weights[i] = weight
		totalWeight += weight
	}

	// Weighted random selection
	if totalWeight <= 0 {
		// Fallback to random selection if all weights are zero
		return &numbers[s.rng.Intn(len(numbers))]
	}

	randomValue := s.rng.Float64() * totalWeight
	currentWeight := 0.0

	for i, weight := range weights {
		currentWeight += weight
		if randomValue <= currentWeight {
			return &numbers[i]
		}
	}

	// Fallback (should not reach here)
	return &numbers[len(numbers)-1]
}

// GetAllocationHistory gets allocation history for a specific phone number
func (s *AsteriskService) GetAllocationHistory(phoneID uint, limit int) ([]models.NumberAllocation, error) {
	var allocations []models.NumberAllocation

	query := s.db.Where("phone_number_id = ?", phoneID).
		Order("allocated_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Preload("PhoneNumber").Find(&allocations).Error; err != nil {
		return nil, fmt.Errorf("failed to get allocation history: %w", err)
	}

	return allocations, nil
}

// GetAllocationStats gets allocation statistics
func (s *AsteriskService) GetAllocationStats(days int) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total allocations
	var totalAllocations int64
	if err := s.db.Model(&models.NumberAllocation{}).Count(&totalAllocations).Error; err != nil {
		return nil, fmt.Errorf("failed to count total allocations: %w", err)
	}
	stats["total_allocations"] = totalAllocations

	// Allocations in time range
	startDate := time.Now().AddDate(0, 0, -days)
	var periodAllocations int64
	if err := s.db.Model(&models.NumberAllocation{}).
		Where("allocated_at >= ?", startDate).
		Count(&periodAllocations).Error; err != nil {
		return nil, fmt.Errorf("failed to count period allocations: %w", err)
	}
	stats["period_allocations"] = periodAllocations
	stats["period_days"] = days

	// Daily average
	if days > 0 {
		stats["daily_average"] = float64(periodAllocations) / float64(days)
	}

	// Most used numbers
	type NumberUsage struct {
		PhoneNumberID uint   `json:"phone_number_id"`
		Number        string `json:"number"`
		Count         int64  `json:"count"`
	}

	var mostUsed []NumberUsage
	err := s.db.Table("number_allocations").
		Select("number_allocations.phone_number_id, phone_numbers.number, COUNT(*) as count").
		Joins("JOIN phone_numbers ON phone_numbers.id = number_allocations.phone_number_id").
		Where("number_allocations.allocated_at >= ?", startDate).
		Group("number_allocations.phone_number_id, phone_numbers.number").
		Order("count DESC").
		Limit(10).
		Scan(&mostUsed).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get most used numbers: %w", err)
	}
	stats["most_used_numbers"] = mostUsed

	// Allocations by purpose
	type PurposeCount struct {
		Purpose string `json:"purpose"`
		Count   int64  `json:"count"`
	}

	var purposeCounts []PurposeCount
	err = s.db.Table("number_allocations").
		Select("purpose, COUNT(*) as count").
		Where("allocated_at >= ?", startDate).
		Group("purpose").
		Order("count DESC").
		Scan(&purposeCounts).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get purpose counts: %w", err)
	}
	stats["allocations_by_purpose"] = purposeCounts

	// Clean numbers available
	cleanNumbers, err := s.getCleanNumbersWithStats()
	if err == nil {
		stats["clean_numbers_available"] = len(cleanNumbers)
	}

	return stats, nil
}

// GetCurrentAllocations gets current allocations for monitoring
func (s *AsteriskService) GetCurrentAllocations(minutes int) ([]models.NumberAllocation, error) {
	since := time.Now().Add(-time.Duration(minutes) * time.Minute)

	var allocations []models.NumberAllocation
	err := s.db.Where("allocated_at >= ?", since).
		Order("allocated_at DESC").
		Preload("PhoneNumber").
		Find(&allocations).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get current allocations: %w", err)
	}

	return allocations, nil
}

// CleanupOldAllocations removes old allocation records (for maintenance)
func (s *AsteriskService) CleanupOldAllocations(daysBefore int) (int64, error) {
	cutoffDate := time.Now().AddDate(0, 0, -daysBefore)

	result := s.db.Where("allocated_at < ?", cutoffDate).Delete(&models.NumberAllocation{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup old allocations: %w", result.Error)
	}

	return result.RowsAffected, nil
}
