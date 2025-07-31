package handlers

import (
	"spam-checker/internal/middleware"
	"spam-checker/internal/models"
	"spam-checker/internal/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type UpdateUserRequest struct {
	Username string          `json:"username"`
	Email    string          `json:"email"`
	Password string          `json:"password"`
	Role     models.UserRole `json:"role"`
	IsActive *bool           `json:"is_active"`
}

type CreateUserRequest struct {
	Username string          `json:"username" validate:"required,min=3,max=50"`
	Email    string          `json:"email" validate:"required,email"`
	Password string          `json:"password" validate:"required,min=6"`
	Role     models.UserRole `json:"role" validate:"required,oneof=admin supervisor user"`
}

type ChangePasswordRequest struct {
	Password    string `json:"password" validate:"required,min=6"`
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}

// RegisterUserRoutes registers user management routes
func RegisterUserRoutes(api fiber.Router, userService *services.UserService, authMiddleware *middleware.AuthMiddleware) {
	users := api.Group("/users")

	// @Summary List users
	// @Description Get list of users with pagination
	// @Tags users
	// @Accept json
	// @Produce json
	// @Param page query int false "Page number" default(1)
	// @Param limit query int false "Items per page" default(20)
	// @Param role query string false "Filter by role"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /users [get]
	users.Get("/", authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor), func(c *fiber.Ctx) error {
		page, _ := strconv.Atoi(c.Query("page", "1"))
		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		role := c.Query("role")

		offset := (page - 1) * limit
		users, total, err := userService.ListUsers(offset, limit, role)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get users",
			})
		}

		return c.JSON(fiber.Map{
			"users": users,
			"total": total,
			"page":  page,
			"limit": limit,
		})
	})

	// @Summary Get user
	// @Description Get user by ID
	// @Tags users
	// @Accept json
	// @Produce json
	// @Param id path int true "User ID"
	// @Success 200 {object} models.User
	// @Security BearerAuth
	// @Router /users/{id} [get]
	users.Get("/:id", authMiddleware.RequireRole(models.RoleAdmin, models.RoleSupervisor), func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid user ID",
			})
		}

		user, err := userService.GetUserByID(uint(id))
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(user)
	})

	// @Summary Create user
	// @Description Create a new user (admin only)
	// @Tags users
	// @Accept json
	// @Produce json
	// @Param request body CreateUserRequest true "User data"
	// @Success 201 {object} models.User
	// @Security BearerAuth
	// @Router /users [post]
	users.Post("/", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		var req CreateUserRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		user := &models.User{
			Username: req.Username,
			Email:    req.Email,
			Password: req.Password,
			Role:     req.Role,
			IsActive: true,
		}

		if err := userService.CreateUser(user); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusCreated).JSON(user)
	})

	// @Summary Update user
	// @Description Update user information
	// @Tags users
	// @Accept json
	// @Produce json
	// @Param id path int true "User ID"
	// @Param request body UpdateUserRequest true "User update data"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /users/{id} [put]
	users.Put("/:id", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid user ID",
			})
		}

		var req UpdateUserRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		// Can't change own role
		currentUserID := middleware.GetUserID(c)
		if uint(id) == currentUserID && req.Role != "" {
			currentUser, _ := userService.GetUserByID(currentUserID)
			if currentUser != nil && currentUser.Role != req.Role {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "Cannot change your own role",
				})
			}
		}

		updates := make(map[string]interface{})
		if req.Username != "" {
			updates["username"] = req.Username
		}
		if req.Email != "" {
			updates["email"] = req.Email
		}
		if req.Password != "" {
			updates["password"] = req.Password
		}
		if req.Role != "" {
			updates["role"] = req.Role
		}
		if req.IsActive != nil {
			updates["is_active"] = *req.IsActive
		}

		if err := userService.UpdateUser(uint(id), updates); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"message": "User updated successfully",
		})
	})

	// @Summary Delete user
	// @Description Delete user (admin only)
	// @Tags users
	// @Accept json
	// @Produce json
	// @Param id path int true "User ID"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /users/{id} [delete]
	users.Delete("/:id", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid user ID",
			})
		}

		// Can't delete yourself
		currentUserID := middleware.GetUserID(c)
		if uint(id) == currentUserID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Cannot delete your own account",
			})
		}

		if err := userService.DeleteUser(uint(id)); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to delete user",
			})
		}

		return c.JSON(fiber.Map{
			"message": "User deleted successfully",
		})
	})

	// @Summary Change user password
	// @Description Change user password (admin only)
	// @Tags users
	// @Accept json
	// @Produce json
	// @Param id path int true "User ID"
	// @Param request body ChangePasswordRequest true "New password"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /users/{id}/password [put]
	users.Put("/:id/password", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		id, err := strconv.ParseUint(c.Params("id"), 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid user ID",
			})
		}

		var req ChangePasswordRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		if err := userService.UpdateUser(uint(id), map[string]interface{}{
			"password": req.Password,
		}); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to change password",
			})
		}

		return c.JSON(fiber.Map{
			"message": "Password changed successfully",
		})
	})

	// @Summary Get current user
	// @Description Get current authenticated user
	// @Tags users
	// @Accept json
	// @Produce json
	// @Success 200 {object} models.User
	// @Security BearerAuth
	// @Router /users/me [get]
	users.Get("/me", func(c *fiber.Ctx) error {
		userID := middleware.GetUserID(c)
		user, err := userService.GetUserByID(userID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found",
			})
		}

		return c.JSON(user)
	})

	// @Summary Update current user
	// @Description Update current user profile
	// @Tags users
	// @Accept json
	// @Produce json
	// @Param request body UpdateProfileRequest true "Profile data"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /users/me [put]
	users.Put("/me", func(c *fiber.Ctx) error {
		userID := middleware.GetUserID(c)

		var req struct {
			Username string `json:"username"`
			Email    string `json:"email"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		updates := make(map[string]interface{})
		if req.Username != "" {
			updates["username"] = req.Username
		}
		if req.Email != "" {
			updates["email"] = req.Email
		}

		if err := userService.UpdateUser(userID, updates); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"message": "Profile updated successfully",
		})
	})

	// @Summary Change my password
	// @Description Change current user password
	// @Tags users
	// @Accept json
	// @Produce json
	// @Param request body ChangeMyPasswordRequest true "Password data"
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /users/me/password [put]
	users.Put("/me/password", func(c *fiber.Ctx) error {
		userID := middleware.GetUserID(c)

		var req struct {
			OldPassword string `json:"old_password" validate:"required"`
			NewPassword string `json:"new_password" validate:"required,min=6"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		if err := userService.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"message": "Password changed successfully",
		})
	})

	// @Summary Get user statistics
	// @Description Get user statistics (admin only)
	// @Tags users
	// @Accept json
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Security BearerAuth
	// @Router /users/stats [get]
	users.Get("/stats", authMiddleware.RequireRole(models.RoleAdmin), func(c *fiber.Ctx) error {
		stats, err := userService.GetUserStats()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get user statistics",
			})
		}

		return c.JSON(stats)
	})
}
