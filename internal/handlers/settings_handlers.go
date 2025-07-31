package handlers

import (
	"spam-checker/internal/middleware"
	"spam-checker/internal/models"
	"spam-checker/internal/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type UpdateSettingRequest struct {
	Value interface{} `json:"value" validate:"required"`
}

type CreateSettingRequest struct {
	Key      string `json:"key" validate:"required"`
	Value    string `json:"value" validate:"required"`
	Type     string `json:"type" validate:"required,oneof=string int bool float json"`
	Category string `json:"category" validate:"required"`
}

// RegisterSettingsRoutes registers settings routes
func RegisterSettingsRoutes(api fiber.Router, settingsService *services.SettingsService, authMiddleware *middleware.AuthMiddleware) {
	settings := api.Group("/settings")

	// All settings routes require admin or supervisor role
	settings.Use(authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor))

	// @Summary Get all settings
	// @Description Get all system settings
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Success 200 {array} models.SystemSettings
	// @Security BearerAuth
	// @Router /settings [get]
	settings.Get("/", func(c *fiber.Ctx) error {
		settings, err := settingsService.GetAllSettings()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get settings",
			})
		}

		return c.JSON(settings)
	})

	// @Summary Get settings by category
	// @Description Get all settings in a category
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Param category path string true "Category name"
	// @Success 200 {array} models.SystemSettings
	// @Security BearerAuth
	// @Router /settings/category/{category} [get]
	settings.Get("/category/:category", func(c *fiber.Ctx) error {
		category := c.Params("category")
		settings, err := settingsService.GetSettingsByCategory(category)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get settings",
			})
		}

		return c.JSON(settings)
	})

	// @Summary Get settings groups
	// @Description Get settings grouped by category
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Success 200 {object} map[string][]models.SystemSettings
	// @Security BearerAuth
	// @Router /settings/groups [get]
	settings.Get("/groups", func(c *fiber.Ctx) error {
		groups, err := settingsService.GetSettingsGroups()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get settings groups",
			})
		}

		return c.JSON(groups)
	})

	// @Summary Get setting
	// @Description Get a single setting by key
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Param key path string true "Setting key"
	// @Success 200 {object} models.SystemSettings
	// @Security BearerAuth
	// @Router /settings/{key} [get]
	settings.Get("/:key", func(c *fiber.Ctx) error {
		key := c.Params("key")
		setting, err := settingsService.GetSetting(key)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(setting)
	})

	// @Summary Update setting
	// @Description Update a setting value
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Param key path string true "Setting key"
	// @Param request body UpdateSettingRequest true "New value"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /settings/{key} [put]
	settings.Put("/:key", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
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

		return c.JSON(fiber.Map{
			"message": "Setting updated successfully",
		})
	})

	// @Summary Create setting
	// @Description Create a new setting (admin only)
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Param request body CreateSettingRequest true "Setting data"
	// @Success 201 {object} models.SystemSettings
	// @Security BearerAuth
	// @Router /settings [post]
	settings.Post("/", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
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
	})

	// @Summary Delete setting
	// @Description Delete a setting (admin only)
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Param key path string true "Setting key"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /settings/{key} [delete]
	settings.Delete("/:key", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		key := c.Params("key")

		if err := settingsService.DeleteSetting(key); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"message": "Setting deleted successfully",
		})
	})

	// @Summary Get database config
	// @Description Get database configuration and stats
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /settings/database/config [get]
	settings.Get("/database/config", func(c *fiber.Ctx) error {
		config, err := settingsService.GetDatabaseConfig()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get database config",
			})
		}

		return c.JSON(config)
	})

	// @Summary Get OCR config
	// @Description Get OCR configuration
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /settings/ocr/config [get]
	settings.Get("/ocr/config", func(c *fiber.Ctx) error {
		config, err := settingsService.GetOCRConfig()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get OCR config",
			})
		}

		return c.JSON(config)
	})

	// @Summary Update OCR config
	// @Description Update OCR configuration
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Param request body map[string]interface{} true "OCR configuration"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /settings/ocr/config [put]
	settings.Put("/ocr/config", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
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

		return c.JSON(fiber.Map{
			"message": "OCR configuration updated successfully",
		})
	})

	// @Summary Get check intervals
	// @Description Get check interval settings
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /settings/intervals [get]
	settings.Get("/intervals", func(c *fiber.Ctx) error {
		intervals, err := settingsService.GetCheckIntervals()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get check intervals",
			})
		}

		return c.JSON(intervals)
	})

	// @Summary Export settings
	// @Description Export all settings as JSON
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Success 200 {object} []models.SystemSettings
	// @Security BearerAuth
	// @Router /settings/export [get]
	settings.Get("/export", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		data, err := settingsService.ExportSettings()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to export settings",
			})
		}

		c.Set("Content-Type", "application/json")
		c.Set("Content-Disposition", "attachment; filename=settings.json")

		return c.Send(data)
	})

	// @Summary Get spam keywords
	// @Description Get all spam keywords
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Success 200 {array} models.SpamKeyword
	// @Security BearerAuth
	// @Router /settings/keywords [get]
	settings.Get("/keywords", func(c *fiber.Ctx) error {
		keywords, err := settingsService.GetSpamKeywords()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get keywords",
			})
		}

		return c.JSON(keywords)
	})

	// @Summary Create spam keyword
	// @Description Create a new spam keyword
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Param request body CreateKeywordRequest true "Keyword data"
	// @Success 201 {object} models.SpamKeyword
	// @Security BearerAuth
	// @Router /settings/keywords [post]
	settings.Post("/keywords", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		var req struct {
			Keyword   string `json:"keyword" validate:"required"`
			ServiceID *uint  `json:"service_id"`
		}

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
	})

	// @Summary Update spam keyword
	// @Description Update spam keyword
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Param id path int true "Keyword ID"
	// @Param request body UpdateKeywordRequest true "Keyword update data"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /settings/keywords/{id} [put]
	settings.Put("/keywords/:id", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid keyword ID",
			})
		}

		var req struct {
			Keyword   string `json:"keyword"`
			ServiceID *uint  `json:"service_id"`
			IsActive  *bool  `json:"is_active"`
		}

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

		return c.JSON(fiber.Map{
			"message": "Keyword updated successfully",
		})
	})

	// @Summary Delete spam keyword
	// @Description Delete spam keyword
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Param id path int true "Keyword ID"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /settings/keywords/{id} [delete]
	settings.Delete("/keywords/:id", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
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

		return c.JSON(fiber.Map{
			"message": "Keyword deleted successfully",
		})
	})

	// @Summary Get check schedules
	// @Description Get all check schedules
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Success 200 {array} models.CheckSchedule
	// @Security BearerAuth
	// @Router /settings/schedules [get]
	settings.Get("/schedules", func(c *fiber.Ctx) error {
		schedules, err := settingsService.GetCheckSchedules()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get schedules",
			})
		}

		return c.JSON(schedules)
	})

	// @Summary Create check schedule
	// @Description Create a new check schedule
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Param request body CreateScheduleRequest true "Schedule data"
	// @Success 201 {object} models.CheckSchedule
	// @Security BearerAuth
	// @Router /settings/schedules [post]
	settings.Post("/schedules", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		var req struct {
			Name           string `json:"name" validate:"required"`
			CronExpression string `json:"cron_expression" validate:"required"`
			IsActive       bool   `json:"is_active"`
		}

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
	})

	// @Summary Update check schedule
	// @Description Update check schedule
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Param id path int true "Schedule ID"
	// @Param request body UpdateScheduleRequest true "Schedule update data"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /settings/schedules/{id} [put]
	settings.Put("/schedules/:id", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid schedule ID",
			})
		}

		var req struct {
			Name           string `json:"name"`
			CronExpression string `json:"cron_expression"`
			IsActive       *bool  `json:"is_active"`
		}

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

		return c.JSON(fiber.Map{
			"message": "Schedule updated successfully",
		})
	})

	// @Summary Delete check schedule
	// @Description Delete check schedule
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Param id path int true "Schedule ID"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /settings/schedules/{id} [delete]
	settings.Delete("/schedules/:id", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
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
		return c.JSON(fiber.Map{
			"message": "Schedule delete successfully",
		})
	})

	// @Summary Import settings
	// @Description Import settings from JSON
	// @Tags settings
	// @Accept json
	// @Produce json
	// @Param settings body []models.SystemSettings true "Settings to import"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /settings/import [post]
	settings.Post("/import", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		data := c.Body()

		if err := settingsService.ImportSettings(data); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"message": "Settings imported successfully",
		})
	})
}
