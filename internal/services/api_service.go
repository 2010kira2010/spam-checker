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
	"github.com/tidwall/gjson"
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
		"phone":  phone.Number,
		"api":    apiService.Name,
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
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(apiService.Method, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
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
	log.Debugf("API response for %s: %s", phone.Number, rawResponse)

	// Extract data using JSONPath if configured
	extractedText := ""
	if apiService.ResponsePath != "" {
		extractedText = s.extractWithJSONPath(rawResponse, apiService.ResponsePath)
		log.Debugf("Extracted text using path '%s': %s", apiService.ResponsePath, extractedText)
	}

	// Extract keywords using JSONPath if configured
	var extractedKeywords []string
	if apiService.KeywordPaths != "" {
		extractedKeywords = s.extractKeywordsWithJSONPath(rawResponse, apiService.KeywordPaths)
		log.Debugf("Extracted keywords using path '%s': %v", apiService.KeywordPaths, extractedKeywords)
	}

	// Analyze response for spam
	isSpam, foundKeywords := s.analyzeAPIResponse(rawResponse, extractedText, extractedKeywords, service.ID)

	// Save result
	result := &models.CheckResult{
		PhoneNumberID: phone.ID,
		ServiceID:     service.ID,
		IsSpam:        isSpam,
		FoundKeywords: models.StringArray(foundKeywords),
		RawResponse:   rawResponse,
		RawText:       extractedText, // Store extracted text in RawText field
		CheckedAt:     time.Now(),
	}

	if err := s.db.Create(result).Error; err != nil {
		return nil, fmt.Errorf("failed to save check result: %w", err)
	}

	log.Infof("API check completed for %s on %s: isSpam=%v, keywords=%v",
		phone.Number, apiService.Name, isSpam, foundKeywords)

	return result, nil
}

// extractWithJSONPath extracts data using JSONPath
func (s *APICheckService) extractWithJSONPath(jsonStr string, jsonPath string) string {
	if jsonPath == "" {
		return ""
	}

	// Handle multiple paths separated by comma
	paths := strings.Split(jsonPath, ",")
	var results []string

	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		// Use gjson for JSONPath evaluation
		result := gjson.Get(jsonStr, s.convertToGJSONPath(path))
		if result.Exists() {
			if result.IsArray() {
				// Join array elements
				result.ForEach(func(key, value gjson.Result) bool {
					results = append(results, value.String())
					return true
				})
			} else {
				results = append(results, result.String())
			}
		}
	}

	return strings.Join(results, " ")
}

// extractKeywordsWithJSONPath extracts keywords using JSONPath
func (s *APICheckService) extractKeywordsWithJSONPath(jsonStr string, jsonPaths string) []string {
	if jsonPaths == "" {
		return []string{}
	}

	keywordSet := make(map[string]bool)
	paths := strings.Split(jsonPaths, ",")

	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		// Use gjson for JSONPath evaluation
		result := gjson.Get(jsonStr, s.convertToGJSONPath(path))
		if result.Exists() {
			if result.IsArray() {
				// Extract each array element
				result.ForEach(func(key, value gjson.Result) bool {
					keyword := strings.TrimSpace(value.String())
					if keyword != "" {
						keywordSet[keyword] = true
					}
					return true
				})
			} else {
				// Single value
				keyword := strings.TrimSpace(result.String())
				if keyword != "" {
					keywordSet[keyword] = true
				}
			}
		}
	}

	// Convert set to slice
	keywords := make([]string, 0, len(keywordSet))
	for k := range keywordSet {
		keywords = append(keywords, k)
	}

	return keywords
}

// convertToGJSONPath converts JSONPath syntax to gjson path syntax
func (s *APICheckService) convertToGJSONPath(jsonPath string) string {
	// Remove leading $ if present
	path := strings.TrimPrefix(jsonPath, "$.")
	path = strings.TrimPrefix(path, "$")

	// Replace array notation
	path = strings.ReplaceAll(path, "[*]", "#")
	path = strings.ReplaceAll(path, "[?(@", "#(")
	path = strings.ReplaceAll(path, ")]", ")")

	// Handle specific patterns
	if strings.Contains(path, "==") {
		// Convert equality checks to gjson syntax
		path = strings.ReplaceAll(path, "=='", "==\"")
		path = strings.ReplaceAll(path, "').", "\").")
	}

	return path
}

// analyzeAPIResponse analyzes API response for spam indicators
func (s *APICheckService) analyzeAPIResponse(rawResponse string, extractedText string, extractedKeywords []string, serviceID uint) (bool, []string) {
	log := s.log.WithFields(logrus.Fields{
		"method":    "analyzeAPIResponse",
		"serviceID": serviceID,
	})

	var foundKeywords []string

	// Get spam keywords from database
	var dbKeywords []models.SpamKeyword
	query := s.db.Where("is_active = ?", true)
	query = query.Where("service_id IS NULL OR service_id = ?", serviceID)

	if err := query.Find(&dbKeywords).Error; err != nil {
		log.Errorf("Failed to get spam keywords: %v", err)
		return false, foundKeywords
	}

	// Create keyword set for quick lookup
	keywordSet := make(map[string]string) // lowercase -> original
	for _, kw := range dbKeywords {
		keywordSet[strings.ToLower(kw.Keyword)] = kw.Keyword
	}

	// Check extracted keywords against database keywords
	for _, extractedKw := range extractedKeywords {
		extractedLower := strings.ToLower(extractedKw)
		if original, exists := keywordSet[extractedLower]; exists {
			foundKeywords = append(foundKeywords, original)
		}
		// Also check if extracted keyword contains any database keywords
		for dbKwLower, dbKwOriginal := range keywordSet {
			if strings.Contains(extractedLower, dbKwLower) {
				foundKeywords = append(foundKeywords, dbKwOriginal)
			}
		}
	}

	// Check extracted text for keywords
	if extractedText != "" {
		textLower := strings.ToLower(extractedText)
		for dbKwLower, dbKwOriginal := range keywordSet {
			if strings.Contains(textLower, dbKwLower) {
				// Check if not already added
				alreadyAdded := false
				for _, fk := range foundKeywords {
					if fk == dbKwOriginal {
						alreadyAdded = true
						break
					}
				}
				if !alreadyAdded {
					foundKeywords = append(foundKeywords, dbKwOriginal)
				}
			}
		}
	}

	// Also check the full response for keywords (as fallback)
	responseLower := strings.ToLower(rawResponse)
	for dbKwLower, dbKwOriginal := range keywordSet {
		if strings.Contains(responseLower, dbKwLower) {
			// Check if not already added
			alreadyAdded := false
			for _, fk := range foundKeywords {
				if fk == dbKwOriginal {
					alreadyAdded = true
					break
				}
			}
			if !alreadyAdded {
				foundKeywords = append(foundKeywords, dbKwOriginal)
			}
		}
	}

	// Also check for common spam indicators in JSON structure
	isSpamFromStructure := s.checkJSONForSpamIndicators(rawResponse)

	// Determine if it's spam
	isSpam := len(foundKeywords) > 0 || isSpamFromStructure

	log.Debugf("Analysis complete: isSpam=%v, foundKeywords=%v", isSpam, foundKeywords)

	return isSpam, foundKeywords
}

// checkJSONForSpamIndicators checks JSON response for common spam indicator fields
func (s *APICheckService) checkJSONForSpamIndicators(jsonStr string) bool {
	// Common spam indicator fields
	spamFields := []string{
		"spam",
		"is_spam",
		"isSpam",
		"unwanted",
		"junk",
		"спам",
		"нежелательный",
		"fraud",
		"scam",
		"мошенник",
	}

	jsonLower := strings.ToLower(jsonStr)

	for _, field := range spamFields {
		// Check various patterns
		patterns := []string{
			fmt.Sprintf(`"%s":true`, field),
			fmt.Sprintf(`"%s": true`, field),
			fmt.Sprintf(`"%s":"true"`, field),
			fmt.Sprintf(`"%s": "true"`, field),
			fmt.Sprintf(`"%s":"yes"`, field),
			fmt.Sprintf(`"%s": "yes"`, field),
			fmt.Sprintf(`"%s":"да"`, field),
			fmt.Sprintf(`"%s": "да"`, field),
			fmt.Sprintf(`"%s":1`, field),
			fmt.Sprintf(`"%s": 1`, field),
		}

		for _, pattern := range patterns {
			if strings.Contains(jsonLower, pattern) {
				return true
			}
		}
	}

	// Check for negative polarity
	if strings.Contains(jsonLower, `"polarity":"negative"`) ||
		strings.Contains(jsonLower, `"polarity": "negative"`) {
		return true
	}

	return false
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
	replacements := map[string]string{
		"{{phone}}":        digitsOnly,
		"{{phoneNumber}}":  digitsOnly,
		"{{number}}":       digitsOnly,
		"{{PHONE}}":        digitsOnly,
		"{{PHONE_NUMBER}}": digitsOnly,
		"{phone}":          digitsOnly,
		"{phoneNumber}":    digitsOnly,
		"{number}":         digitsOnly,
		"{PHONE}":          digitsOnly,
		"{PHONE_NUMBER}":   digitsOnly,
	}

	// Replace all placeholders
	for placeholder, value := range replacements {
		str = strings.ReplaceAll(str, placeholder, value)
	}

	// Handle formatted versions if phone is Russian format
	if len(digitsOnly) == 11 && digitsOnly[0] == '7' {
		// With plus
		str = strings.ReplaceAll(str, "{{+phone}}", "+"+digitsOnly)
		str = strings.ReplaceAll(str, "{+phone}", "+"+digitsOnly)

		// Formatted version
		formatted := fmt.Sprintf("+%s (%s) %s-%s-%s",
			digitsOnly[0:1], digitsOnly[1:4], digitsOnly[4:7], digitsOnly[7:9], digitsOnly[9:11])
		str = strings.ReplaceAll(str, "{{phone_formatted}}", formatted)
		str = strings.ReplaceAll(str, "{phone_formatted}", formatted)
	}

	return str
}

// TestAPIService tests an API service with a sample phone number
func (s *APICheckService) TestAPIService(id uint, testPhone string) (map[string]interface{}, error) {
	apiService, err := s.GetAPIServiceByID(id)
	if err != nil {
		return nil, err
	}

	// Test the API
	startTime := time.Now()

	url := s.replacePhonePlaceholder(apiService.APIURL, testPhone)

	var req *http.Request
	if apiService.Method == "POST" && apiService.RequestBody != "" {
		body := s.replacePhonePlaceholder(apiService.RequestBody, testPhone)
		req, err = http.NewRequest(apiService.Method, url, bytes.NewBuffer([]byte(body)))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(apiService.Method, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
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

	// Extract data using JSONPath
	extractedText := ""
	if apiService.ResponsePath != "" {
		extractedText = s.extractWithJSONPath(responseStr, apiService.ResponsePath)
	}

	// Extract keywords using JSONPath
	var extractedKeywords []string
	if apiService.KeywordPaths != "" {
		extractedKeywords = s.extractKeywordsWithJSONPath(responseStr, apiService.KeywordPaths)
	}

	// Get service ID for keyword lookup
	var service models.SpamService
	s.db.Where("code = ?", apiService.ServiceCode).First(&service)

	// Analyze for spam
	isSpam, keywords := s.analyzeAPIResponse(responseStr, extractedText, extractedKeywords, service.ID)

	return map[string]interface{}{
		"success":            true,
		"status_code":        resp.StatusCode,
		"response_time":      responseTime,
		"response":           responseStr,
		"extracted_text":     extractedText,
		"extracted_keywords": extractedKeywords,
		"is_spam":            isSpam,
		"keywords":           keywords,
		"url":                url,
	}, nil
}
