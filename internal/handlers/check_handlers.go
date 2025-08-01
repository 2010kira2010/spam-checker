package handlers

import (
	"spam-checker/internal/middleware"
	"spam-checker/internal/models"
	"spam-checker/internal/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// CheckPhoneRequest represents phone check request
type CheckPhoneRequest struct {
	PhoneNumber string `json:"phone_number" validate:"required"`
}

// CheckAllRequest represents check all phones request
type CheckAllRequest struct {
	Force bool `json:"force"`
}

// CheckStartedResponse represents check started response
type CheckStartedResponse struct {
	Message string `json:"message"`
	PhoneID uint   `json:"phone_id,omitempty"`
}

// CheckResultsResponse represents check results response
type CheckResultsResponse struct {
	Results []models.CheckResult `json:"results"`
	Count   int                  `json:"count"`
}

// LatestResultsResponse represents latest results response
type LatestResultsResponse struct {
	Results []map[string]interface{} `json:"results"`
}

// RegisterCheckRoutes registers check routes
func RegisterCheckRoutes(api fiber.Router, checkService *services.CheckService, authMiddleware *middleware.AuthMiddleware) {
	checks := api.Group("/checks")

	checks.Post("/phone/:id", authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor), checkPhoneHandler(checkService))
	checks.Post("/all", authMiddleware.RequireRole(models.RoleAdmin), checkAllPhonesHandler(checkService))
	checks.Post("/realtime", checkRealtimeHandler(checkService))
	checks.Get("/results", getCheckResultsHandler(checkService))
	checks.Get("/latest", getLatestResultsHandler(checkService))
	checks.Get("/screenshot/:id", getScreenshotHandler(checkService))
}

// checkPhoneHandler godoc
// @Summary Check phone
// @Description Check a specific phone number
// @Tags checks
// @Accept json
// @Produce json
// @Param id path int true "Phone ID"
// @Success 200 {object} CheckStartedResponse
// @Security BearerAuth
// @Router /checks/phone/{id} [post]
func checkPhoneHandler(checkService *services.CheckService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid phone ID",
			})
		}

		// Start check in background
		go checkService.CheckPhoneNumber(uint(id))

		return c.JSON(CheckStartedResponse{
			Message: "Check started",
			PhoneID: uint(id),
		})
	}
}

// checkAllPhonesHandler godoc
// @Summary Check all phones
// @Description Check all active phone numbers
// @Tags checks
// @Accept json
// @Produce json
// @Param request body CheckAllRequest false "Check options"
// @Success 200 {object} CheckStartedResponse
// @Security BearerAuth
// @Router /checks/all [post]
func checkAllPhonesHandler(checkService *services.CheckService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req CheckAllRequest
		c.BodyParser(&req)

		// Start check in background
		go checkService.CheckAllPhones()

		return c.JSON(CheckStartedResponse{
			Message: "Check started for all active phones",
		})
	}
}

// checkRealtimeHandler godoc
// @Summary Check realtime
// @Description Check phone number in real-time (without saving)
// @Tags checks
// @Accept json
// @Produce json
// @Param request body CheckPhoneRequest true "Phone number to check"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /checks/realtime [post]
func checkRealtimeHandler(checkService *services.CheckService) fiber.Handler {
	return func(c *fiber.Ctx) error {
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
	}
}

// getCheckResultsHandler godoc
// @Summary Get check results
// @Description Get check results with filters
// @Tags checks
// @Accept json
// @Produce json
// @Param phone_id query int false "Filter by phone ID"
// @Param service_id query int false "Filter by service ID"
// @Param limit query int false "Limit results" default(50)
// @Success 200 {object} CheckResultsResponse
// @Security BearerAuth
// @Router /checks/results [get]
func getCheckResultsHandler(checkService *services.CheckService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		phoneID, _ := strconv.ParseUint(c.Query("phone_id", "0"), 10, 32)
		serviceID, _ := strconv.ParseUint(c.Query("service_id", "0"), 10, 32)
		limit, _ := strconv.Atoi(c.Query("limit", "50"))

		results, err := checkService.GetCheckResults(uint(phoneID), uint(serviceID), limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get results",
			})
		}

		return c.JSON(CheckResultsResponse{
			Results: results,
			Count:   len(results),
		})
	}
}

// getLatestResultsHandler godoc
// @Summary Get latest results
// @Description Get latest check results for all phones
// @Tags checks
// @Accept json
// @Produce json
// @Success 200 {object} LatestResultsResponse
// @Security BearerAuth
// @Router /checks/latest [get]
func getLatestResultsHandler(checkService *services.CheckService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		results, err := checkService.GetLatestResults()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get latest results",
			})
		}

		return c.JSON(LatestResultsResponse{
			Results: results,
		})
	}
}

// getScreenshotHandler godoc
// @Summary Get screenshot
// @Description Get screenshot from check result
// @Tags checks
// @Accept json
// @Produce image/png
// @Param id path int true "Check result ID"
// @Success 200 {file} file
// @Security BearerAuth
// @Router /checks/screenshot/{id} [get]
func getScreenshotHandler(checkService *services.CheckService) fiber.Handler {
	return func(c *fiber.Ctx) error {
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
	}
}
