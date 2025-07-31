package handlers

import (
	"spam-checker/internal/middleware"
	"spam-checker/internal/models"
	"spam-checker/internal/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type CreateNotificationRequest struct {
	Type   string `json:"type" validate:"required,oneof=telegram email"`
	Config string `json:"config" validate:"required"`
}

type UpdateNotificationRequest struct {
	Config   string `json:"config"`
	IsActive *bool  `json:"is_active"`
}

type TestNotificationRequest struct {
	Message string `json:"message"`
}

// RegisterNotificationRoutes registers notification routes
func RegisterNotificationRoutes(api fiber.Router, notificationService *services.NotificationService, authMiddleware *middleware.AuthMiddleware) {
	notifications := api.Group("/notifications")

	// All notification routes require admin or supervisor role
	notifications.Use(authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor))

	// @Summary List notifications
	// @Description Get all notification channels
	// @Tags notifications
	// @Accept json
	// @Produce json
	// @Success 200 {array} models.Notification
	// @Security BearerAuth
	// @Router /notifications [get]
	notifications.Get("/", func(c *fiber.Ctx) error {
		notifications, err := notificationService.GetNotifications()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get notifications",
			})
		}

		return c.JSON(notifications)
	})

	// @Summary Get notification
	// @Description Get notification channel by ID
	// @Tags notifications
	// @Accept json
	// @Produce json
	// @Param id path int true "Notification ID"
	// @Success 200 {object} models.Notification
	// @Security BearerAuth
	// @Router /notifications/{id} [get]
	notifications.Get("/:id", func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid notification ID",
			})
		}

		notification, err := notificationService.GetNotificationByID(uint(id))
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(notification)
	})

	// @Summary Create notification
	// @Description Create a new notification channel
	// @Tags notifications
	// @Accept json
	// @Produce json
	// @Param request body CreateNotificationRequest true "Notification data"
	// @Success 201 {object} models.Notification
	// @Security BearerAuth
	// @Router /notifications [post]
	notifications.Post("/", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		var req CreateNotificationRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		notification := &models.Notification{
			Type:     req.Type,
			Config:   req.Config,
			IsActive: true,
		}

		if err := notificationService.CreateNotification(notification); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusCreated).JSON(notification)
	})

	// @Summary Update notification
	// @Description Update notification channel
	// @Tags notifications
	// @Accept json
	// @Produce json
	// @Param id path int true "Notification ID"
	// @Param request body UpdateNotificationRequest true "Notification update data"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /notifications/{id} [put]
	notifications.Put("/:id", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid notification ID",
			})
		}

		var req UpdateNotificationRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		updates := make(map[string]interface{})
		if req.Config != "" {
			updates["config"] = req.Config
		}
		if req.IsActive != nil {
			updates["is_active"] = *req.IsActive
		}

		if err := notificationService.UpdateNotification(uint(id), updates); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"message": "Notification updated successfully",
		})
	})

	// @Summary Delete notification
	// @Description Delete notification channel
	// @Tags notifications
	// @Accept json
	// @Produce json
	// @Param id path int true "Notification ID"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /notifications/{id} [delete]
	notifications.Delete("/:id", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid notification ID",
			})
		}

		if err := notificationService.DeleteNotification(uint(id)); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to delete notification",
			})
		}

		return c.JSON(fiber.Map{
			"message": "Notification deleted successfully",
		})
	})

	// @Summary Test notification
	// @Description Test notification channel
	// @Tags notifications
	// @Accept json
	// @Produce json
	// @Param id path int true "Notification ID"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /notifications/{id}/test [post]
	notifications.Post("/:id/test", func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid notification ID",
			})
		}

		if err := notificationService.TestNotification(uint(id)); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"message": "Test notification sent successfully",
		})
	})

	// @Summary Send notification
	// @Description Send notification to all active channels
	// @Tags notifications
	// @Accept json
	// @Produce json
	// @Param request body TestNotificationRequest true "Notification message"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /notifications/send [post]
	notifications.Post("/send", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		var req TestNotificationRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		subject := "Manual Notification"
		message := req.Message
		if message == "" {
			message = "This is a test notification sent manually from SpamChecker"
		}

		if err := notificationService.SendNotification(subject, message); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"message": "Notification sent successfully",
		})
	})
}
