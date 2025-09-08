package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"spam-checker/internal/logger"
	"spam-checker/internal/models"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type APICheckService struct {
	db  *gorm.DB
	log *logrus.Entry
}

func NewAPICheckService(db *gorm.DB) *APICheckService {
	return &APICheckService{
		db:  db,
		log: logger.WithField("service", "APICheckService"),
	}
}

// CreateAPIService creates a new API service
func (s *APICheckService) CreateAPIService(service *models.APIService) error {
	// Validate headers JSON
	if service.Headers != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(service.Headers), &headers); err != nil {
			return fmt.Errorf("invalid headers JSON: %w", err)
		}
	}

	if err := s.db.Create(service).Error; err != nil {
		return fmt.Errorf("failed to create API service: %w", err)
	}

	return nil
}

// GetAPIServiceByID gets API service by ID
func (s *APICheckService) GetAPIServiceByID(id uint) (*models.APIService, error) {
	var service models.APIService
	if err := s.db.First(&service, id).Error; err != nil {
		return nil, fmt.Errorf("API service not found: %w", err)
	}
	return &service, nil
}

// ListAPIServices lists all API services
func (s *APICheckService) ListAPIServices() ([]models.APIService, error) {
	var services []models.APIService
	if err := s.db.Find(&services).Error; err != nil {
		return nil, fmt.Errorf("failed to list API services: %w", err)
	}
	return services, nil
}

// GetActiveAPIServices gets all active API services
func (s *APICheckService) GetActiveAPIServices() ([]models.APIService, error) {
	var services []models.APIService
	if err := s.db.Where("is_active = ?", true).Find(&services).Error; err != nil {
		return nil, fmt.Errorf("failed to get active API services: %w", err)
	}
	return services, nil
}

// UpdateAPIService updates API service information
func (s *APICheckService) UpdateAPIService(id uint, updates map[string]interface{}) error {
	// Validate headers if being updated
	if headers, ok := updates["headers"].(string); ok && headers != "" {
		var headersMap map[string]string
		if err := json.Unmarshal([]byte(headers), &headersMap); err != nil {
			return fmt.Errorf("invalid headers JSON: %w", err)
		}
	}

	if err := s.db.Model(&models.APIService{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update API service: %w", err)
	}

	return nil
}

// DeleteAPIService deletes an API service
func (s *APICheckService) DeleteAPIService(id uint) error {
	if err := s.db.Delete(&models.APIService{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete API service: %w", err)
	}
	return nil
}

// CheckPhoneViaAPI checks phone number using external API
func (s *APICheckService) CheckPhoneViaAPI(phone *models.PhoneNumber, apiService *models.APIService) (*models.CheckResult, error) {
	log := s.log.WithFields(logrus.Fields{
		"method": "CheckPhoneViaAPI",
	})

	// Get service info
	var service models.SpamService
	if err := s.db.Where("code = ?", apiService.ServiceCode).First(&service).Error; err != nil {
		return nil, fmt.Errorf("spam service not found: %w", err)
	}

	log.Infof("Checking %s via API service %s", phone.Number, apiService.Name)

	// Replace placeholders in URL
	url := s.replacePhonePlaceholder(apiService.APIURL, phone.Number)

	// Create request
	var req *http.Request
	var err error

	if apiService.Method == "POST" && apiService.RequestBody != "" {
		// Replace placeholders in request body
		body := s.replacePhonePlaceholder(apiService.RequestBody, phone.Number)
		req, err = http.NewRequest(apiService.Method, url, bytes.NewBuffer([]byte(body)))
	} else {
		req, err = http.NewRequest(apiService.Method, url, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	if apiService.Headers != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(apiService.Headers), &headers); err == nil {
			for key, value := range headers {
				req.Header.Set(key, value)
			}
		}
	}

	// Set timeout
	client := &http.Client{
		Timeout: time.Duration(apiService.Timeout) * time.Second,
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Process response
	rawResponse := string(body)
	isSpam, foundKeywords := s.analyzeAPIResponse(rawResponse, service.ID)

	// Save result
	result := &models.CheckResult{
		PhoneNumberID: phone.ID,
		ServiceID:     service.ID,
		IsSpam:        isSpam,
		FoundKeywords: models.StringArray(foundKeywords),
		RawResponse:   rawResponse,
		CheckedAt:     time.Now(),
	}

	if err := s.db.Create(result).Error; err != nil {
		return nil, fmt.Errorf("failed to save check result: %w", err)
	}

	log.Infof("API check completed for %s on %s: isSpam=%v, keywords=%v",
		phone.Number, apiService.Name, isSpam, foundKeywords)

	return result, nil
}

// replacePhonePlaceholder replaces phone number placeholders in string
func (s *APICheckService) replacePhonePlaceholder(str string, phoneNumber string) string {
	// Remove non-digits from phone number
	digitsOnly := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phoneNumber)

	// Replace various placeholder formats
	str = strings.ReplaceAll(str, "{phone}", digitsOnly)
	str = strings.ReplaceAll(str, "{phoneNumber}", digitsOnly)
	str = strings.ReplaceAll(str, "{number}", digitsOnly)
	str = strings.ReplaceAll(str, "{PHONE}", digitsOnly)
	str = strings.ReplaceAll(str, "{PHONE_NUMBER}", digitsOnly)

	// Replace with formatted versions if needed
	if len(digitsOnly) == 11 && digitsOnly[0] == '7' {
		// Russian format with various representations
		str = strings.ReplaceAll(str, "{+phone}", "+"+digitsOnly)
		str = strings.ReplaceAll(str, "{phone_formatted}", fmt.Sprintf("+%s (%s) %s-%s-%s",
			digitsOnly[0:1], digitsOnly[1:4], digitsOnly[4:7], digitsOnly[7:9], digitsOnly[9:11]))
	}

	// Replace numeric placeholders (for cases like {79999999999})
	for i := 0; i < len(str); i++ {
		if str[i] == '{' {
			end := strings.Index(str[i:], "}")
			if end > 0 {
				placeholder := str[i+1 : i+end]
				if strings.HasPrefix(placeholder, "7") && len(placeholder) == 11 {
					// Check if it's all digits
					allDigits := true
					for _, ch := range placeholder {
						if ch < '0' || ch > '9' {
							allDigits = false
							break
						}
					}
					if allDigits {
						str = strings.ReplaceAll(str, "{"+placeholder+"}", digitsOnly)
					}
				}
			}
		}
	}

	return str
}

// analyzeAPIResponse analyzes API response for spam indicators
func (s *APICheckService) analyzeAPIResponse(response string, serviceID uint) (bool, []string) {
	log := s.log.WithFields(logrus.Fields{
		"method": "analyzeAPIResponse",
	})

	responseText := strings.ToLower(response)
	var foundKeywords []string

	// Get keywords
	var keywords []models.SpamKeyword
	query := s.db.Where("is_active = ?", true)
	query = query.Where("service_id IS NULL OR service_id = ?", serviceID)

	if err := query.Find(&keywords).Error; err != nil {
		log.Errorf("Failed to get spam keywords: %v", err)
		return false, foundKeywords
	}

	// Check each keyword in response
	for _, keyword := range keywords {
		if strings.Contains(responseText, strings.ToLower(keyword.Keyword)) {
			foundKeywords = append(foundKeywords, keyword.Keyword)
		}
	}

	// Also try to parse JSON response for specific fields
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(response), &jsonData); err == nil {
		// Check for common spam indicators in JSON
		isSpam := s.checkJSONForSpam(jsonData, &foundKeywords)
		if isSpam {
			return true, foundKeywords
		}
	}

	return len(foundKeywords) > 0, foundKeywords
}

// checkJSONForSpam checks JSON response for spam indicators
func (s *APICheckService) checkJSONForSpam(data map[string]interface{}, foundKeywords *[]string) bool {
	// Check various possible spam indicator fields
	spamFields := []string{"spam", "is_spam", "isSpam", "unwanted", "junk", "спам", "нежелательный"}

	for _, field := range spamFields {
		if val, exists := data[field]; exists {
			switch v := val.(type) {
			case bool:
				if v {
					*foundKeywords = append(*foundKeywords, field)
					return true
				}
			case string:
				if v == "true" || v == "1" || v == "yes" || v == "да" {
					*foundKeywords = append(*foundKeywords, field)
					return true
				}
			case float64:
				if v > 0 {
					*foundKeywords = append(*foundKeywords, field)
					return true
				}
			}
		}
	}

	// Check for nested structures (like in Yandex response)
	if result, ok := data["result"].(map[string]interface{}); ok {
		if oldUgc, ok := result["old_ugc"].(map[string]interface{}); ok {
			if verdict, ok := oldUgc["verdict"].(string); ok && verdict != "" {
				// Check if verdict contains spam keywords
				verdictLower := strings.ToLower(verdict)
				spamIndicators := []string{"спам", "реклама", "мошенник", "коллектор", "spam", "scam", "fraud"}

				for _, indicator := range spamIndicators {
					if strings.Contains(verdictLower, indicator) {
						*foundKeywords = append(*foundKeywords, verdict)
						return true
					}
				}
			}

			if polarity, ok := oldUgc["polarity"].(string); ok && polarity == "negative" {
				*foundKeywords = append(*foundKeywords, "negative_polarity")
				return true
			}
		}

		// Check questionary for spam indicators
		if questionary, ok := result["questionary"].(map[string]interface{}); ok {
			return s.checkQuestionaryForSpam(questionary, foundKeywords)
		}
	}

	return false
}

// checkQuestionaryForSpam checks questionary structure for spam indicators
func (s *APICheckService) checkQuestionaryForSpam(questionary map[string]interface{}, foundKeywords *[]string) bool {
	// Check for dialogs with negative options selected
	if dialogs, ok := questionary["Dialogs"].([]interface{}); ok {
		for _, dialog := range dialogs {
			if d, ok := dialog.(map[string]interface{}); ok {
				if questions, ok := d["questions"].([]interface{}); ok {
					for _, question := range questions {
						if q, ok := question.(map[string]interface{}); ok {
							if answers, ok := q["answers"].([]interface{}); ok {
								for _, answer := range answers {
									if a, ok := answer.(map[string]interface{}); ok {
										if store, ok := a["store"].(map[string]interface{}); ok {
											// Check if it's marked as negative
											if polarity, ok := store["polarity"].(string); ok && polarity == "negative" {
												if tag, ok := a["tag"].(string); ok {
													*foundKeywords = append(*foundKeywords, tag)
													return true
												}
											}

											// Check for spam-related intentions
											if intention, ok := store["intention"].(string); ok {
												spamIntentions := []string{"spam", "telemarketing", "scam", "debt", "silent", "fraud"}
												for _, spamInt := range spamIntentions {
													if intention == spamInt {
														*foundKeywords = append(*foundKeywords, intention)
														return true
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return false
}

// TestAPIService tests an API service with a sample phone number
func (s *APICheckService) TestAPIService(id uint, testPhone string) (map[string]interface{}, error) {
	apiService, err := s.GetAPIServiceByID(id)
	if err != nil {
		return nil, err
	}

	// Create temporary phone for testing
	//tempPhone := &models.PhoneNumber{
	//	Number:      testPhone,
	//	Description: "API Test",
	//	IsActive:    false,
	//}

	// Test the API
	startTime := time.Now()

	url := s.replacePhonePlaceholder(apiService.APIURL, testPhone)

	var req *http.Request
	if apiService.Method == "POST" && apiService.RequestBody != "" {
		body := s.replacePhonePlaceholder(apiService.RequestBody, testPhone)
		req, err = http.NewRequest(apiService.Method, url, bytes.NewBuffer([]byte(body)))
	} else {
		req, err = http.NewRequest(apiService.Method, url, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	if apiService.Headers != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(apiService.Headers), &headers); err == nil {
			for key, value := range headers {
				req.Header.Set(key, value)
			}
		}
	}

	client := &http.Client{
		Timeout: time.Duration(apiService.Timeout) * time.Second,
	}

	resp, err := client.Do(req)
	responseTime := time.Since(startTime).Milliseconds()

	if err != nil {
		return map[string]interface{}{
			"success":       false,
			"error":         err.Error(),
			"response_time": responseTime,
		}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	responseStr := string(body)

	// Try to format JSON response
	var formattedResponse interface{}
	if err := json.Unmarshal(body, &formattedResponse); err == nil {
		// Successfully parsed as JSON
		responseStr = string(body)
	}

	isSpam, keywords := s.analyzeAPIResponse(responseStr, 0)

	return map[string]interface{}{
		"success":       true,
		"status_code":   resp.StatusCode,
		"response_time": responseTime,
		"response":      responseStr,
		"is_spam":       isSpam,
		"keywords":      keywords,
		"url":           url,
	}, nil
}
