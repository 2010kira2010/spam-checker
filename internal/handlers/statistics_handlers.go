package handlers

import (
	"spam-checker/internal/middleware"
	"spam-checker/internal/services"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RegisterStatisticsRoutes registers statistics routes
func RegisterStatisticsRoutes(api fiber.Router, statisticsService *services.StatisticsService, authMiddleware *middleware.AuthMiddleware) {
	stats := api.Group("/statistics")

	stats.Get("/overview", getOverviewStatsHandler(statisticsService))
	stats.Get("/dashboard", getDashboardStatsHandler(statisticsService))
	stats.Get("/timeseries", getTimeSeriesStatsHandler(statisticsService))
	stats.Get("/services", getServiceStatsHandler(statisticsService))
	stats.Get("/keywords", getTopSpamKeywordsHandler(statisticsService))
	stats.Get("/phone-history", getPhoneSpamHistoryHandler(statisticsService))
	stats.Get("/trends", getSpamTrendsHandler(statisticsService))
	stats.Get("/recent-spam", getRecentSpamDetectionsHandler(statisticsService))
	stats.Get("/export", exportStatisticsHandler(statisticsService))
}

// getOverviewStatsHandler godoc
// @Summary Get overview statistics
// @Description Get general overview statistics
// @Tags statistics
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /statistics/overview [get]
func getOverviewStatsHandler(statisticsService *services.StatisticsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		stats, err := statisticsService.GetOverviewStats()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get overview statistics",
			})
		}

		return c.JSON(stats)
	}
}

// getDashboardStatsHandler godoc
// @Summary Get dashboard statistics
// @Description Get statistics specifically for dashboard
// @Tags statistics
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /statistics/dashboard [get]
func getDashboardStatsHandler(statisticsService *services.StatisticsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		stats, err := statisticsService.GetDashboardStats()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get dashboard statistics",
			})
		}

		return c.JSON(stats)
	}
}

// getTimeSeriesStatsHandler godoc
// @Summary Get time series statistics
// @Description Get statistics for time series charts
// @Tags statistics
// @Accept json
// @Produce json
// @Param days query int false "Number of days" default(7)
// @Success 200 {array} map[string]interface{}
// @Security BearerAuth
// @Router /statistics/timeseries [get]
func getTimeSeriesStatsHandler(statisticsService *services.StatisticsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		days, _ := strconv.Atoi(c.Query("days", "7"))
		if days < 1 || days > 365 {
			days = 7
		}

		stats, err := statisticsService.GetTimeSeriesStats(days)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get time series statistics",
			})
		}

		// Ensure we return an array, even if empty
		if stats == nil {
			stats = []map[string]interface{}{}
		}

		return c.JSON(stats)
	}
}

// getServiceStatsHandler godoc
// @Summary Get service statistics
// @Description Get statistics by service
// @Tags statistics
// @Accept json
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Security BearerAuth
// @Router /statistics/services [get]
func getServiceStatsHandler(statisticsService *services.StatisticsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		stats, err := statisticsService.GetServiceStats()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get service statistics",
			})
		}

		// Ensure we return an array, even if empty
		if stats == nil {
			stats = []map[string]interface{}{}
		}

		return c.JSON(stats)
	}
}

// getTopSpamKeywordsHandler godoc
// @Summary Get top spam keywords
// @Description Get most common spam keywords
// @Tags statistics
// @Accept json
// @Produce json
// @Param limit query int false "Limit results" default(10)
// @Success 200 {array} map[string]interface{}
// @Security BearerAuth
// @Router /statistics/keywords [get]
func getTopSpamKeywordsHandler(statisticsService *services.StatisticsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		limit, _ := strconv.Atoi(c.Query("limit", "10"))
		if limit < 1 || limit > 100 {
			limit = 10
		}

		keywords, err := statisticsService.GetTopSpamKeywords(limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get top keywords",
			})
		}

		// Ensure we return an array, even if empty
		if keywords == nil {
			keywords = []map[string]interface{}{}
		}

		return c.JSON(keywords)
	}
}

// getPhoneSpamHistoryHandler godoc
// @Summary Get phone spam history
// @Description Get spam detection history for specific phone
// @Tags statistics
// @Accept json
// @Produce json
// @Param phone_id query int true "Phone ID"
// @Success 200 {array} map[string]interface{}
// @Security BearerAuth
// @Router /statistics/phone-history [get]
func getPhoneSpamHistoryHandler(statisticsService *services.StatisticsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		phoneID, err := strconv.ParseUint(c.Query("phone_id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid phone ID",
			})
		}

		history, err := statisticsService.GetPhoneSpamHistory(uint(phoneID))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get phone history",
			})
		}

		// Ensure we return an array, even if empty
		if history == nil {
			history = []map[string]interface{}{}
		}

		return c.JSON(history)
	}
}

// getSpamTrendsHandler godoc
// @Summary Get spam trends
// @Description Get spam trends over time
// @Tags statistics
// @Accept json
// @Produce json
// @Param interval query string false "Interval (hourly, daily, weekly, monthly)" default(daily)
// @Success 200 {array} map[string]interface{}
// @Security BearerAuth
// @Router /statistics/trends [get]
func getSpamTrendsHandler(statisticsService *services.StatisticsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		interval := c.Query("interval", "daily")
		validIntervals := map[string]bool{
			"hourly":  true,
			"daily":   true,
			"weekly":  true,
			"monthly": true,
		}

		if !validIntervals[interval] {
			interval = "daily"
		}

		trends, err := statisticsService.GetSpamTrends(interval)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get spam trends",
			})
		}

		// Ensure we return an array, even if empty
		if trends == nil {
			trends = []map[string]interface{}{}
		}

		return c.JSON(trends)
	}
}

// getRecentSpamDetectionsHandler godoc
// @Summary Get recent spam detections
// @Description Get recent spam detections
// @Tags statistics
// @Accept json
// @Produce json
// @Param limit query int false "Limit results" default(10)
// @Success 200 {array} map[string]interface{}
// @Security BearerAuth
// @Router /statistics/recent-spam [get]
func getRecentSpamDetectionsHandler(statisticsService *services.StatisticsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		limit, _ := strconv.Atoi(c.Query("limit", "10"))
		if limit < 1 || limit > 100 {
			limit = 10
		}

		detections, err := statisticsService.GetRecentSpamDetections(limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get recent spam detections",
			})
		}

		// Ensure we return an array, even if empty
		if detections == nil {
			detections = []map[string]interface{}{}
		}

		return c.JSON(detections)
	}
}

// exportStatisticsHandler godoc
// @Summary Export statistics report
// @Description Export statistics report as CSV
// @Tags statistics
// @Produce text/csv
// @Param start_date query string false "Start date (YYYY-MM-DD)"
// @Param end_date query string false "End date (YYYY-MM-DD)"
// @Success 200 {file} file
// @Security BearerAuth
// @Router /statistics/export [get]
func exportStatisticsHandler(statisticsService *services.StatisticsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		startDateStr := c.Query("start_date", "")
		endDateStr := c.Query("end_date", "")

		var startDate, endDate time.Time
		var err error

		// Parse dates if provided
		if startDateStr != "" {
			startDate, err = time.Parse("2006-01-02", startDateStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Invalid start date format",
				})
			}
		} else {
			// Default to 30 days ago
			startDate = time.Now().AddDate(0, 0, -30)
		}

		if endDateStr != "" {
			endDate, err = time.Parse("2006-01-02", endDateStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Invalid end date format",
				})
			}
		} else {
			// Default to today
			endDate = time.Now()
		}

		// For now, return not implemented
		// TODO: Implement CSV export with date range
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error":      "Export feature not implemented yet",
			"start_date": startDate.Format("2006-01-02"),
			"end_date":   endDate.Format("2006-01-02"),
		})
	}
}
