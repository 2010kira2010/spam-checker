package models

import (
	"database/sql/driver"
	"gorm.io/gorm"
	"strings"
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

// StringArray custom type for PostgreSQL text[] array
type StringArray []string

// Scan implements sql.Scanner interface for StringArray
func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = []string{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return a.scanBytes(v)
	case string:
		return a.scanBytes([]byte(v))
	case []string:
		*a = v
		return nil
	default:
		*a = []string{}
		return nil
	}
}

// scanBytes parses PostgreSQL array format
func (a *StringArray) scanBytes(src []byte) error {
	strValue := string(src)
	*a = []string{}

	// Handle empty array
	if strValue == "{}" || strValue == "" {
		return nil
	}

	// Remove curly braces
	strValue = strings.TrimPrefix(strValue, "{")
	strValue = strings.TrimSuffix(strValue, "}")

	// Handle empty content
	if strValue == "" {
		return nil
	}

	// Split by comma, handling quoted values
	elements := []string{}
	current := ""
	inQuotes := false
	escaped := false

	for _, char := range strValue {
		if escaped {
			current += string(char)
			escaped = false
			continue
		}

		switch char {
		case '\\':
			escaped = true
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				elements = append(elements, strings.Trim(current, `"`))
				current = ""
			} else {
				current += string(char)
			}
		default:
			current += string(char)
		}
	}

	// Add last element
	if current != "" {
		elements = append(elements, strings.Trim(current, `"`))
	}

	*a = elements
	return nil
}

// Value implements driver.Valuer interface for StringArray
func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}

	// Build PostgreSQL array format
	elements := make([]string, len(a))
	for i, v := range a {
		// Escape special characters
		v = strings.ReplaceAll(v, `\`, `\\`)
		v = strings.ReplaceAll(v, `"`, `\"`)
		// Quote if contains comma, space, or special chars
		if strings.ContainsAny(v, `, {}"\`) || v == "" {
			elements[i] = `"` + v + `"`
		} else {
			elements[i] = v
		}
	}

	return "{" + strings.Join(elements, ",") + "}", nil
}

// CheckResult represents spam check result
type CheckResult struct {
	ID            uint        `gorm:"primaryKey" json:"id"`
	PhoneNumberID uint        `json:"phone_number_id"`
	PhoneNumber   PhoneNumber `gorm:"foreignKey:PhoneNumberID" json:"-"`
	ServiceID     uint        `json:"service_id"`
	Service       SpamService `gorm:"foreignKey:ServiceID" json:"service"`
	IsSpam        bool        `json:"is_spam"`
	FoundKeywords StringArray `gorm:"type:text[]" json:"found_keywords"`
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
	IsDocker    bool       `gorm:"default:false" json:"is_docker"`
	ContainerID string     `json:"container_id"`
	VNCPort     int        `json:"vnc_port"`
	ADBPort1    int        `json:"adb_port1"`
	ADBPort2    int        `json:"adb_port2"`
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
