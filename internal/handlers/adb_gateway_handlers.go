package handlers

import (
	"fmt"
	"io"
	"os"
	"spam-checker/internal/middleware"
	"spam-checker/internal/models"
	"spam-checker/internal/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// CreateADBGatewayRequest represents ADB gateway creation request
type CreateADBGatewayRequest struct {
	Name        string `json:"name" validate:"required"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	ServiceCode string `json:"service_code" validate:"required,oneof=yandex_aon kaspersky getcontact"`
	IsDocker    bool   `json:"is_docker"`
}

// UpdateADBGatewayRequest represents ADB gateway update request
type UpdateADBGatewayRequest struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	ServiceCode string `json:"service_code"`
	IsActive    *bool  `json:"is_active"`
}

// ExecuteCommandRequest represents ADB command execution request
type ExecuteCommandRequest struct {
	Command string `json:"command" validate:"required"`
}

// GatewayStatusResponse represents gateway status response
type GatewayStatusResponse struct {
	Message string `json:"message"`
	Status  string `json:"status,omitempty"`
}

// CommandOutputResponse represents command output response
type CommandOutputResponse struct {
	Output string `json:"output"`
}

// RegisterADBRoutes registers ADB gateway routes
func RegisterADBRoutes(api fiber.Router, adbService *services.ADBService, authMiddleware *middleware.AuthMiddleware) {
	adb := api.Group("/adb")

	// All ADB routes require admin or supervisor role
	adb.Use(authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor))

	adb.Get("/gateways", listGatewaysHandler(adbService))
	adb.Get("/gateways/:id", getGatewayHandler(adbService))
	adb.Post("/gateways", authMiddleware.RequireRole(models.RoleAdmin), createGatewayHandler(adbService))
	adb.Post("/gateways/docker", authMiddleware.RequireRole(models.RoleAdmin), createDockerGatewayHandler(adbService))
	adb.Put("/gateways/:id", authMiddleware.RequireRole(models.RoleAdmin), updateGatewayHandler(adbService))
	adb.Delete("/gateways/:id", authMiddleware.RequireRole(models.RoleAdmin), deleteGatewayHandler(adbService))
	adb.Post("/gateways/:id/status", updateGatewayStatusHandler(adbService))
	adb.Post("/gateways/status", updateAllGatewayStatusesHandler(adbService))
	adb.Get("/gateways/:id/device-info", getDeviceInfoHandler(adbService))
	adb.Post("/gateways/:id/execute", authMiddleware.RequireRole(models.RoleAdmin), executeCommandHandler(adbService))
	adb.Post("/gateways/:id/restart", authMiddleware.RequireRole(models.RoleAdmin), restartDeviceHandler(adbService))
	adb.Post("/gateways/:id/install-apk", authMiddleware.RequireRole(models.RoleAdmin), installAPKHandler(adbService))
	adb.Get("/docker/status", checkDockerStatusHandler(adbService))
	adb.Get("/docker/containers", listDockerContainersHandler(adbService))
}

// listGatewaysHandler godoc
// @Summary List ADB gateways
// @Description Get all ADB gateways
// @Tags adb
// @Accept json
// @Produce json
// @Success 200 {object} []models.ADBGateway
// @Security BearerAuth
// @Router /adb/gateways [get]
func listGatewaysHandler(adbService *services.ADBService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		gateways, err := adbService.ListGateways()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get gateways",
			})
		}

		return c.JSON(gateways)
	}
}

// getGatewayHandler godoc
// @Summary Get ADB gateway
// @Description Get ADB gateway by ID
// @Tags adb
// @Accept json
// @Produce json
// @Param id path int true "Gateway ID"
// @Success 200 {object} models.ADBGateway
// @Security BearerAuth
// @Router /adb/gateways/{id} [get]
func getGatewayHandler(adbService *services.ADBService) fiber.Handler {
	return func(c *fiber.Ctx) error {
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
	}
}

// createGatewayHandler godoc
// @Summary Create ADB gateway
// @Description Create a new ADB gateway
// @Tags adb
// @Accept json
// @Produce json
// @Param request body CreateADBGatewayRequest true "Gateway data"
// @Success 201 {object} models.ADBGateway
// @Security BearerAuth
// @Router /adb/gateways [post]
func createGatewayHandler(adbService *services.ADBService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req CreateADBGatewayRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		// Validate based on gateway type
		if !req.IsDocker {
			if req.Host == "" || req.Port == 0 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Host and port are required for manual gateways",
				})
			}
		}

		gateway := &models.ADBGateway{
			Name:        req.Name,
			Host:        req.Host,
			Port:        req.Port,
			ServiceCode: req.ServiceCode,
			IsActive:    true,
			Status:      "offline",
			IsDocker:    false, // Always false for manual creation
		}

		if err := adbService.CreateGateway(gateway); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusCreated).JSON(gateway)
	}
}

// createDockerGatewayHandler godoc
// @Summary Create Docker ADB gateway
// @Description Create a new Docker-based ADB gateway with APK installation
// @Tags adb
// @Accept multipart/form-data
// @Produce json
// @Param name formData string true "Gateway name"
// @Param service_code formData string true "Service code (yandex_aon, kaspersky, getcontact)"
// @Param apk formData file false "APK file to install"
// @Success 201 {object} models.ADBGateway
// @Security BearerAuth
// @Router /adb/gateways/docker [post]
func createDockerGatewayHandler(adbService *services.ADBService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Parse form data
		name := c.FormValue("name")
		serviceCode := c.FormValue("service_code")

		if name == "" || serviceCode == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Name and service_code are required",
			})
		}

		// Validate service code
		validServices := map[string]bool{
			"yandex_aon": true,
			"kaspersky":  true,
			"getcontact": true,
		}

		if !validServices[serviceCode] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid service code",
			})
		}

		// Read APK file if provided
		var apkData []byte
		if file, err := c.FormFile("apk"); err == nil {
			src, err := file.Open()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to open APK file",
				})
			}
			defer src.Close()

			apkData, err = io.ReadAll(src)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to read APK file",
				})
			}
		}

		gateway := &models.ADBGateway{
			Name:        name,
			ServiceCode: serviceCode,
			IsActive:    true,
			Status:      "creating",
			IsDocker:    true,
		}

		if err := adbService.CreateDockerGateway(gateway, apkData); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusCreated).JSON(gateway)
	}
}

// updateGatewayHandler godoc
// @Summary Update ADB gateway
// @Description Update ADB gateway
// @Tags adb
// @Accept json
// @Produce json
// @Param id path int true "Gateway ID"
// @Param request body UpdateADBGatewayRequest true "Gateway update data"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /adb/gateways/{id} [put]
func updateGatewayHandler(adbService *services.ADBService) fiber.Handler {
	return func(c *fiber.Ctx) error {
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

		return c.JSON(MessageResponse{
			Message: "Gateway updated successfully",
		})
	}
}

// deleteGatewayHandler godoc
// @Summary Delete ADB gateway
// @Description Delete ADB gateway
// @Tags adb
// @Accept json
// @Produce json
// @Param id path int true "Gateway ID"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /adb/gateways/{id} [delete]
func deleteGatewayHandler(adbService *services.ADBService) fiber.Handler {
	return func(c *fiber.Ctx) error {
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

		return c.JSON(MessageResponse{
			Message: "Gateway deleted successfully",
		})
	}
}

// updateGatewayStatusHandler godoc
// @Summary Update gateway status
// @Description Update ADB gateway status
// @Tags adb
// @Accept json
// @Produce json
// @Param id path int true "Gateway ID"
// @Success 200 {object} GatewayStatusResponse
// @Security BearerAuth
// @Router /adb/gateways/{id}/status [post]
func updateGatewayStatusHandler(adbService *services.ADBService) fiber.Handler {
	return func(c *fiber.Ctx) error {
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

		return c.JSON(GatewayStatusResponse{
			Message: "Gateway status updated",
			Status:  gateway.Status,
		})
	}
}

// updateAllGatewayStatusesHandler godoc
// @Summary Update all gateway statuses
// @Description Update status for all ADB gateways
// @Tags adb
// @Accept json
// @Produce json
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /adb/gateways/status [post]
func updateAllGatewayStatusesHandler(adbService *services.ADBService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := adbService.UpdateAllGatewayStatuses(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update gateway statuses",
			})
		}

		return c.JSON(MessageResponse{
			Message: "All gateway statuses updated",
		})
	}
}

// getDeviceInfoHandler godoc
// @Summary Get device info
// @Description Get Android device information
// @Tags adb
// @Accept json
// @Produce json
// @Param id path int true "Gateway ID"
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /adb/gateways/{id}/device-info [get]
func getDeviceInfoHandler(adbService *services.ADBService) fiber.Handler {
	return func(c *fiber.Ctx) error {
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
	}
}

// executeCommandHandler godoc
// @Summary Execute ADB command
// @Description Execute custom ADB command on gateway
// @Tags adb
// @Accept json
// @Produce json
// @Param id path int true "Gateway ID"
// @Param command body ExecuteCommandRequest true "ADB command"
// @Success 200 {object} CommandOutputResponse
// @Security BearerAuth
// @Router /adb/gateways/{id}/execute [post]
func executeCommandHandler(adbService *services.ADBService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid gateway ID",
			})
		}

		var req ExecuteCommandRequest
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

		return c.JSON(CommandOutputResponse{
			Output: output,
		})
	}
}

// restartDeviceHandler godoc
// @Summary Restart device
// @Description Restart Android device
// @Tags adb
// @Accept json
// @Produce json
// @Param id path int true "Gateway ID"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /adb/gateways/{id}/restart [post]
func restartDeviceHandler(adbService *services.ADBService) fiber.Handler {
	return func(c *fiber.Ctx) error {
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

		return c.JSON(MessageResponse{
			Message: "Device restart initiated",
		})
	}
}

// installAPKHandler godoc
// @Summary Install APK
// @Description Install APK on Android device
// @Tags adb
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "Gateway ID"
// @Param apk formData file true "APK file"
// @Success 200 {object} MessageResponse
// @Security BearerAuth
// @Router /adb/gateways/{id}/install-apk [post]
func installAPKHandler(adbService *services.ADBService) fiber.Handler {
	return func(c *fiber.Ctx) error {
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

		return c.JSON(MessageResponse{
			Message: "APK installed successfully",
		})
	}
}

// checkDockerStatusHandler godoc
// @Summary Check Docker status
// @Description Check if Docker daemon is accessible
// @Tags adb
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /adb/docker/status [get]
func checkDockerStatusHandler(adbService *services.ADBService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := adbService.CheckDockerConnection()
		status := "connected"
		message := "Docker daemon is accessible"

		if err != nil {
			status = "disconnected"
			message = err.Error()
		}

		return c.JSON(fiber.Map{
			"status":  status,
			"message": message,
		})
	}
}

// listDockerContainersHandler godoc
// @Summary List Docker containers
// @Description List all Docker containers
// @Tags adb
// @Accept json
// @Produce json
// @Success 200 {object} []map[string]interface{}
// @Security BearerAuth
// @Router /adb/docker/containers [get]
func listDockerContainersHandler(adbService *services.ADBService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		containers, err := adbService.ListDockerContainers()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Transform to simplified format
		result := make([]map[string]interface{}, len(containers))
		for i, container := range containers {
			result[i] = map[string]interface{}{
				"id":     container.ID[:12],
				"names":  container.Names,
				"image":  container.Image,
				"state":  container.State,
				"status": container.Status,
				"ports":  container.Ports,
			}
		}

		return c.JSON(result)
	}
}
