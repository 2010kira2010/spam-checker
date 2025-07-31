package handlers

import (
	"fmt"
	"os"
	"spam-checker/internal/middleware"
	"spam-checker/internal/models"
	"spam-checker/internal/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type CreateADBGatewayRequest struct {
	Name        string `json:"name" validate:"required"`
	Host        string `json:"host" validate:"required"`
	Port        int    `json:"port" validate:"required,min=1,max=65535"`
	ServiceCode string `json:"service_code" validate:"required,oneof=yandex_aon kaspersky getcontact"`
}

type UpdateADBGatewayRequest struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	ServiceCode string `json:"service_code"`
	IsActive    *bool  `json:"is_active"`
}

// RegisterADBRoutes registers ADB gateway routes
func RegisterADBRoutes(api fiber.Router, adbService *services.ADBService, authMiddleware *middleware.AuthMiddleware) {
	adb := api.Group("/adb")

	// All ADB routes require admin or supervisor role
	adb.Use(authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor))

	// @Summary List ADB gateways
	// @Description Get all ADB gateways
	// @Tags adb
	// @Accept json
	// @Produce json
	// @Success 200 {array} models.ADBGateway
	// @Security BearerAuth
	// @Router /adb/gateways [get]
	adb.Get("/gateways", func(c *fiber.Ctx) error {
		gateways, err := adbService.ListGateways()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get gateways",
			})
		}

		return c.JSON(gateways)
	})

	// @Summary Get ADB gateway
	// @Description Get ADB gateway by ID
	// @Tags adb
	// @Accept json
	// @Produce json
	// @Param id path int true "Gateway ID"
	// @Success 200 {object} models.ADBGateway
	// @Security BearerAuth
	// @Router /adb/gateways/{id} [get]
	adb.Get("/gateways/:id", func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid gateway ID",
			})
		}

		gateway, err := adbService.GetGatewayByID(uint(id))
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(gateway)
	})

	// @Summary Create ADB gateway
	// @Description Create a new ADB gateway
	// @Tags adb
	// @Accept json
	// @Produce json
	// @Param request body CreateADBGatewayRequest true "Gateway data"
	// @Success 201 {object} models.ADBGateway
	// @Security BearerAuth
	// @Router /adb/gateways [post]
	adb.Post("/gateways", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		var req CreateADBGatewayRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		gateway := &models.ADBGateway{
			Name:        req.Name,
			Host:        req.Host,
			Port:        req.Port,
			ServiceCode: req.ServiceCode,
			IsActive:    true,
			Status:      "offline",
		}

		if err := adbService.CreateGateway(gateway); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusCreated).JSON(gateway)
	})

	// @Summary Update ADB gateway
	// @Description Update ADB gateway
	// @Tags adb
	// @Accept json
	// @Produce json
	// @Param id path int true "Gateway ID"
	// @Param request body UpdateADBGatewayRequest true "Gateway update data"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /adb/gateways/{id} [put]
	adb.Put("/gateways/:id", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid gateway ID",
			})
		}

		var req UpdateADBGatewayRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		updates := make(map[string]interface{})
		if req.Name != "" {
			updates["name"] = req.Name
		}
		if req.Host != "" {
			updates["host"] = req.Host
		}
		if req.Port > 0 {
			updates["port"] = req.Port
		}
		if req.ServiceCode != "" {
			updates["service_code"] = req.ServiceCode
		}
		if req.IsActive != nil {
			updates["is_active"] = *req.IsActive
		}

		if err := adbService.UpdateGateway(uint(id), updates); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"message": "Gateway updated successfully",
		})
	})

	// @Summary Delete ADB gateway
	// @Description Delete ADB gateway
	// @Tags adb
	// @Accept json
	// @Produce json
	// @Param id path int true "Gateway ID"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /adb/gateways/{id} [delete]
	adb.Delete("/gateways/:id", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid gateway ID",
			})
		}

		if err := adbService.DeleteGateway(uint(id)); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to delete gateway",
			})
		}

		return c.JSON(fiber.Map{
			"message": "Gateway deleted successfully",
		})
	})

	// @Summary Update gateway status
	// @Description Update ADB gateway status
	// @Tags adb
	// @Accept json
	// @Produce json
	// @Param id path int true "Gateway ID"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /adb/gateways/{id}/status [post]
	adb.Post("/gateways/:id/status", func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid gateway ID",
			})
		}

		if err := adbService.UpdateGatewayStatus(uint(id)); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update gateway status",
			})
		}

		// Get updated gateway
		gateway, _ := adbService.GetGatewayByID(uint(id))

		return c.JSON(fiber.Map{
			"message": "Gateway status updated",
			"status":  gateway.Status,
		})
	})

	// @Summary Update all gateway statuses
	// @Description Update status for all ADB gateways
	// @Tags adb
	// @Accept json
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /adb/gateways/status [post]
	adb.Post("/gateways/status", func(c *fiber.Ctx) error {
		if err := adbService.UpdateAllGatewayStatuses(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update gateway statuses",
			})
		}

		return c.JSON(fiber.Map{
			"message": "All gateway statuses updated",
		})
	})

	// @Summary Get device info
	// @Description Get Android device information
	// @Tags adb
	// @Accept json
	// @Produce json
	// @Param id path int true "Gateway ID"
	// @Success 200 {object} map[string]string
	// @Security BearerAuth
	// @Router /adb/gateways/{id}/device-info [get]
	adb.Get("/gateways/:id/device-info", func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid gateway ID",
			})
		}

		info, err := adbService.GetDeviceInfo(uint(id))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(info)
	})

	// @Summary Execute ADB command
	// @Description Execute custom ADB command on gateway
	// @Tags adb
	// @Accept json
	// @Produce json
	// @Param id path int true "Gateway ID"
	// @Param command body string true "ADB command"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /adb/gateways/{id}/execute [post]
	adb.Post("/gateways/:id/execute", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid gateway ID",
			})
		}

		var req struct {
			Command string `json:"command" validate:"required"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		output, err := adbService.ExecuteCommand(uint(id), req.Command)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"output": output,
		})
	})

	// @Summary Restart device
	// @Description Restart Android device
	// @Tags adb
	// @Accept json
	// @Produce json
	// @Param id path int true "Gateway ID"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /adb/gateways/{id}/restart [post]
	adb.Post("/gateways/:id/restart", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid gateway ID",
			})
		}

		if err := adbService.RestartDevice(uint(id)); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"message": "Device restart initiated",
		})
	})

	// @Summary Install APK
	// @Description Install APK on Android device
	// @Tags adb
	// @Accept multipart/form-data
	// @Produce json
	// @Param id path int true "Gateway ID"
	// @Param apk formData file true "APK file"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /adb/gateways/{id}/install-apk [post]
	adb.Post("/gateways/:id/install-apk", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid gateway ID",
			})
		}

		// Get uploaded file
		file, err := c.FormFile("apk")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "APK file is required",
			})
		}

		// Save file temporarily
		tempPath := fmt.Sprintf("/tmp/%s", file.Filename)
		if err := c.SaveFile(file, tempPath); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to save APK file",
			})
		}

		// Install APK
		if err := adbService.InstallAPK(uint(id), tempPath); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Clean up temp file
		os.Remove(tempPath)

		return c.JSON(fiber.Map{
			"message": "APK installed successfully",
		})
	})
}
