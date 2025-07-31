package middleware

import (
	"spam-checker/internal/config"
	"spam-checker/internal/models"
	"spam-checker/internal/utils"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type AuthMiddleware struct {
	jwtManager *utils.JWTManager
}

func NewAuthMiddleware(cfg config.JWTConfig) *AuthMiddleware {
	return &AuthMiddleware{
		jwtManager: utils.NewJWTManager(cfg),
	}
}

// Protect validates JWT token
func (m *AuthMiddleware) Protect() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing authorization header",
			})
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid authorization header format",
			})
		}

		token := parts[1]
		claims, err := m.jwtManager.ValidateToken(token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or expired token",
			})
		}

		// Store user info in context
		c.Locals("userID", claims.UserID)
		c.Locals("username", claims.Username)
		c.Locals("email", claims.Email)
		c.Locals("role", claims.Role)

		return c.Next()
	}
}

// RequireRole checks if user has required role
func (m *AuthMiddleware) RequireRole(roles ...models.UserRole) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole, ok := c.Locals("role").(models.UserRole)
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Access denied",
			})
		}

		// Check if user has any of the required roles
		for _, role := range roles {
			if userRole == role {
				return c.Next()
			}
		}

		// Special case: admin has access to everything
		if userRole == models.RoleAdmin {
			return c.Next()
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Insufficient permissions",
		})
	}
}

// GetUserID extracts user ID from context
func GetUserID(c *fiber.Ctx) uint {
	userID, _ := c.Locals("userID").(uint)
	return userID
}

// GetUserRole extracts user role from context
func GetUserRole(c *fiber.Ctx) models.UserRole {
	role, _ := c.Locals("role").(models.UserRole)
	return role
}

// GetUsername extracts username from context
func GetUsername(c *fiber.Ctx) string {
	username, _ := c.Locals("username").(string)
	return username
}

// GetEmail extracts email from context
func GetEmail(c *fiber.Ctx) string {
	email, _ := c.Locals("email").(string)
	return email
}
