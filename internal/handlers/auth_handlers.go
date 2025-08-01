package handlers

import (
	"spam-checker/internal/config"
	"spam-checker/internal/models"
	"spam-checker/internal/services"
	"spam-checker/internal/utils"

	"github.com/gofiber/fiber/v2"
)

// LoginRequest represents login request
type LoginRequest struct {
	Login    string `json:"login" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// RegisterRequest represents registration request
type RegisterRequest struct {
	Username string          `json:"username" validate:"required,min=3,max=50"`
	Email    string          `json:"email" validate:"required,email"`
	Password string          `json:"password" validate:"required,min=6"`
	Role     models.UserRole `json:"role" validate:"required,oneof=admin supervisor user"`
}

// RefreshTokenRequest represents refresh token request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LoginResponse represents login response
type LoginResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	User         UserInfo `json:"user"`
}

// UserInfo represents user information in responses
type UserInfo struct {
	ID       uint            `json:"id"`
	Username string          `json:"username"`
	Email    string          `json:"email"`
	Role     models.UserRole `json:"role"`
}

// RegisterResponse represents registration response
type RegisterResponse struct {
	Message string   `json:"message"`
	User    UserInfo `json:"user"`
}

// RefreshTokenResponse represents refresh token response
type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
}

// RegisterAuthRoutes registers authentication routes
func RegisterAuthRoutes(api fiber.Router, userService *services.UserService, jwtConfig config.JWTConfig) {
	auth := api.Group("/auth")
	jwtManager := utils.NewJWTManager(jwtConfig)

	auth.Post("/login", loginHandler(userService, jwtManager))
	auth.Post("/register", registerHandler(userService))
	auth.Post("/refresh", refreshTokenHandler(userService, jwtManager))
}

// loginHandler godoc
// @Summary Login
// @Description Authenticate user and get access token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /auth/login [post]
func loginHandler(userService *services.UserService, jwtManager *utils.JWTManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req LoginRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		// Authenticate user
		user, err := userService.AuthenticateUser(req.Login, req.Password)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Generate tokens
		accessToken, err := jwtManager.GenerateToken(user)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to generate access token",
			})
		}

		refreshToken, err := jwtManager.GenerateRefreshToken(user)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to generate refresh token",
			})
		}

		return c.JSON(LoginResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			User: UserInfo{
				ID:       user.ID,
				Username: user.Username,
				Email:    user.Email,
				Role:     user.Role,
			},
		})
	}
}

// registerHandler godoc
// @Summary Register
// @Description Register a new user (admin only)
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "User registration data"
// @Success 201 {object} RegisterResponse
// @Failure 400 {object} map[string]interface{}
// @Router /auth/register [post]
func registerHandler(userService *services.UserService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req RegisterRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		// Create user
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

		return c.Status(fiber.StatusCreated).JSON(RegisterResponse{
			Message: "User created successfully",
			User: UserInfo{
				ID:       user.ID,
				Username: user.Username,
				Email:    user.Email,
				Role:     user.Role,
			},
		})
	}
}

// refreshTokenHandler godoc
// @Summary Refresh Token
// @Description Get new access token using refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RefreshTokenRequest true "Refresh token"
// @Success 200 {object} RefreshTokenResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /auth/refresh [post]
func refreshTokenHandler(userService *services.UserService, jwtManager *utils.JWTManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req RefreshTokenRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		// Validate refresh token
		userID, err := jwtManager.ValidateRefreshToken(req.RefreshToken)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid refresh token",
			})
		}

		// Get user
		user, err := userService.GetUserByID(userID)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "User not found",
			})
		}

		// Check if user is active
		if !user.IsActive {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "User account is disabled",
			})
		}

		// Generate new access token
		accessToken, err := jwtManager.GenerateToken(user)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to generate access token",
			})
		}

		return c.JSON(RefreshTokenResponse{
			AccessToken: accessToken,
		})
	}
}
