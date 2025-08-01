package handlers

import (
	"spam-checker/internal/middleware"
	"spam-checker/internal/models"
	"spam-checker/internal/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// UpdateSettingRequest represents setting update request
type UpdateSettingRequest struct {
	Value interface{} `json:"value" validate:"required"`
}

// CreateSettingRequest represents setting creation request
type CreateSettingRequest struct {
	Key      string `json:"key" validate:"required"`
	Value    string `json:"value" validate:"required"`
	Type     string `json:"type" validate:"required,oneof=string int bool float json"`
	Category string `json:"category" validate:"required"`
}

// CreateKeywordRequest represents keyword creation request
type CreateKeywordRequest struct {
	Keyword   string `json:"keyword" validate:"required"`
	ServiceID *uint  `json:"service_id"`
}

// UpdateKeywordRequest represents keyword update request
type UpdateKeywordRequest struct {
	Keyword   string `json:"keyword"`
	ServiceID *uint  `json:"service_id"`
	IsActive  *bool  `json:"is_active"`
}

// CreateScheduleRequest represents schedule creation request
type CreateScheduleRequest struct {
	Name           string `json:"name" validate:"required"`
	CronExpression string `json:"cron_expression" validate:"required"`
	IsActive       bool   `json:"is_active"`
}

// UpdateScheduleRequest represents schedule update request
type UpdateScheduleRequest struct {
	Name           string `json:"name"`
	CronExpression string `json:"cron_expression"`
	IsActive       *bool  `json:"is_active"`
}

// RegisterSettingsRoutes registers settings routes
func RegisterSettingsRoutes(api fiber.Router, settingsService *services.SettingsService, authMiddleware *middleware.AuthMiddleware) {
	settings := api.Group("/settings")

	// All settings routes require admin or supervisor role
	settings.Use(authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor))

	settings.Get("/", getAllSettingsHandler(settingsService))
	settings.Get("/category/:category", getSettingsByCategoryHandler(settingsService))
	settings.Get("/groups", getSettingsGroupsHandler(settingsService))
	settings.Get("/database/config", getDatabaseConfigHandler(settingsService))
	settings.Get("/ocr/config", getOCRConfigHandler(settingsService))
	settings.Put("/ocr/config", authMiddleware.RequireRole(models.RoleAdmin), updateOCRConfigHandler(settingsService))
	settings.Get("/intervals", getCheckIntervalsHandler(settingsService))
	settings.Get("/export", authMiddleware.RequireRole(models.RoleAdmin), exportSettingsHandler(settingsService))
	settings.Post("/import", authMiddleware.RequireRole(models.RoleAdmin), importSettingsHandler(settingsService))
	settings.Get("/keywords", getSpamKeywordsHandler(settingsService))
	settings.Post("/keywords", authMiddleware.RequireRole(models.RoleAdmin), createSpamKeywordHandler(settingsService))
	settings.Put("/keywords/:id", authMiddleware.RequireRole(models.RoleAdmin), updateSpamKeywordHandler(settingsService))
	settings.Delete("/keywords/:id", authMiddleware.RequireRole(models.RoleAdmin), deleteSpamKeywordHandler(settingsService))
	settings.Get("/schedules", getCheckSchedulesHandler(settingsService))
	settings.Post("/schedules", authMiddleware.RequireRole(models.RoleAdmin), createCheckScheduleHandler(settingsService))
	settings.Put("/schedules/:id", authMiddleware.RequireRole(models.RoleAdmin), updateCheckScheduleHandler(settingsService))
	settings.Delete("/schedules/:id", authMiddleware.RequireRole(models.RoleAdmin), deleteCheckScheduleHandler(settingsService))
	settings.Get("/:key", getSettingHandler(settingsService))
	settings.Put("/:key", authMiddleware.RequireRole(models.RoleAdmin), updateSettingHandler(settingsService))
	settings.Post("/", authMiddleware.RequireRole(models.RoleAdmin), createSettingHandler(settingsService))
	settings.Delete("/:key", authMiddleware.RequireRole(models.RoleAdmin), deleteSettingHandler(settingsService))
}

// getAllSettingsHandler godoc
// @Summary Get all settings
// @Description Get all system settings
// @Tags settings
// @Accept json
// @Produce json
// @Success 200 {array} models.SystemSettings
// @Security BearerAuth
// @Router /settings [get]
func getAllSettingsHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		settings, err := settingsService.GetAllSettings()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get settings",
			})
		}

		return c.JSON(settings)
	}
}

// getSettingsByCategoryHandler godoc
// @Summary Get settings by category
// @Description Get all settings in a category
// @Tags settings
// @Accept json
// @Produce json
// @Param category path string true "Category name"
// @Success 200 {array} models.SystemSettings
// @Security BearerAuth
// @Router /settings/category/{category} [get]
func getSettingsByCategoryHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		category := c.Params("category")
		settings, err := settingsService.GetSettingsByCategory(category)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get settings",
			})
		}

		return c.JSON(settings)
	}
}

// getSettingsGroupsHandler godoc
// @Summary Get settings groups
// @Description Get settings grouped by category
// @Tags settings
// @Accept json
// @Produce json
// @Success 200 {object} map[string][]models.SystemSettings
// @Security BearerAuth
// @Router /settings/groups [get]
func getSettingsGroupsHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		groups, err := settingsService.GetSettingsGroups()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get settings groups",
			})
		}

		return c.JSON(groups)
	}
}

// getSettingHandler godoc
// @Summary Get setting
// @Description Get a single setting by key
// @Tags settings
// @Accept json
// @Produce json
// @Param key path string true "Setting key"
// @Success 200 {object} models.SystemSettings
// @Security BearerAuth
// @Router /settings/{key} [get]
func getSettingHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := c.Params("key")
		setting, err := settingsService.GetSetting(key)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(setting)
	}
}

// updateSettingHandler godoc
// @Summary Update setting
// @Description Update a setting value
// @Tags settings
// @Accept json
// @Produce json
// @Param key path string true "Setting key"
// @Param request body UpdateSettingRequest true "New value"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /settings/{key} [put]
func updateSettingHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := c.Params("key")

		var req UpdateSettingRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		if err := settingsService.UpdateSetting(key, req.Value); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(MessageResponse{
			Message: "Setting updated successfully",
		})
	}
}

// createSettingHandler godoc
// @Summary Create setting
// @Description Create a new setting (admin only)
// @Tags settings
// @Accept json
// @Produce json
// @Param request body CreateSettingRequest true "Setting data"
// @Success 201 {object} models.SystemSettings
// @Security BearerAuth
// @Router /settings [post]
func createSettingHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req CreateSettingRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		setting := &models.SystemSettings{
			Key:      req.Key,
			Value:    req.Value,
			Type:     req.Type,
			Category: req.Category,
		}

		if err := settingsService.CreateSetting(setting); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusCreated).JSON(setting)
	}
}

// deleteSettingHandler godoc
// @Summary Delete setting
// @Description Delete a setting (admin only)
// @Tags settings
// @Accept json
// @Produce json
// @Param key path string true "Setting key"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /settings/{key} [delete]
func deleteSettingHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := c.Params("key")

		if err := settingsService.DeleteSetting(key); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(MessageResponse{
			Message: "Setting deleted successfully",
		})
	}
}

// getDatabaseConfigHandler godoc
// @Summary Get database config
// @Description Get database configuration and stats
// @Tags settings
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /settings/database/config [get]
func getDatabaseConfigHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		config, err := settingsService.GetDatabaseConfig()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get database config",
			})
		}

		return c.JSON(config)
	}
}

// getOCRConfigHandler godoc
// @Summary Get OCR config
// @Description Get OCR configuration
// @Tags settings
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /settings/ocr/config [get]
func getOCRConfigHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		config, err := settingsService.GetOCRConfig()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get OCR config",
			})
		}

		return c.JSON(config)
	}
}

// updateOCRConfigHandler godoc
// @Summary Update OCR config
// @Description Update OCR configuration
// @Tags settings
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "OCR configuration"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /settings/ocr/config [put]
func updateOCRConfigHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var config map[string]interface{}
		if err := c.BodyParser(&config); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		if err := settingsService.UpdateOCRConfig(config); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(MessageResponse{
			Message: "OCR configuration updated successfully",
		})
	}
}

// getCheckIntervalsHandler godoc
// @Summary Get check intervals
// @Description Get check interval settings
// @Tags settings
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /settings/intervals [get]
func getCheckIntervalsHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		intervals, err := settingsService.GetCheckIntervals()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get check intervals",
			})
		}

		return c.JSON(intervals)
	}
}

// exportSettingsHandler godoc
// @Summary Export settings
// @Description Export all settings as JSON
// @Tags settings
// @Accept json
// @Produce json
// @Success 200 {object} []models.SystemSettings
// @Security BearerAuth
// @Router /settings/export [get]
func exportSettingsHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		data, err := settingsService.ExportSettings()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to export settings",
			})
		}

		c.Set("Content-Type", "application/json")
		c.Set("Content-Disposition", "attachment; filename=settings.json")

		return c.Send(data)
	}
}

// getSpamKeywordsHandler godoc
// @Summary Get spam keywords
// @Description Get all spam keywords
// @Tags settings
// @Accept json
// @Produce json
// @Success 200 {array} models.SpamKeyword
// @Security BearerAuth
// @Router /settings/keywords [get]
func getSpamKeywordsHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		keywords, err := settingsService.GetSpamKeywords()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get keywords",
			})
		}

		return c.JSON(keywords)
	}
}

// createSpamKeywordHandler godoc
// @Summary Create spam keyword
// @Description Create a new spam keyword
// @Tags settings
// @Accept json
// @Produce json
// @Param request body CreateKeywordRequest true "Keyword data"
// @Success 201 {object} models.SpamKeyword
// @Security BearerAuth
// @Router /settings/keywords [post]
func createSpamKeywordHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req CreateKeywordRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		keyword := &models.SpamKeyword{
			Keyword:   req.Keyword,
			ServiceID: req.ServiceID,
			IsActive:  true,
		}

		if err := settingsService.CreateSpamKeyword(keyword); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusCreated).JSON(keyword)
	}
}

// updateSpamKeywordHandler godoc
// @Summary Update spam keyword
// @Description Update spam keyword
// @Tags settings
// @Accept json
// @Produce json
// @Param id path int true "Keyword ID"
// @Param request body UpdateKeywordRequest true "Keyword update data"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /settings/keywords/{id} [put]
func updateSpamKeywordHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid keyword ID",
			})
		}

		var req UpdateKeywordRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		updates := make(map[string]interface{})
		if req.Keyword != "" {
			updates["keyword"] = req.Keyword
		}
		if req.ServiceID != nil {
			updates["service_id"] = req.ServiceID
		}
		if req.IsActive != nil {
			updates["is_active"] = *req.IsActive
		}

		if err := settingsService.UpdateSpamKeyword(uint(id), updates); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(MessageResponse{
			Message: "Keyword updated successfully",
		})
	}
}

// deleteSpamKeywordHandler godoc
// @Summary Delete spam keyword
// @Description Delete spam keyword
// @Tags settings
// @Accept json
// @Produce json
// @Param id path int true "Keyword ID"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /settings/keywords/{id} [delete]
func deleteSpamKeywordHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid keyword ID",
			})
		}

		if err := settingsService.DeleteSpamKeyword(uint(id)); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(MessageResponse{
			Message: "Keyword deleted successfully",
		})
	}
}

// getCheckSchedulesHandler godoc
// @Summary Get check schedules
// @Description Get all check schedules
// @Tags settings
// @Accept json
// @Produce json
// @Success 200 {array} models.CheckSchedule
// @Security BearerAuth
// @Router /settings/schedules [get]
func getCheckSchedulesHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		schedules, err := settingsService.GetCheckSchedules()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get schedules",
			})
		}

		return c.JSON(schedules)
	}
}

// createCheckScheduleHandler godoc
// @Summary Create check schedule
// @Description Create a new check schedule
// @Tags settings
// @Accept json
// @Produce json
// @Param request body CreateScheduleRequest true "Schedule data"
// @Success 201 {object} models.CheckSchedule
// @Security BearerAuth
// @Router /settings/schedules [post]
func createCheckScheduleHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req CreateScheduleRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		schedule := &models.CheckSchedule{
			Name:           req.Name,
			CronExpression: req.CronExpression,
			IsActive:       req.IsActive,
		}

		if err := settingsService.CreateCheckSchedule(schedule); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusCreated).JSON(schedule)
	}
}

// updateCheckScheduleHandler godoc
// @Summary Update check schedule
// @Description Update check schedule
// @Tags settings
// @Accept json
// @Produce json
// @Param id path int true "Schedule ID"
// @Param request body UpdateScheduleRequest true "Schedule update data"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /settings/schedules/{id} [put]
func updateCheckScheduleHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid schedule ID",
			})
		}

		var req UpdateScheduleRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		updates := make(map[string]interface{})
		if req.Name != "" {
			updates["name"] = req.Name
		}
		if req.CronExpression != "" {
			updates["cron_expression"] = req.CronExpression
		}
		if req.IsActive != nil {
			updates["is_active"] = *req.IsActive
		}

		if err := settingsService.UpdateCheckSchedule(uint(id), updates); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(MessageResponse{
			Message: "Schedule updated successfully",
		})
	}
}

// deleteCheckScheduleHandler godoc
// @Summary Delete check schedule
// @Description Delete check schedule
// @Tags settings
// @Accept json
// @Produce json
// @Param id path int true "Schedule ID"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /settings/schedules/{id} [delete]
func deleteCheckScheduleHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid schedule ID",
			})
		}

		if err := settingsService.DeleteCheckSchedule(uint(id)); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.JSON(MessageResponse{
			Message: "Schedule deleted successfully",
		})
	}
}

// importSettingsHandler godoc
// @Summary Import settings
// @Description Import settings from JSON
// @Tags settings
// @Accept json
// @Produce json
// @Param settings body []models.SystemSettings true "Settings to import"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /settings/import [post]
func importSettingsHandler(settingsService *services.SettingsService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		data := c.Body()

		if err := settingsService.ImportSettings(data); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(MessageResponse{
			Message: "Settings imported successfully",
		})
	}
}
