package handlers

import (
	"spam-checker/internal/middleware"
	"spam-checker/internal/models"
	"spam-checker/internal/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// GetCleanNumberRequest represents request for getting clean number
type GetCleanNumberRequest struct {
	Purpose  string                       `json:"purpose,omitempty"`
	Metadata *services.AllocationMetadata `json:"metadata,omitempty"`
}

// GetAllocationHistoryResponse represents allocation history response
type GetAllocationHistoryResponse struct {
	Allocations []AllocationInfo `json:"allocations"`
	Total       int              `json:"total"`
}

// AllocationInfo represents allocation information
type AllocationInfo struct {
	ID          uint   `json:"id"`
	PhoneNumber string `json:"phone_number"`
	PhoneID     uint   `json:"phone_id"`
	AllocatedTo string `json:"allocated_to"`
	Purpose     string `json:"purpose"`
	AllocatedAt string `json:"allocated_at"`
	Metadata    string `json:"metadata,omitempty"`
}

// RegisterAsteriskRoutes registers Asterisk integration routes
func RegisterAsteriskRoutes(api fiber.Router, asteriskService *services.AsteriskService, authMiddleware *middleware.AuthMiddleware) {
	asterisk := api.Group("/asterisk")

	// Public endpoint for getting clean number (can be protected if needed)
	asterisk.Post("/get-clean-number", getCleanNumberHandler(asteriskService))

	// Protected endpoints for monitoring and stats
	protected := asterisk.Use(authMiddleware.Protect())
	protected.Get("/allocation-history/:phone_id", authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor), getAllocationHistoryHandler(asteriskService))
	protected.Get("/allocation-stats", authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor), getAllocationStatsHandler(asteriskService))
	protected.Get("/current-allocations", authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor), getCurrentAllocationsHandler(asteriskService))
	protected.Post("/cleanup-allocations", authMiddleware.RequireRole(models.RoleAdmin), cleanupAllocationsHandler(asteriskService))
}

// getCleanNumberHandler godoc
// @Summary Get clean phone number
// @Description Get a clean (non-spam) phone number for use in Asterisk
// @Tags asterisk
// @Accept json
// @Produce json
// @Param request body GetCleanNumberRequest false "Optional allocation details"
// @Success 200 {object} services.CleanNumberResponse
// @Failure 404 {object} map[string]interface{} "No clean numbers available"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /asterisk/get-clean-number [post]
func getCleanNumberHandler(asteriskService *services.AsteriskService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get client IP
		clientIP := c.IP()

		// Parse optional request body
		var req GetCleanNumberRequest
		c.BodyParser(&req)

		// Default purpose if not provided
		purpose := req.Purpose
		if purpose == "" {
			purpose = "asterisk_call"
		}

		// Add user agent to metadata
		if req.Metadata == nil {
			req.Metadata = &services.AllocationMetadata{}
		}
		req.Metadata.UserAgent = string(c.Request().Header.UserAgent())

		// Get clean number
		response, err := asteriskService.GetCleanNumber(clientIP, purpose, req.Metadata)
		if err != nil {
			statusCode := fiber.StatusInternalServerError
			errorMsg := "Failed to allocate clean number"

			if err.Error() == "no clean numbers available" {
				statusCode = fiber.StatusNotFound
				errorMsg = "No clean numbers available"
			}

			return c.Status(statusCode).JSON(fiber.Map{
				"error":   errorMsg,
				"details": err.Error(),
			})
		}

		return c.JSON(response)
	}
}

// getAllocationHistoryHandler godoc
// @Summary Get allocation history
// @Description Get allocation history for a specific phone number
// @Tags asterisk
// @Accept json
// @Produce json
// @Param phone_id path int true "Phone ID"
// @Param limit query int false "Limit results" default(100)
// @Success 200 {object} GetAllocationHistoryResponse
// @Security BearerAuth
// @Router /asterisk/allocation-history/{phone_id} [get]
func getAllocationHistoryHandler(asteriskService *services.AsteriskService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		phoneID, err := strconv.ParseUint(c.Params("phone_id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid phone ID",
			})
		}

		limit, _ := strconv.Atoi(c.Query("limit", "100"))
		if limit <= 0 || limit > 1000 {
			limit = 100
		}

		allocations, err := asteriskService.GetAllocationHistory(uint(phoneID), limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get allocation history",
			})
		}

		// Format response
		allocationInfo := make([]AllocationInfo, len(allocations))
		for i, alloc := range allocations {
			allocationInfo[i] = AllocationInfo{
				ID:          alloc.ID,
				PhoneNumber: alloc.PhoneNumber.Number,
				PhoneID:     alloc.PhoneNumberID,
				AllocatedTo: alloc.AllocatedTo,
				Purpose:     alloc.Purpose,
				AllocatedAt: alloc.AllocatedAt.Format("2006-01-02 15:04:05"),
				Metadata:    alloc.Metadata,
			}
		}

		return c.JSON(GetAllocationHistoryResponse{
			Allocations: allocationInfo,
			Total:       len(allocationInfo),
		})
	}
}

// getAllocationStatsHandler godoc
// @Summary Get allocation statistics
// @Description Get allocation statistics for the specified period
// @Tags asterisk
// @Accept json
// @Produce json
// @Param days query int false "Number of days" default(7)
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /asterisk/allocation-stats [get]
func getAllocationStatsHandler(asteriskService *services.AsteriskService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		days, _ := strconv.Atoi(c.Query("days", "7"))
		if days <= 0 || days > 365 {
			days = 7
		}

		stats, err := asteriskService.GetAllocationStats(days)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get allocation statistics",
			})
		}

		return c.JSON(stats)
	}
}

// getCurrentAllocationsHandler godoc
// @Summary Get current allocations
// @Description Get recent allocations within specified minutes
// @Tags asterisk
// @Accept json
// @Produce json
// @Param minutes query int false "Minutes to look back" default(60)
// @Success 200 {array} AllocationInfo
// @Security BearerAuth
// @Router /asterisk/current-allocations [get]
func getCurrentAllocationsHandler(asteriskService *services.AsteriskService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		minutes, _ := strconv.Atoi(c.Query("minutes", "60"))
		if minutes <= 0 || minutes > 1440 { // Max 24 hours
			minutes = 60
		}

		allocations, err := asteriskService.GetCurrentAllocations(minutes)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get current allocations",
			})
		}

		// Format response
		allocationInfo := make([]AllocationInfo, len(allocations))
		for i, alloc := range allocations {
			allocationInfo[i] = AllocationInfo{
				ID:          alloc.ID,
				PhoneNumber: alloc.PhoneNumber.Number,
				PhoneID:     alloc.PhoneNumberID,
				AllocatedTo: alloc.AllocatedTo,
				Purpose:     alloc.Purpose,
				AllocatedAt: alloc.AllocatedAt.Format("2006-01-02 15:04:05"),
				Metadata:    alloc.Metadata,
			}
		}

		return c.JSON(allocationInfo)
	}
}

// cleanupAllocationsHandler godoc
// @Summary Cleanup old allocations
// @Description Remove allocation records older than specified days
// @Tags asterisk
// @Accept json
// @Produce json
// @Param days query int false "Days to keep" default(90)
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /asterisk/cleanup-allocations [post]
func cleanupAllocationsHandler(asteriskService *services.AsteriskService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		days, _ := strconv.Atoi(c.Query("days", "90"))
		if days < 30 {
			days = 30 // Minimum 30 days retention
		}

		deleted, err := asteriskService.CleanupOldAllocations(days)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to cleanup allocations",
			})
		}

		return c.JSON(fiber.Map{
			"message": "Cleanup completed successfully",
			"deleted": deleted,
			"days":    days,
		})
	}
}
