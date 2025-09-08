package services

import (
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"spam-checker/internal/logger"
	"spam-checker/internal/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserService struct {
	db  *gorm.DB
	log *logrus.Entry
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{
		db:  db,
		log: logger.WithField("service", "UserService"),
	}
}

// CreateUser creates a new user
func (s *UserService) CreateUser(user *models.User) error {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	user.Password = string(hashedPassword)

	// Create user
	if err := s.db.Create(user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errors.New("user with this email or username already exists")
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUserByID gets user by ID
func (s *UserService) GetUserByID(id uint) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// GetUserByEmail gets user by email
func (s *UserService) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// GetUserByUsername gets user by username
func (s *UserService) GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// AuthenticateUser authenticates user by email/username and password
func (s *UserService) AuthenticateUser(login, password string) (*models.User, error) {
	// Try to find user by email or username
	var user models.User
	if err := s.db.Where("email = ? OR username = ?", login, login).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid credentials")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if user is active
	if !user.IsActive {
		return nil, errors.New("user account is disabled")
	}

	// Compare password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	return &user, nil
}

// ListUsers lists all users with pagination
func (s *UserService) ListUsers(offset, limit int, role string) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	query := s.db.Model(&models.User{})

	if role != "" {
		query = query.Where("role = ?", role)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// Get users
	if err := query.Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	return users, total, nil
}

// UpdateUser updates user information
func (s *UserService) UpdateUser(id uint, updates map[string]interface{}) error {
	// If password is being updated, hash it
	if password, ok := updates["password"].(string); ok && password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		updates["password"] = string(hashedPassword)
	} else {
		// Remove password from updates if empty
		delete(updates, "password")
	}

	if err := s.db.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// DeleteUser soft deletes a user
func (s *UserService) DeleteUser(id uint) error {
	if err := s.db.Delete(&models.User{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

// ChangePassword changes user password
func (s *UserService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return err
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return errors.New("invalid old password")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	if err := s.db.Model(user).Update("password", string(hashedPassword)).Error; err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// GetUserStats gets user statistics
func (s *UserService) GetUserStats() (map[string]interface{}, error) {
	var totalUsers int64
	var activeUsers int64
	var roleStats []struct {
		Role  string `json:"role"`
		Count int64  `json:"count"`
	}

	// Total users
	if err := s.db.Model(&models.User{}).Count(&totalUsers).Error; err != nil {
		return nil, fmt.Errorf("failed to count total users: %w", err)
	}

	// Active users
	if err := s.db.Model(&models.User{}).Where("is_active = ?", true).Count(&activeUsers).Error; err != nil {
		return nil, fmt.Errorf("failed to count active users: %w", err)
	}

	// Users by role
	if err := s.db.Model(&models.User{}).
		Select("role, COUNT(*) as count").
		Group("role").
		Scan(&roleStats).Error; err != nil {
		return nil, fmt.Errorf("failed to get role statistics: %w", err)
	}

	return map[string]interface{}{
		"total_users":  totalUsers,
		"active_users": activeUsers,
		"by_role":      roleStats,
	}, nil
}
