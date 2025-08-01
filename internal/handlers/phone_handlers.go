package handlers

import (
	"spam-checker/internal/middleware"
	"spam-checker/internal/models"
	"spam-checker/internal/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// CreatePhoneRequest represents phone creation request
type CreatePhoneRequest struct {
	Number      string `json:"number" validate:"required"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
}

// UpdatePhoneRequest represents phone update request
type UpdatePhoneRequest struct {
	Number      string `json:"number"`
	Description string `json:"description"`
	IsActive    *bool  `json:"is_active"`
}

// PhonesListResponse represents phones list response
type PhonesListResponse struct {
	Phones []models.PhoneNumber `json:"phones"`
	Total  int64                `json:"total"`
	Page   int                  `json:"page"`
	Limit  int                  `json:"limit"`
}

// ImportPhonesResponse represents import phones response
type ImportPhonesResponse struct {
	Imported int      `json:"imported"`
	Errors   []string `json:"errors"`
}

// RegisterPhoneRoutes registers phone number routes
func RegisterPhoneRoutes(api fiber.Router, phoneService *services.PhoneService, authMiddleware *middleware.AuthMiddleware) {
	phones := api.Group("/phones")

	phones.Get("/", listPhonesHandler(phoneService))
	phones.Get("/stats", getPhoneStatsHandler(phoneService))
	phones.Get("/export", exportPhonesHandler(phoneService))
	phones.Get("/:id", getPhoneByIDHandler(phoneService))
	phones.Post("/", authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor), createPhoneHandler(phoneService))
	phones.Put("/:id", authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor), updatePhoneHandler(phoneService))
	phones.Delete("/:id", authMiddleware.RequireRole(models.RoleAdmin), deletePhoneHandler(phoneService))
	phones.Post("/import", authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor), importPhonesHandler(phoneService))
}

// listPhonesHandler godoc
// @Summary List phones
// @Description Get list of phone numbers with pagination
// @Tags phones
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Param search query string false "Search query"
// @Param is_active query bool false "Filter by active status"
// @Success 200 {object} PhonesListResponse
// @Security BearerAuth
// @Router /phones [get]
func listPhonesHandler(phoneService *services.PhoneService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		page, _ := strconv.Atoi(c.Query("page", "1"))
		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		search := c.Query("search")

		var isActive *bool
		if activeStr := c.Query("is_active"); activeStr != "" {
			active := activeStr == "true"
			isActive = &active
		}

		offset := (page - 1) * limit
		phones, total, err := phoneService.ListPhones(offset, limit, search, isActive)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get phones",
			})
		}

		return c.JSON(PhonesListResponse{
			Phones: phones,
			Total:  total,
			Page:   page,
			Limit:  limit,
		})
	}
}

// getPhoneByIDHandler godoc
// @Summary Get phone
// @Description Get phone number by ID
// @Tags phones
// @Accept json
// @Produce json
// @Param id path int true "Phone ID"
// @Success 200 {object} models.PhoneNumber
// @Security BearerAuth
// @Router /phones/{id} [get]
func getPhoneByIDHandler(phoneService *services.PhoneService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid phone ID",
			})
		}

		phone, err := phoneService.GetPhoneByID(uint(id))
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(phone)
	}
}

// createPhoneHandler godoc
// @Summary Create phone
// @Description Create a new phone number
// @Tags phones
// @Accept json
// @Produce json
// @Param request body CreatePhoneRequest true "Phone data"
// @Success 201 {object} models.PhoneNumber
// @Security BearerAuth
// @Router /phones [post]
func createPhoneHandler(phoneService *services.PhoneService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req CreatePhoneRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		userID := middleware.GetUserID(c)
		phone := &models.PhoneNumber{
			Number:      req.Number,
			Description: req.Description,
			IsActive:    req.IsActive,
			CreatedBy:   userID,
		}

		if err := phoneService.CreatePhone(phone); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusCreated).JSON(phone)
	}
}

// updatePhoneHandler godoc
// @Summary Update phone
// @Description Update phone number
// @Tags phones
// @Accept json
// @Produce json
// @Param id path int true "Phone ID"
// @Param request body UpdatePhoneRequest true "Phone update data"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /phones/{id} [put]
func updatePhoneHandler(phoneService *services.PhoneService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid phone ID",
			})
		}

		var req UpdatePhoneRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		updates := make(map[string]interface{})
		if req.Number != "" {
			updates["number"] = req.Number
		}
		if req.Description != "" {
			updates["description"] = req.Description
		}
		if req.IsActive != nil {
			updates["is_active"] = *req.IsActive
		}

		if err := phoneService.UpdatePhone(uint(id), updates); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(MessageResponse{
			Message: "Phone updated successfully",
		})
	}
}

// deletePhoneHandler godoc
// @Summary Delete phone
// @Description Delete phone number
// @Tags phones
// @Accept json
// @Produce json
// @Param id path int true "Phone ID"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /phones/{id} [delete]
func deletePhoneHandler(phoneService *services.PhoneService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid phone ID",
			})
		}

		if err := phoneService.DeletePhone(uint(id)); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to delete phone",
			})
		}

		return c.JSON(MessageResponse{
			Message: "Phone deleted successfully",
		})
	}
}

// importPhonesHandler godoc
// @Summary Import phones
// @Description Import phone numbers from CSV file
// @Tags phones
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "CSV file"
// @Success 200 {object} ImportPhonesResponse
// @Security BearerAuth
// @Router /phones/import [post]
func importPhonesHandler(phoneService *services.PhoneService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		file, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "File is required",
			})
		}

		// Open file
		src, err := file.Open()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to open file",
			})
		}
		defer src.Close()

		userID := middleware.GetUserID(c)
		imported, errors, err := phoneService.ImportPhones(src, userID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(ImportPhonesResponse{
			Imported: imported,
			Errors:   errors,
		})
	}
}

// exportPhonesHandler godoc
// @Summary Export phones
// @Description Export phone numbers to CSV file
// @Tags phones
// @Produce text/csv
// @Param is_active query bool false "Filter by active status"
// @Success 200 {file} file
// @Security BearerAuth
// @Router /phones/export [get]
func exportPhonesHandler(phoneService *services.PhoneService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var isActive *bool
		if activeStr := c.Query("is_active"); activeStr != "" {
			active := activeStr == "true"
			isActive = &active
		}

		c.Set("Content-Type", "text/csv")
		c.Set("Content-Disposition", "attachment; filename=phones.csv")

		writer := &responseWriter{ctx: c}
		if err := phoneService.ExportPhones(writer, isActive); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to export phones",
			})
		}

		return nil
	}
}

// getPhoneStatsHandler godoc
// @Summary Get phone stats
// @Description Get phone statistics
// @Tags phones
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /phones/stats [get]
func getPhoneStatsHandler(phoneService *services.PhoneService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		stats, err := phoneService.GetPhoneStats()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get statistics",
			})
		}

		return c.JSON(stats)
	}
}

// responseWriter implements io.Writer for Fiber context
type responseWriter struct {
	ctx *fiber.Ctx
}

func (w *responseWriter) Write(p []byte) (n int, err error) {
	return w.ctx.Write(p)
}
