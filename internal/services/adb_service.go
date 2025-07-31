package services

import (
	"fmt"
	"os/exec"
	"spam-checker/internal/models"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ADBService struct {
	db *gorm.DB
}

func NewADBService(db *gorm.DB) *ADBService {
	return &ADBService{db: db}
}

// CreateGateway creates a new ADB gateway
func (s *ADBService) CreateGateway(gateway *models.ADBGateway) error {
	if err := s.db.Create(gateway).Error; err != nil {
		return fmt.Errorf("failed to create gateway: %w", err)
	}

	// Test connection
	go s.UpdateGatewayStatus(gateway.ID)

	return nil
}

// GetGatewayByID gets gateway by ID
func (s *ADBService) GetGatewayByID(id uint) (*models.ADBGateway, error) {
	var gateway models.ADBGateway
	if err := s.db.First(&gateway, id).Error; err != nil {
		return nil, fmt.Errorf("gateway not found: %w", err)
	}
	return &gateway, nil
}

// ListGateways lists all gateways
func (s *ADBService) ListGateways() ([]models.ADBGateway, error) {
	var gateways []models.ADBGateway
	if err := s.db.Find(&gateways).Error; err != nil {
		return nil, fmt.Errorf("failed to list gateways: %w", err)
	}
	return gateways, nil
}

// GetActiveGateways gets all active gateways
func (s *ADBService) GetActiveGateways() ([]models.ADBGateway, error) {
	var gateways []models.ADBGateway
	if err := s.db.Where("is_active = ? AND status = ?", true, "online").Find(&gateways).Error; err != nil {
		return nil, fmt.Errorf("failed to get active gateways: %w", err)
	}
	return gateways, nil
}

// UpdateGateway updates gateway information
func (s *ADBService) UpdateGateway(id uint, updates map[string]interface{}) error {
	if err := s.db.Model(&models.ADBGateway{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update gateway: %w", err)
	}

	// Test connection after update
	go s.UpdateGatewayStatus(id)

	return nil
}

// DeleteGateway deletes a gateway
func (s *ADBService) DeleteGateway(id uint) error {
	if err := s.db.Delete(&models.ADBGateway{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete gateway: %w", err)
	}
	return nil
}

// UpdateGatewayStatus checks and updates gateway status
func (s *ADBService) UpdateGatewayStatus(gatewayID uint) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	// Test ADB connection
	status := "offline"
	deviceID := ""

	// Connect to ADB
	connectCmd := fmt.Sprintf("adb connect %s:%d", gateway.Host, gateway.Port)
	output, err := exec.Command("sh", "-c", connectCmd).Output()
	if err == nil && strings.Contains(string(output), "connected") {
		// Get device ID
		devicesCmd := fmt.Sprintf("adb devices | grep %s:%d", gateway.Host, gateway.Port)
		devOutput, err := exec.Command("sh", "-c", devicesCmd).Output()
		if err == nil {
			parts := strings.Fields(string(devOutput))
			if len(parts) >= 2 && parts[1] == "device" {
				status = "online"
				deviceID = parts[0]
			}
		}
	}

	// Update status
	now := time.Now()
	updates := map[string]interface{}{
		"status":    status,
		"device_id": deviceID,
		"last_ping": &now,
	}

	if err := s.db.Model(gateway).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update gateway status: %w", err)
	}

	logrus.Infof("Gateway %s status updated: %s", gateway.Name, status)

	return nil
}

// UpdateAllGatewayStatuses updates status for all gateways
func (s *ADBService) UpdateAllGatewayStatuses() error {
	gateways, err := s.ListGateways()
	if err != nil {
		return err
	}

	for _, gateway := range gateways {
		if err := s.UpdateGatewayStatus(gateway.ID); err != nil {
			logrus.Errorf("Failed to update gateway %s status: %v", gateway.Name, err)
		}
	}

	return nil
}

// InstallAPK installs APK on gateway
func (s *ADBService) InstallAPK(gatewayID uint, apkPath string) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	// Check if gateway is online
	if gateway.Status != "online" {
		return fmt.Errorf("gateway %s is not online", gateway.Name)
	}

	// Install APK
	installCmd := fmt.Sprintf("adb -s %s install -r %s", gateway.DeviceID, apkPath)
	output, err := exec.Command("sh", "-c", installCmd).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install APK: %v - %s", err, string(output))
	}

	if !strings.Contains(string(output), "Success") {
		return fmt.Errorf("APK installation failed: %s", string(output))
	}

	logrus.Infof("APK installed successfully on gateway %s", gateway.Name)

	return nil
}

// ExecuteCommand executes ADB command on gateway
func (s *ADBService) ExecuteCommand(gatewayID uint, command string) (string, error) {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return "", err
	}

	// Check if gateway is online
	if gateway.Status != "online" {
		return "", fmt.Errorf("gateway %s is not online", gateway.Name)
	}

	// Execute command
	adbCmd := fmt.Sprintf("adb -s %s %s", gateway.DeviceID, command)
	output, err := exec.Command("sh", "-c", adbCmd).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %v - %s", err, string(output))
	}

	return string(output), nil
}

// GetDeviceInfo gets device information
func (s *ADBService) GetDeviceInfo(gatewayID uint) (map[string]string, error) {
	info := make(map[string]string)

	// Get basic props
	props := []string{
		"ro.product.model",
		"ro.product.manufacturer",
		"ro.build.version.release",
		"ro.build.version.sdk",
	}

	for _, prop := range props {
		output, err := s.ExecuteCommand(gatewayID, fmt.Sprintf("shell getprop %s", prop))
		if err == nil {
			info[prop] = strings.TrimSpace(output)
		}
	}

	// Get battery info
	batteryOutput, err := s.ExecuteCommand(gatewayID, "shell dumpsys battery")
	if err == nil {
		lines := strings.Split(batteryOutput, "\n")
		for _, line := range lines {
			if strings.Contains(line, "level:") {
				parts := strings.Split(line, ":")
				if len(parts) == 2 {
					info["battery_level"] = strings.TrimSpace(parts[1])
				}
			}
		}
	}

	// Get screen resolution
	wmOutput, err := s.ExecuteCommand(gatewayID, "shell wm size")
	if err == nil {
		if strings.Contains(wmOutput, "Physical size:") {
			parts := strings.Split(wmOutput, ":")
			if len(parts) == 2 {
				info["screen_size"] = strings.TrimSpace(parts[1])
			}
		}
	}

	return info, nil
}

// RestartDevice restarts Android device
func (s *ADBService) RestartDevice(gatewayID uint) error {
	_, err := s.ExecuteCommand(gatewayID, "reboot")
	if err != nil {
		return fmt.Errorf("failed to restart device: %w", err)
	}

	// Update status to offline (will be updated when device comes back online)
	s.db.Model(&models.ADBGateway{}).Where("id = ?", gatewayID).Update("status", "restarting")

	return nil
}

// ClearAppData clears app data for service
func (s *ADBService) ClearAppData(gatewayID uint, serviceCode string) error {
	// Get app package based on service
	var appPackage string
	switch serviceCode {
	case "yandex_aon":
		appPackage = "ru.yandex.whocalls"
	case "kaspersky":
		appPackage = "com.kaspersky.whocalls"
	case "getcontact":
		appPackage = "app.source.getcontact"
	default:
		return fmt.Errorf("unknown service code: %s", serviceCode)
	}

	// Clear app data
	_, err := s.ExecuteCommand(gatewayID, fmt.Sprintf("shell pm clear %s", appPackage))
	if err != nil {
		return fmt.Errorf("failed to clear app data: %w", err)
	}

	logrus.Infof("App data cleared for %s on gateway %d", appPackage, gatewayID)

	return nil
}

// MonitorGateways continuously monitors gateway statuses
func (s *ADBService) MonitorGateways(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.UpdateAllGatewayStatuses(); err != nil {
				logrus.Errorf("Failed to update gateway statuses: %v", err)
			}
		}
	}
}
