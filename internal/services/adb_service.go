package services

import (
	"fmt"
	"os"
	"spam-checker/internal/models"
	"strings"
	"time"

	"github.com/electricbubble/gadb"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ADBService struct {
	db      *gorm.DB
	clients map[uint]*gadb.Device
}

func NewADBService(db *gorm.DB) *ADBService {
	return &ADBService{
		db:      db,
		clients: make(map[uint]*gadb.Device),
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
	// Close connection if exists
	if client, exists := s.clients[id]; exists {
		client.Close()
		delete(s.clients, id)
	}

	if err := s.db.Delete(&models.ADBGateway{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete gateway: %w", err)
	}
	return nil
}

// getOrCreateClient gets or creates ADB client for gateway
func (s *ADBService) getOrCreateClient(gateway *models.ADBGateway) (*gadb.Device, error) {
	// Check if client already exists
	if client, exists := s.clients[gateway.ID]; exists {
		// Test if connection is still alive
		if _, err := client.DeviceInfo(); err == nil {
			return client, nil
		}
		// Connection is dead, remove it
		client.Close()
		delete(s.clients, gateway.ID)
	}

	// Create new ADB client
	adbClient, err := gadb.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create ADB client: %w", err)
	}

	// Connect to device
	address := fmt.Sprintf("%s:%d", gateway.Host, gateway.Port)
	err = adbClient.Connect(address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to device: %w", err)
	}

	// Get device
	device, err := adbClient.Device(gadb.WithSerial(address))
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	// Store client
	s.clients[gateway.ID] = device

	return device, nil
}

// UpdateGatewayStatus checks and updates gateway status
func (s *ADBService) UpdateGatewayStatus(gatewayID uint) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	status := "offline"
	deviceID := ""

	// Try to connect and get device info
	device, err := s.getOrCreateClient(gateway)
	if err == nil {
		// Get device info to verify connection
		info, err := device.DeviceInfo()
		if err == nil {
			status = "online"
			deviceID = fmt.Sprintf("%s:%d", gateway.Host, gateway.Port)
			logrus.Infof("Device info: %+v", info)
		} else {
			logrus.Errorf("Failed to get device info: %v", err)
		}
	} else {
		logrus.Errorf("Failed to connect to gateway %s: %v", gateway.Name, err)
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

	// Get device client
	device, err := s.getOrCreateClient(gateway)
	if err != nil {
		return fmt.Errorf("failed to get device client: %w", err)
	}

	// Open APK file
	apkFile, err := os.Open(apkPath)
	if err != nil {
		return fmt.Errorf("failed to open APK file: %w", err)
	}
	defer apkFile.Close()

	// Get file info for progress tracking
	apkInfo, err := apkFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get APK file info: %w", err)
	}

	// Push and install APK
	remotePath := fmt.Sprintf("/data/local/tmp/%s", apkInfo.Name())

	// Push APK to device
	err = device.Push(apkFile, remotePath, apkInfo.ModTime())
	if err != nil {
		return fmt.Errorf("failed to push APK: %w", err)
	}

	// Install APK
	output, err := device.RunShellCommand(fmt.Sprintf("pm install -r %s", remotePath))
	if err != nil {
		return fmt.Errorf("failed to install APK: %w", err)
	}

	if !strings.Contains(output, "Success") {
		return fmt.Errorf("APK installation failed: %s", output)
	}

	// Clean up remote file
	device.RunShellCommand(fmt.Sprintf("rm %s", remotePath))

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

	// Get device client
	device, err := s.getOrCreateClient(gateway)
	if err != nil {
		return "", fmt.Errorf("failed to get device client: %w", err)
	}

	// Execute command
	output, err := device.RunShellCommand(command)
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %w", err)
	}

	return output, nil
}

// GetDeviceInfo gets device information
func (s *ADBService) GetDeviceInfo(gatewayID uint) (map[string]string, error) {
	info := make(map[string]string)

	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return nil, err
	}

	// Get device client
	device, err := s.getOrCreateClient(gateway)
	if err != nil {
		return nil, fmt.Errorf("failed to get device client: %w", err)
	}

	// Get device info from gadb
	deviceInfo, err := device.DeviceInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get device info: %w", err)
	}

	// Convert DeviceInfo to map
	info["product"] = deviceInfo.Product
	info["model"] = deviceInfo.Model
	info["device"] = deviceInfo.Device
	info["features"] = strings.Join(deviceInfo.Features, ", ")
	info["abi"] = deviceInfo.ABI

	// Get additional properties
	props := []string{
		"ro.build.version.release",
		"ro.build.version.sdk",
		"ro.product.manufacturer",
	}

	for _, prop := range props {
		output, err := device.RunShellCommand(fmt.Sprintf("getprop %s", prop))
		if err == nil {
			info[prop] = strings.TrimSpace(output)
		}
	}

	// Get battery info
	batteryOutput, err := device.RunShellCommand("dumpsys battery")
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
	wmOutput, err := device.RunShellCommand("wm size")
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
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	// Get device client
	device, err := s.getOrCreateClient(gateway)
	if err != nil {
		return fmt.Errorf("failed to get device client: %w", err)
	}

	// Reboot device
	err = device.Reboot()
	if err != nil {
		return fmt.Errorf("failed to restart device: %w", err)
	}

	// Update status to offline (will be updated when device comes back online)
	s.db.Model(&models.ADBGateway{}).Where("id = ?", gatewayID).Update("status", "restarting")

	// Remove client from map
	delete(s.clients, gatewayID)

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

	// Get device client
	device, err := s.getOrCreateClient(gateway)
	if err != nil {
		return fmt.Errorf("failed to get device client: %w", err)
	}

	// Clear app data
	output, err := device.RunShellCommand(fmt.Sprintf("pm clear %s", appPackage))
	if err != nil {
		return fmt.Errorf("failed to clear app data: %w", err)
	}

	if !strings.Contains(output, "Success") {
		return fmt.Errorf("failed to clear app data: %s", output)
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

	// Get device client
	device, err := s.getOrCreateClient(gateway)
	if err != nil {
		return nil, fmt.Errorf("failed to get device client: %w", err)
	}

	// Take screenshot using gadb
	screenshot, err := device.Screenshot()
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	return screenshot, nil
}

// InputText inputs text on device
func (s *ADBService) InputText(gatewayID uint, text string) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	// Get device client
	device, err := s.getOrCreateClient(gateway)
	if err != nil {
		return fmt.Errorf("failed to get device client: %w", err)
	}

	// Input text
	_, err = device.RunShellCommand(fmt.Sprintf("input text '%s'", text))
	return err
}

// SendKeyEvent sends key event to device
func (s *ADBService) SendKeyEvent(gatewayID uint, keyCode string) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	// Get device client
	device, err := s.getOrCreateClient(gateway)
	if err != nil {
		return fmt.Errorf("failed to get device client: %w", err)
	}

	// Send key event
	_, err = device.RunShellCommand(fmt.Sprintf("input keyevent %s", keyCode))
	return err
}

// StartApp starts app on device
func (s *ADBService) StartApp(gatewayID uint, packageName, activityName string) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	// Get device client
	device, err := s.getOrCreateClient(gateway)
	if err != nil {
		return fmt.Errorf("failed to get device client: %w", err)
	}

	// Start app
	_, err = device.RunShellCommand(fmt.Sprintf("am start -n %s/%s", packageName, activityName))
	return err
}

// CloseAllConnections closes all ADB connections
func (s *ADBService) CloseAllConnections() {
	for id, client := range s.clients {
		client.Close()
		delete(s.clients, id)
	}
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
