package handlers

import (
	"fmt"
	"spam-checker/internal/middleware"
	"spam-checker/internal/models"
	"spam-checker/internal/services"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// CreateAPIServiceRequest represents API service creation request
type CreateAPIServiceRequest struct {
	Name         string `json:"name" validate:"required"`
	ServiceCode  string `json:"service_code" validate:"required"`
	APIURL       string `json:"api_url" validate:"required"`
	Headers      string `json:"headers"`
	Method       string `json:"method" validate:"required,oneof=GET POST"`
	RequestBody  string `json:"request_body"`
	Timeout      int    `json:"timeout" validate:"min=1,max=300"`
	KeywordPaths string `json:"keyword_paths"`
	ResponsePath string `json:"response_path"`
}

// UpdateAPIServiceRequest represents API service update request
type UpdateAPIServiceRequest struct {
	Name         string `json:"name"`
	ServiceCode  string `json:"service_code"`
	APIURL       string `json:"api_url"`
	Headers      string `json:"headers"`
	Method       string `json:"method"`
	RequestBody  string `json:"request_body"`
	Timeout      *int   `json:"timeout"`
	IsActive     *bool  `json:"is_active"`
	KeywordPaths string `json:"keyword_paths"`
	ResponsePath string `json:"response_path"`
}

// TestAPIServiceRequest represents API service test request
type TestAPIServiceRequest struct {
	PhoneNumber string `json:"phone_number" validate:"required"`
}

// RegisterAPIServiceRoutes registers API service routes
func RegisterAPIServiceRoutes(api fiber.Router, apiService *services.APICheckService, authMiddleware *middleware.AuthMiddleware) {
	apis := api.Group("/api-services")

	// All API service routes require admin or supervisor role
	apis.Use(authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor))

	apis.Get("/", listAPIServicesHandler(apiService))
	apis.Get("/:id", getAPIServiceHandler(apiService))
	apis.Post("/", authMiddleware.RequireRole(models.RoleAdmin), createAPIServiceHandler(apiService))
	apis.Put("/:id", authMiddleware.RequireRole(models.RoleAdmin), updateAPIServiceHandler(apiService))
	apis.Delete("/:id", authMiddleware.RequireRole(models.RoleAdmin), deleteAPIServiceHandler(apiService))
	apis.Post("/:id/test", testAPIServiceHandler(apiService))
	apis.Post("/:id/toggle", toggleAPIServiceHandler(apiService))
}

// listAPIServicesHandler godoc
// @Summary List API services
// @Description Get all API services
// @Tags api-services
// @Accept json
// @Produce json
// @Success 200 {array} models.APIService
// @Security BearerAuth
// @Router /api-services [get]
func listAPIServicesHandler(apiService *services.APICheckService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		services, err := apiService.ListAPIServices()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get API services",
			})
		}

		return c.JSON(services)
	}
}

// getAPIServiceHandler godoc
// @Summary Get API service
// @Description Get API service by ID
// @Tags api-services
// @Accept json
// @Produce json
// @Param id path int true "API Service ID"
// @Success 200 {object} models.APIService
// @Security BearerAuth
// @Router /api-services/{id} [get]
func getAPIServiceHandler(apiService *services.APICheckService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid API service ID",
			})
		}

		service, err := apiService.GetAPIServiceByID(uint(id))
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(service)
	}
}

// createAPIServiceHandler godoc
// @Summary Create API service
// @Description Create a new API service
// @Tags api-services
// @Accept json
// @Produce json
// @Param request body CreateAPIServiceRequest true "API service data"
// @Success 201 {object} models.APIService
// @Security BearerAuth
// @Router /api-services [post]
func createAPIServiceHandler(apiService *services.APICheckService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req CreateAPIServiceRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		// Validate service code - allow custom codes
		validPredefinedCodes := map[string]bool{
			"yandex_aon": true,
			"kaspersky":  true,
			"getcontact": true,
		}

		// If not a predefined code, ensure it starts with "custom" or is "custom"
		if !validPredefinedCodes[req.ServiceCode] {
			if req.ServiceCode != "custom" && !strings.HasPrefix(req.ServiceCode, "custom_") {
				// Auto-prefix with custom_ for non-standard codes
				req.ServiceCode = "custom_" + strings.ToLower(strings.ReplaceAll(req.ServiceCode, " ", "_"))
			}
		}

		// Set default timeout if not provided
		timeout := req.Timeout
		if timeout == 0 {
			timeout = 30
		}

		// Set default headers if not provided
		headers := req.Headers
		if headers == "" {
			headers = "{}"
		}

		service := &models.APIService{
			Name:         req.Name,
			ServiceCode:  req.ServiceCode,
			APIURL:       req.APIURL,
			Headers:      headers,
			Method:       req.Method,
			RequestBody:  req.RequestBody,
			Timeout:      timeout,
			IsActive:     true,
			KeywordPaths: req.KeywordPaths,
			ResponsePath: req.ResponsePath,
		}

		if err := apiService.CreateAPIService(service); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusCreated).JSON(service)
	}
}

// updateAPIServiceHandler godoc
// @Summary Update API service
// @Description Update API service
// @Tags api-services
// @Accept json
// @Produce json
// @Param id path int true "API Service ID"
// @Param request body UpdateAPIServiceRequest true "API service update data"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /api-services/{id} [put]
func updateAPIServiceHandler(apiService *services.APICheckService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid API service ID",
			})
		}

		var req UpdateAPIServiceRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		updates := make(map[string]interface{})
		if req.Name != "" {
			updates["name"] = req.Name
		}
		if req.ServiceCode != "" {
			// Validate and normalize service code
			validPredefinedCodes := map[string]bool{
				"yandex_aon": true,
				"kaspersky":  true,
				"getcontact": true,
			}

			if !validPredefinedCodes[req.ServiceCode] {
				if req.ServiceCode != "custom" && !strings.HasPrefix(req.ServiceCode, "custom_") {
					req.ServiceCode = "custom_" + strings.ToLower(strings.ReplaceAll(req.ServiceCode, " ", "_"))
				}
			}
			updates["service_code"] = req.ServiceCode
		}
		if req.APIURL != "" {
			updates["api_url"] = req.APIURL
		}
		if req.Headers != "" {
			updates["headers"] = req.Headers
		}
		if req.Method != "" {
			updates["method"] = req.Method
		}
		if req.RequestBody != "" {
			updates["request_body"] = req.RequestBody
		}
		if req.Timeout != nil {
			updates["timeout"] = *req.Timeout
		}
		if req.IsActive != nil {
			updates["is_active"] = *req.IsActive
		}
		if req.KeywordPaths != "" {
			updates["keyword_paths"] = req.KeywordPaths
		}
		if req.ResponsePath != "" {
			updates["response_path"] = req.ResponsePath
		}

		if err := apiService.UpdateAPIService(uint(id), updates); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(MessageResponse{
			Message: "API service updated successfully",
		})
	}
}

// deleteAPIServiceHandler godoc
// @Summary Delete API service
// @Description Delete API service
// @Tags api-services
// @Accept json
// @Produce json
// @Param id path int true "API Service ID"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /api-services/{id} [delete]
func deleteAPIServiceHandler(apiService *services.APICheckService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid API service ID",
			})
		}

		if err := apiService.DeleteAPIService(uint(id)); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to delete API service",
			})
		}

		return c.JSON(MessageResponse{
			Message: "API service deleted successfully",
		})
	}
}

// testAPIServiceHandler godoc
// @Summary Test API service
// @Description Test API service with a phone number
// @Tags api-services
// @Accept json
// @Produce json
// @Param id path int true "API Service ID"
// @Param request body TestAPIServiceRequest true "Test phone number"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /api-services/{id}/test [post]
func testAPIServiceHandler(apiService *services.APICheckService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid API service ID",
			})
		}

		var req TestAPIServiceRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		result, err := apiService.TestAPIService(uint(id), req.PhoneNumber)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(result)
	}
}

// toggleAPIServiceHandler godoc
// @Summary Toggle API service
// @Description Enable or disable API service
// @Tags api-services
// @Accept json
// @Produce json
// @Param id path int true "API Service ID"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /api-services/{id}/toggle [post]
func toggleAPIServiceHandler(apiService *services.APICheckService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid API service ID",
			})
		}

		service, err := apiService.GetAPIServiceByID(uint(id))
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Toggle active status
		updates := map[string]interface{}{
			"is_active": !service.IsActive,
		}

		if err := apiService.UpdateAPIService(uint(id), updates); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		status := "disabled"
		if !service.IsActive {
			status = "enabled"
		}

		return c.JSON(MessageResponse{
			Message: fmt.Sprintf("API service %s successfully", status),
		})
	}
}
