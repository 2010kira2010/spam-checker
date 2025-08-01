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
	return &ADBService{
		db: db,
	}
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

	status := "offline"
	deviceID := ""

	// Format device address
	deviceAddr := s.getDeviceAddress(gateway)

	// Try to connect
	cmd := exec.Command("adb", "connect", deviceAddr)
	output, err := cmd.CombinedOutput()
	if err == nil && strings.Contains(string(output), "connected") {
		// Check device state
		cmd = exec.Command("adb", "-s", deviceAddr, "get-state")
		stateOutput, err := cmd.CombinedOutput()
		if err == nil && strings.TrimSpace(string(stateOutput)) == "device" {
			status = "online"
			deviceID = deviceAddr
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

	if gateway.Status != "online" {
		return fmt.Errorf("gateway %s is not online", gateway.Name)
	}

	deviceAddr := s.getDeviceAddress(gateway)

	// Install APK
	cmd := exec.Command("adb", "-s", deviceAddr, "install", "-r", apkPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install APK: %w, output: %s", err, string(output))
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

	if gateway.Status != "online" {
		return "", fmt.Errorf("gateway %s is not online", gateway.Name)
	}

	deviceAddr := s.getDeviceAddress(gateway)

	// Execute shell command
	args := append([]string{"-s", deviceAddr, "shell"}, strings.Fields(command)...)
	cmd := exec.Command("adb", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

// GetDeviceInfo gets device information
func (s *ADBService) GetDeviceInfo(gatewayID uint) (map[string]string, error) {
	info := make(map[string]string)

	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return nil, err
	}

	deviceAddr := s.getDeviceAddress(gateway)

	// Get device state
	cmd := exec.Command("adb", "-s", deviceAddr, "get-state")
	output, err := cmd.CombinedOutput()
	if err == nil {
		info["state"] = strings.TrimSpace(string(output))
	}

	// Get device properties
	props := map[string]string{
		"android_version": "ro.build.version.release",
		"sdk_version":     "ro.build.version.sdk",
		"manufacturer":    "ro.product.manufacturer",
		"model":           "ro.product.model",
		"device":          "ro.product.device",
		"brand":           "ro.product.brand",
	}

	for key, prop := range props {
		cmd = exec.Command("adb", "-s", deviceAddr, "shell", "getprop", prop)
		output, err = cmd.CombinedOutput()
		if err == nil {
			info[key] = strings.TrimSpace(string(output))
		}
	}

	// Get battery info
	cmd = exec.Command("adb", "-s", deviceAddr, "shell", "dumpsys", "battery")
	output, err = cmd.CombinedOutput()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "level:") {
				info["battery_level"] = strings.TrimSpace(strings.TrimPrefix(line, "level:"))
			} else if strings.HasPrefix(line, "temperature:") {
				temp := strings.TrimSpace(strings.TrimPrefix(line, "temperature:"))
				if tempInt := strings.TrimSpace(temp); tempInt != "" {
					// Battery temperature is in tenths of degrees Celsius
					info["battery_temperature"] = tempInt
				}
			}
		}
	}

	// Get screen resolution
	cmd = exec.Command("adb", "-s", deviceAddr, "shell", "wm", "size")
	output, err = cmd.CombinedOutput()
	if err == nil {
		outputStr := string(output)
		if idx := strings.Index(outputStr, "Physical size:"); idx != -1 {
			size := strings.TrimSpace(outputStr[idx+14:])
			if endIdx := strings.Index(size, "\n"); endIdx != -1 {
				size = size[:endIdx]
			}
			info["screen_size"] = size
		}
	}

	return info, nil
}

// RestartDevice restarts Android device
func (s *ADBService) RestartDevice(gatewayID uint) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	deviceAddr := s.getDeviceAddress(gateway)

	// Reboot device
	cmd := exec.Command("adb", "-s", deviceAddr, "reboot")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to restart device: %w", err)
	}

	// Update status to restarting
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

	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	deviceAddr := s.getDeviceAddress(gateway)

	// Clear app data
	cmd := exec.Command("adb", "-s", deviceAddr, "shell", "pm", "clear", appPackage)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clear app data: %w, output: %s", err, string(output))
	}

	if !strings.Contains(string(output), "Success") {
		return fmt.Errorf("failed to clear app data: %s", string(output))
	}

	logrus.Infof("App data cleared for %s on gateway %d", appPackage, gatewayID)

	return nil
}

// TakeScreenshot takes a screenshot from device
func (s *ADBService) TakeScreenshot(gatewayID uint) ([]byte, error) {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return nil, err
	}

	deviceAddr := s.getDeviceAddress(gateway)

	// Take screenshot to stdout
	cmd := exec.Command("adb", "-s", deviceAddr, "exec-out", "screencap", "-p")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	return output, nil
}

// InputText inputs text on device
func (s *ADBService) InputText(gatewayID uint, text string) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	deviceAddr := s.getDeviceAddress(gateway)

	// Escape special characters for shell
	text = strings.ReplaceAll(text, "'", "'\"'\"'")

	// Input text
	cmd := exec.Command("adb", "-s", deviceAddr, "shell", "input", "text", "'"+text+"'")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to input text: %w", err)
	}

	return nil
}

// SendKeyEvent sends key event to device
func (s *ADBService) SendKeyEvent(gatewayID uint, keyCode string) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	deviceAddr := s.getDeviceAddress(gateway)

	// Send key event
	cmd := exec.Command("adb", "-s", deviceAddr, "shell", "input", "keyevent", keyCode)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to send key event: %w", err)
	}

	return nil
}

// StartApp starts app on device
func (s *ADBService) StartApp(gatewayID uint, packageName, activityName string) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	deviceAddr := s.getDeviceAddress(gateway)

	// Start app
	cmd := exec.Command("adb", "-s", deviceAddr, "shell", "am", "start", "-n", packageName+"/"+activityName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start app: %w, output: %s", err, string(output))
	}

	return nil
}

// CloseAllConnections closes all ADB connections
func (s *ADBService) CloseAllConnections() {
	// Disconnect all devices
	cmd := exec.Command("adb", "disconnect")
	cmd.Run()
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

// getDeviceAddress returns formatted device address
func (s *ADBService) getDeviceAddress(gateway *models.ADBGateway) string {
	return fmt.Sprintf("%s:%d", gateway.Host, gateway.Port)
}

// CheckADBInstalled checks if ADB is installed
func (s *ADBService) CheckADBInstalled() error {
	cmd := exec.Command("adb", "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ADB not found: %w", err)
	}

	logrus.Infof("ADB version: %s", string(output))
	return nil
}

// StartADBServer starts ADB server
func (s *ADBService) StartADBServer() error {
	cmd := exec.Command("adb", "start-server")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to start ADB server: %w", err)
	}

	logrus.Info("ADB server started")
	return nil
}

// StopADBServer stops ADB server
func (s *ADBService) StopADBServer() error {
	cmd := exec.Command("adb", "kill-server")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to stop ADB server: %w", err)
	}

	logrus.Info("ADB server stopped")
	return nil
}

// ListConnectedDevices lists all connected devices
func (s *ADBService) ListConnectedDevices() ([]string, error) {
	cmd := exec.Command("adb", "devices")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	var devices []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines[1:] { // Skip header line
		line = strings.TrimSpace(line)
		if line != "" && strings.Contains(line, "device") {
			parts := strings.Fields(line)
			if len(parts) >= 2 && parts[1] == "device" {
				devices = append(devices, parts[0])
			}
		}
	}

	return devices, nil
}

// PullFile pulls file from device
func (s *ADBService) PullFile(gatewayID uint, remotePath, localPath string) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	deviceAddr := s.getDeviceAddress(gateway)

	// Pull file
	cmd := exec.Command("adb", "-s", deviceAddr, "pull", remotePath, localPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to pull file: %w, output: %s", err, string(output))
	}

	return nil
}

// PushFile pushes file to device
func (s *ADBService) PushFile(gatewayID uint, localPath, remotePath string) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	deviceAddr := s.getDeviceAddress(gateway)

	// Push file
	cmd := exec.Command("adb", "-s", deviceAddr, "push", localPath, remotePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to push file: %w, output: %s", err, string(output))
	}

	return nil
}

// TapScreen taps on screen coordinates
func (s *ADBService) TapScreen(gatewayID uint, x, y int) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	deviceAddr := s.getDeviceAddress(gateway)

	// Tap screen
	cmd := exec.Command("adb", "-s", deviceAddr, "shell", "input", "tap", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to tap screen: %w", err)
	}

	return nil
}

// SwipeScreen performs swipe gesture
func (s *ADBService) SwipeScreen(gatewayID uint, x1, y1, x2, y2, duration int) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	deviceAddr := s.getDeviceAddress(gateway)

	// Swipe screen
	cmd := exec.Command("adb", "-s", deviceAddr, "shell", "input", "swipe",
		fmt.Sprintf("%d", x1), fmt.Sprintf("%d", y1),
		fmt.Sprintf("%d", x2), fmt.Sprintf("%d", y2),
		fmt.Sprintf("%d", duration))
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to swipe screen: %w", err)
	}

	return nil
}
