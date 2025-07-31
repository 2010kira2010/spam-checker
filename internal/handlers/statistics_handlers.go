package handlers

import (
	"spam-checker/internal/middleware"
	"spam-checker/internal/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// RegisterStatisticsRoutes registers statistics routes
func RegisterStatisticsRoutes(api fiber.Router, statisticsService *services.StatisticsService, authMiddleware *middleware.AuthMiddleware) {
	stats := api.Group("/statistics")

	// @Summary Get overview statistics
	// @Description Get general overview statistics
	// @Tags statistics
	// @Accept json
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /statistics/overview [get]
	stats.Get("/overview", func(c *fiber.Ctx) error {
		stats, err := statisticsService.GetOverviewStats()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get overview statistics",
			})
		}

		return c.JSON(stats)
	})

	// @Summary Get time series statistics
	// @Description Get statistics for time series charts
	// @Tags statistics
	// @Accept json
	// @Produce json
	// @Param days query int false "Number of days" default(7)
	// @Success 200 {array} map[string]interface{}
	// @Security BearerAuth
	// @Router /statistics/timeseries [get]
	stats.Get("/timeseries", func(c *fiber.Ctx) error {
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

		return c.JSON(stats)
	})

	// @Summary Get service statistics
	// @Description Get statistics by service
	// @Tags statistics
	// @Accept json
	// @Produce json
	// @Success 200 {array} map[string]interface{}
	// @Security BearerAuth
	// @Router /statistics/services [get]
	stats.Get("/services", func(c *fiber.Ctx) error {
		stats, err := statisticsService.GetServiceStats()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get service statistics",
			})
		}

		return c.JSON(stats)
	})

	// @Summary Get top spam keywords
	// @Description Get most common spam keywords
	// @Tags statistics
	// @Accept json
	// @Produce json
	// @Param limit query int false "Limit results" default(10)
	// @Success 200 {array} map[string]interface{}
	// @Security BearerAuth
	// @Router /statistics/keywords [get]
	stats.Get("/keywords", func(c *fiber.Ctx) error {
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

		return c.JSON(keywords)
	})

	// @Summary Get phone spam history
	// @Description Get spam detection history for specific phone
	// @Tags statistics
	// @Accept json
	// @Produce json
	// @Param phone_id query int true "Phone ID"
	// @Success 200 {array} map[string]interface{}
	// @Security BearerAuth
	// @Router /statistics/phone-history [get]
	stats.Get("/phone-history", func(c *fiber.Ctx) error {
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

		return c.JSON(history)
	})

	// @Summary Get spam trends
	// @Description Get spam trends over time
	// @Tags statistics
	// @Accept json
	// @Produce json
	// @Param interval query string false "Interval (hourly, daily, weekly, monthly)" default(daily)
	// @Success 200 {array} map[string]interface{}
	// @Security BearerAuth
	// @Router /statistics/trends [get]
	stats.Get("/trends", func(c *fiber.Ctx) error {
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

		return c.JSON(trends)
	})

	// @Summary Get recent spam detections
	// @Description Get recent spam detections
	// @Tags statistics
	// @Accept json
	// @Produce json
	// @Param limit query int false "Limit results" default(10)
	// @Success 200 {array} map[string]interface{}
	// @Security BearerAuth
	// @Router /statistics/recent-spam [get]
	stats.Get("/recent-spam", func(c *fiber.Ctx) error {
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

		return c.JSON(detections)
	})

	// @Summary Export statistics report
	// @Description Export statistics report as CSV
	// @Tags statistics
	// @Produce text/csv
	// @Param start_date query string false "Start date (YYYY-MM-DD)"
	// @Param end_date query string false "End date (YYYY-MM-DD)"
	// @Success 200 {file} file
	// @Security BearerAuth
	// @Router /statistics/export [get]
	stats.Get("/export", func(c *fiber.Ctx) error {
		// TODO: Implement CSV export
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": "Export feature not implemented yet",
		})
	})
}
