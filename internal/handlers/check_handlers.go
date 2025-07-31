package handlers

import (
	"spam-checker/internal/middleware"
	"spam-checker/internal/models"
	"spam-checker/internal/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type CheckPhoneRequest struct {
	PhoneNumber string `json:"phone_number" validate:"required"`
}

type CheckAllRequest struct {
	Force bool `json:"force"`
}

// RegisterCheckRoutes registers check routes
func RegisterCheckRoutes(api fiber.Router, checkService *services.CheckService, authMiddleware *middleware.AuthMiddleware) {
	checks := api.Group("/checks")

	// @Summary Check phone
	// @Description Check a specific phone number
	// @Tags checks
	// @Accept json
	// @Produce json
	// @Param id path int true "Phone ID"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /checks/phone/{id} [post]
	checks.Post("/phone/:id", authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor), func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid phone ID",
			})
		}

		// Start check in background
		go checkService.CheckPhoneNumber(uint(id))

		return c.JSON(fiber.Map{
			"message":  "Check started",
			"phone_id": id,
		})
	})

	// @Summary Check all phones
	// @Description Check all active phone numbers
	// @Tags checks
	// @Accept json
	// @Produce json
	// @Param request body CheckAllRequest false "Check options"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /checks/all [post]
	checks.Post("/all", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		var req CheckAllRequest
		c.BodyParser(&req)

		// Start check in background
		go checkService.CheckAllPhones()

		return c.JSON(fiber.Map{
			"message": "Check started for all active phones",
		})
	})

	// @Summary Check realtime
	// @Description Check phone number in real-time (without saving)
	// @Tags checks
	// @Accept json
	// @Produce json
	// @Param request body CheckPhoneRequest true "Phone number to check"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /checks/realtime [post]
	checks.Post("/realtime", func(c *fiber.Ctx) error {
		var req CheckPhoneRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		result, err := checkService.CheckPhoneRealtime(req.PhoneNumber)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(result)
	})

	// @Summary Get check results
	// @Description Get check results with filters
	// @Tags checks
	// @Accept json
	// @Produce json
	// @Param phone_id query int false "Filter by phone ID"
	// @Param service_id query int false "Filter by service ID"
	// @Param limit query int false "Limit results" default(50)
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /checks/results [get]
	checks.Get("/results", func(c *fiber.Ctx) error {
		phoneID, _ := strconv.ParseUint(c.Query("phone_id", "0"), 10, 32)
		serviceID, _ := strconv.ParseUint(c.Query("service_id", "0"), 10, 32)
		limit, _ := strconv.Atoi(c.Query("limit", "50"))

		results, err := checkService.GetCheckResults(uint(phoneID), uint(serviceID), limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get results",
			})
		}

		return c.JSON(fiber.Map{
			"results": results,
			"count":   len(results),
		})
	})

	// @Summary Get latest results
	// @Description Get latest check results for all phones
	// @Tags checks
	// @Accept json
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /checks/latest [get]
	checks.Get("/latest", func(c *fiber.Ctx) error {
		results, err := checkService.GetLatestResults()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get latest results",
			})
		}

		return c.JSON(fiber.Map{
			"results": results,
		})
	})

	// @Summary Get screenshot
	// @Description Get screenshot from check result
	// @Tags checks
	// @Accept json
	// @Produce image/png
	// @Param id path int true "Check result ID"
	// @Success 200 {file} file
	// @Security BearerAuth
	// @Router /checks/screenshot/{id} [get]
	checks.Get("/screenshot/:id", func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid result ID",
			})
		}

		// Get check result
		var result models.CheckResult
		if err := checkService.GetDB().First(&result, id).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Result not found",
			})
		}

		// Send screenshot file
		return c.SendFile(result.Screenshot)
	})
}
