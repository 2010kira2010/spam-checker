package services

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"spam-checker/internal/config"
	"spam-checker/internal/models"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ADBService struct {
	db           *gorm.DB
	dockerClient *client.Client
	cfg          *config.Config
}

func NewADBService(db *gorm.DB, cfg *config.Config) *ADBService {
	return NewADBServiceWithConfig(db, cfg)
}

func NewADBServiceWithConfig(db *gorm.DB, cfg *config.Config) *ADBService {
	// Initialize Docker client
	dockerHost := "unix:///var/run/docker.sock"
	if cfg != nil && cfg.Docker.Host != "" {
		dockerHost = cfg.Docker.Host
	}

	dockerClient, err := client.NewClientWithOpts(
		client.WithHost(dockerHost),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		logrus.Errorf("Failed to create Docker client: %v", err)
	}

	return &ADBService{
		db:           db,
		dockerClient: dockerClient,
		cfg:          cfg,
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
	containerName := s.getContainerName(gateway)

	// Check if Docker client is available
	if s.dockerClient == nil {
		logrus.Error("Docker client is not initialized")
		return fmt.Errorf("Docker client is not initialized")
	}

	// Check if container is running
	ctx := context.Background()
	containers, err := s.dockerClient.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list containers: %v", err)
		return err
	}

	for _, cont := range containers {
		for _, name := range cont.Names {
			// Container names in Docker have leading slash
			if strings.TrimPrefix(name, "/") == containerName {
				if cont.State == "running" {
					// Test ADB connection inside container
					output, err := s.executeInContainer(containerName, []string{"adb", "devices"})
					if err == nil && strings.Contains(output, "emulator") {
						status = "online"
					}
				}
				break
			}
		}
	}

	// Update status
	now := time.Now()
	updates := map[string]interface{}{
		"status":    status,
		"device_id": containerName,
		"last_ping": &now,
	}

	if err := s.db.Model(gateway).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update gateway status: %w", err)
	}

	logrus.Infof("Gateway %s (%s) status updated: %s", gateway.Name, containerName, status)

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

// ExecuteCommand executes ADB command on gateway
func (s *ADBService) ExecuteCommand(gatewayID uint, command string) (string, error) {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return "", err
	}

	if gateway.Status != "online" {
		return "", fmt.Errorf("gateway %s is not online", gateway.Name)
	}

	containerName := s.getContainerName(gateway)

	// Execute command inside container
	fullCommand := []string{"adb", "shell"}
	fullCommand = append(fullCommand, strings.Fields(command)...)

	return s.executeInContainer(containerName, fullCommand)
}

// GetDeviceInfo gets device information
func (s *ADBService) GetDeviceInfo(gatewayID uint) (map[string]string, error) {
	info := make(map[string]string)

	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return nil, err
	}

	containerName := s.getContainerName(gateway)

	// Get device state
	output, err := s.executeInContainer(containerName, []string{"adb", "get-state"})
	if err == nil {
		info["state"] = strings.TrimSpace(output)
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
		output, err = s.executeInContainer(containerName, []string{"adb", "shell", "getprop", prop})
		if err == nil {
			info[key] = strings.TrimSpace(output)
		}
	}

	// Get battery info
	output, err = s.executeInContainer(containerName, []string{"adb", "shell", "dumpsys", "battery"})
	if err == nil {
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "level:") {
				info["battery_level"] = strings.TrimSpace(strings.TrimPrefix(line, "level:"))
			} else if strings.HasPrefix(line, "temperature:") {
				temp := strings.TrimSpace(strings.TrimPrefix(line, "temperature:"))
				if tempInt := strings.TrimSpace(temp); tempInt != "" {
					info["battery_temperature"] = tempInt
				}
			}
		}
	}

	// Get screen resolution
	output, err = s.executeInContainer(containerName, []string{"adb", "shell", "wm", "size"})
	if err == nil {
		if idx := strings.Index(output, "Physical size:"); idx != -1 {
			size := strings.TrimSpace(output[idx+14:])
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

	containerName := s.getContainerName(gateway)

	// Reboot device
	_, err = s.executeInContainer(containerName, []string{"adb", "reboot"})
	if err != nil {
		return fmt.Errorf("failed to restart device: %w", err)
	}

	// Update status to restarting
	s.db.Model(&models.ADBGateway{}).Where("id = ?", gatewayID).Update("status", "restarting")

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

	containerName := s.getContainerName(gateway)

	// Read APK file
	apkFile, err := os.Open(apkPath)
	if err != nil {
		return fmt.Errorf("failed to open APK file: %w", err)
	}
	defer apkFile.Close()

	// Get file info
	fileInfo, err := apkFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Create tar archive
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Add file to tar
	header := &tar.Header{
		Name: "app.apk",
		Mode: 0644,
		Size: fileInfo.Size(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	if _, err := io.Copy(tw, apkFile); err != nil {
		return fmt.Errorf("failed to write file to tar: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	// Copy to container
	ctx := context.Background()
	err = s.dockerClient.CopyToContainer(ctx, containerName, "/tmp/", &buf, container.CopyToContainerOptions{})
	if err != nil {
		return fmt.Errorf("failed to copy APK to container: %w", err)
	}

	// Install APK
	output, err := s.executeInContainer(containerName, []string{"adb", "install", "-r", "/tmp/app.apk"})
	if err != nil {
		return fmt.Errorf("failed to install APK: %w, output: %s", err, output)
	}

	if !strings.Contains(output, "Success") {
		return fmt.Errorf("APK installation failed: %s", output)
	}

	// Clean up
	s.executeInContainer(containerName, []string{"rm", "/tmp/app.apk"})

	logrus.Infof("APK installed successfully on gateway %s", gateway.Name)

	return nil
}

// TakeScreenshot takes a screenshot from device
func (s *ADBService) TakeScreenshot(gatewayID uint) ([]byte, error) {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return nil, err
	}

	containerName := s.getContainerName(gateway)

	// Take screenshot inside container and save to file
	_, err = s.executeInContainer(containerName, []string{"adb", "shell", "screencap", "-p", "/sdcard/screenshot.png"})
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	// Pull screenshot from device to container filesystem
	_, err = s.executeInContainer(containerName, []string{"adb", "pull", "/sdcard/screenshot.png", "/tmp/screenshot.png"})
	if err != nil {
		return nil, fmt.Errorf("failed to pull screenshot: %w", err)
	}

	// Read screenshot from container
	ctx := context.Background()
	reader, _, err := s.dockerClient.CopyFromContainer(ctx, containerName, "/tmp/screenshot.png")
	if err != nil {
		return nil, fmt.Errorf("failed to copy screenshot from container: %w", err)
	}
	defer reader.Close()

	// Extract from tar
	tr := tar.NewReader(reader)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar: %w", err)
		}

		if header.Name == "screenshot.png" || filepath.Base(header.Name) == "screenshot.png" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("failed to read screenshot data: %w", err)
			}

			// Clean up
			s.executeInContainer(containerName, []string{"rm", "/tmp/screenshot.png"})
			s.executeInContainer(containerName, []string{"adb", "shell", "rm", "/sdcard/screenshot.png"})

			return data, nil
		}
	}

	return nil, fmt.Errorf("screenshot not found in tar archive")
}

// InputText inputs text on device
func (s *ADBService) InputText(gatewayID uint, text string) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	containerName := s.getContainerName(gateway)

	// Escape special characters for shell
	text = strings.ReplaceAll(text, "'", "'\"'\"'")

	// Input text
	_, err = s.executeInContainer(containerName, []string{"adb", "shell", "input", "text", "'" + text + "'"})
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

	containerName := s.getContainerName(gateway)

	// Send key event
	_, err = s.executeInContainer(containerName, []string{"adb", "shell", "input", "keyevent", keyCode})
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

	containerName := s.getContainerName(gateway)

	// Start app
	output, err := s.executeInContainer(containerName, []string{"adb", "shell", "am", "start", "-n", packageName + "/" + activityName})
	if err != nil {
		return fmt.Errorf("failed to start app: %w, output: %s", err, output)
	}

	return nil
}

// SimulateIncomingCall simulates incoming call
func (s *ADBService) SimulateIncomingCall(gatewayID uint, phoneNumber string) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	containerName := s.getContainerName(gateway)

	// Normalize phone number for GSM emulator - only digits allowed
	// Remove all non-digit characters
	normalizedNumber := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phoneNumber)

	// Simulate incoming call using emulator console
	output, err := s.executeInContainer(containerName, []string{"adb", "emu", "gsm", "call", normalizedNumber})
	if err != nil {
		return fmt.Errorf("failed to simulate call: %w, output: %s", err, output)
	}

	logrus.Infof("Simulated incoming call from %s on gateway %s", normalizedNumber, gateway.Name)

	return nil
}

// EndCall ends current call
func (s *ADBService) EndCall(gatewayID uint, phoneNumber string) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	containerName := s.getContainerName(gateway)

	// Try different methods to end call
	// Method 1: Try to cancel via GSM emulator (without phone number)
	output, err := s.executeInContainer(containerName, []string{"adb", "emu", "gsm", "cancel", phoneNumber})
	if err != nil {
		logrus.Warnf("Failed to cancel call via GSM emulator: %v", err)

		// Method 2: Use key event as fallback
		err = s.SendKeyEvent(gatewayID, "KEYCODE_ENDCALL")
		if err != nil {
			logrus.Warnf("Failed to end call via KEYCODE_ENDCALL: %v", err)

			// Method 3: Try HOME key to dismiss call screen
			err = s.SendKeyEvent(gatewayID, "KEYCODE_HOME")
			if err != nil {
				return fmt.Errorf("failed to end call using all methods")
			}
			logrus.Info("Dismissed call screen using HOME key")
			return nil
		}
		logrus.Info("Ended call using KEYCODE_ENDCALL")
		return nil
	}

	logrus.Infof("Ended call on gateway %s: %s", gateway.Name, output)
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

	containerName := s.getContainerName(gateway)

	// Clear app data
	output, err := s.executeInContainer(containerName, []string{"adb", "shell", "pm", "clear", appPackage})
	if err != nil {
		return fmt.Errorf("failed to clear app data: %w, output: %s", err, output)
	}

	if !strings.Contains(output, "Success") {
		return fmt.Errorf("failed to clear app data: %s", output)
	}

	logrus.Infof("App data cleared for %s on gateway %d", appPackage, gatewayID)

	return nil
}

// TapScreen taps on screen coordinates
func (s *ADBService) TapScreen(gatewayID uint, x, y int) error {
	gateway, err := s.GetGatewayByID(gatewayID)
	if err != nil {
		return err
	}

	containerName := s.getContainerName(gateway)

	// Tap screen
	_, err = s.executeInContainer(containerName, []string{"adb", "shell", "input", "tap", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y)})
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

	containerName := s.getContainerName(gateway)

	// Swipe screen
	_, err = s.executeInContainer(containerName, []string{"adb", "shell", "input", "swipe",
		fmt.Sprintf("%d", x1), fmt.Sprintf("%d", y1),
		fmt.Sprintf("%d", x2), fmt.Sprintf("%d", y2),
		fmt.Sprintf("%d", duration)})
	if err != nil {
		return fmt.Errorf("failed to swipe screen: %w", err)
	}

	return nil
}

// executeInContainer executes command inside Docker container
func (s *ADBService) executeInContainer(containerName string, cmd []string) (string, error) {
	if s.dockerClient == nil {
		return "", fmt.Errorf("Docker client is not initialized")
	}

	ctx := context.Background()

	// Create exec configuration
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
	}

	// Create exec
	execID, err := s.dockerClient.ContainerExecCreate(ctx, containerName, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	// Start exec
	resp, err := s.dockerClient.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to start exec: %w", err)
	}
	defer resp.Close()

	// Read output
	output := new(bytes.Buffer)
	_, err = io.Copy(output, resp.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to read output: %w", err)
	}

	// Check exec result
	execInspect, err := s.dockerClient.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return output.String(), fmt.Errorf("failed to inspect exec: %w", err)
	}

	if execInspect.ExitCode != 0 {
		return output.String(), fmt.Errorf("command exited with code %d", execInspect.ExitCode)
	}

	return output.String(), nil
}

// getContainerName returns Docker container name for gateway
func (s *ADBService) getContainerName(gateway *models.ADBGateway) string {
	// Map gateway to container name based on service code
	switch gateway.ServiceCode {
	case "yandex_aon":
		return "spam_checker_android_yandex"
	case "kaspersky":
		return "spam_checker_android_kaspersky"
	case "getcontact":
		return "spam_checker_android_getcontact"
	default:
		return fmt.Sprintf("android-%s", gateway.Host)
	}
}

// CheckDockerConnection checks if Docker is accessible
func (s *ADBService) CheckDockerConnection() error {
	if s.dockerClient == nil {
		return fmt.Errorf("Docker client is not initialized")
	}

	ctx := context.Background()
	_, err := s.dockerClient.Ping(ctx)
	if err != nil {
		return fmt.Errorf("failed to ping Docker: %w", err)
	}

	return nil
}

// ListDockerContainers lists all Docker containers
func (s *ADBService) ListDockerContainers() ([]types.Container, error) {
	if s.dockerClient == nil {
		return nil, fmt.Errorf("Docker client is not initialized")
	}

	ctx := context.Background()
	containers, err := s.dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	return containers, nil
}

// Close closes Docker client connection
func (s *ADBService) Close() error {
	if s.dockerClient != nil {
		return s.dockerClient.Close()
	}
	return nil
}
