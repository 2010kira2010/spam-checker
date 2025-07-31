package models

import (
	"gorm.io/gorm"
	"time"
)

// User represents system user
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"unique;not null" json:"username"`
	Email     string         `gorm:"unique;not null" json:"email"`
	Password  string         `gorm:"not null" json:"-"`
	Role      UserRole       `gorm:"not null" json:"role"`
	IsActive  bool           `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// UserRole represents user role in system
type UserRole string

const (
	RoleAdmin      UserRole = "admin"
	RoleSupervisor UserRole = "supervisor"
	RoleUser       UserRole = "user"
)

// PhoneNumber represents company phone number
type PhoneNumber struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Number       string         `gorm:"unique;not null" json:"number"`
	Description  string         `json:"description"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	CreatedBy    uint           `json:"created_by"`
	User         User           `gorm:"foreignKey:CreatedBy" json:"-"`
	CheckResults []CheckResult  `json:"check_results,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// SpamService represents spam check service
type SpamService struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"unique;not null" json:"name"`
	Code      string    `gorm:"unique;not null" json:"code"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CheckResult represents spam check result
type CheckResult struct {
	ID            uint        `gorm:"primaryKey" json:"id"`
	PhoneNumberID uint        `json:"phone_number_id"`
	PhoneNumber   PhoneNumber `gorm:"foreignKey:PhoneNumberID" json:"-"`
	ServiceID     uint        `json:"service_id"`
	Service       SpamService `gorm:"foreignKey:ServiceID" json:"service"`
	IsSpam        bool        `json:"is_spam"`
	FoundKeywords []string    `gorm:"type:text[]" json:"found_keywords"`
	Screenshot    string      `json:"screenshot"`
	RawText       string      `json:"raw_text"`
	CheckedAt     time.Time   `json:"checked_at"`
	CreatedAt     time.Time   `json:"created_at"`
}

// ADBGateway represents Android Debug Bridge gateway
type ADBGateway struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"unique;not null" json:"name"`
	Host        string     `gorm:"not null" json:"host"`
	Port        int        `gorm:"not null" json:"port"`
	DeviceID    string     `json:"device_id"`
	ServiceCode string     `json:"service_code"`
	IsActive    bool       `gorm:"default:true" json:"is_active"`
	Status      string     `gorm:"default:offline" json:"status"`
	LastPing    *time.Time `json:"last_ping"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// SystemSettings represents system configuration
type SystemSettings struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"unique;not null" json:"key"`
	Value     string    `json:"value"`
	Type      string    `json:"type"` // string, int, bool, json
	Category  string    `json:"category"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Notification represents notification configuration
type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Type      string    `gorm:"not null" json:"type"` // telegram, email
	Config    string    `gorm:"type:jsonb" json:"config"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CheckSchedule represents check schedule configuration
type CheckSchedule struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Name           string     `gorm:"not null" json:"name"`
	CronExpression string     `gorm:"not null" json:"cron_expression"`
	IsActive       bool       `gorm:"default:true" json:"is_active"`
	LastRun        *time.Time `json:"last_run"`
	NextRun        *time.Time `json:"next_run"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// SpamKeyword represents keywords for spam detection
type SpamKeyword struct {
	ID        uint         `gorm:"primaryKey" json:"id"`
	Keyword   string       `gorm:"not null" json:"keyword"`
	ServiceID *uint        `json:"service_id,omitempty"`
	Service   *SpamService `gorm:"foreignKey:ServiceID" json:"service,omitempty"`
	IsActive  bool         `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// Statistics represents check statistics
type Statistics struct {
	ID            uint        `gorm:"primaryKey" json:"id"`
	PhoneNumberID uint        `json:"phone_number_id"`
	PhoneNumber   PhoneNumber `gorm:"foreignKey:PhoneNumberID" json:"-"`
	ServiceID     uint        `json:"service_id"`
	Service       SpamService `gorm:"foreignKey:ServiceID" json:"service"`
	FirstSpamDate *time.Time  `json:"first_spam_date"`
	TotalChecks   int         `json:"total_checks"`
	SpamCount     int         `json:"spam_count"`
	LastCheckDate time.Time   `json:"last_check_date"`
	UpdatedAt     time.Time   `json:"updated_at"`
}
