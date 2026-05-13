package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"project-phoenix/v2/internal/db"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type VisitController struct {
	CollectionName string
	DB             db.DBInterface
}

func (c *VisitController) GetCollectionName() string {
	return "visits"
}

func (c *VisitController) PerformIndexing() error {
	if c.DB == nil {
		log.Println("Warning: DB instance is nil, skipping indexing")
		return nil
	}

	if err := c.DB.ValidateIndexing(c.GetCollectionName(), bson.D{{Key: "created_at", Value: -1}}); err != nil {
		return err
	}

	if err := c.DB.ValidateUniqueIndexing(c.GetCollectionName(), bson.D{{Key: "ip", Value: 1}, {Key: "project_type", Value: 1}}); err != nil {
		return err
	}

	return nil
}

func (c *VisitController) TrackVisit(ip string, userAgent string, projectType enum.ProjectType) error {
	if c.DB == nil {
		return fmt.Errorf("db not initialized")
	}

	if !projectType.IsValid() {
		return nil
	}

	normalizedIP := strings.TrimSpace(ip)
	if normalizedIP == "" || isPrivateIP(normalizedIP) {
		return nil
	}

	normalizedUserAgent := strings.TrimSpace(userAgent)

	query := bson.M{
		"ip":           normalizedIP,
		"project_type": string(projectType),
	}

	existingVisit, err := c.DB.FindOne(query, c.GetCollectionName())
	if err == nil && existingVisit != nil {
		return nil
	}

	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return fmt.Errorf("visit lookup failed: %w", err)
	}

	country, countryCode := lookupIPCountry(normalizedIP)
	now := time.Now()
	visit := model.Visit{
		IP:          normalizedIP,
		Country:     country,
		CountryCode: countryCode,
		ProjectType: string(projectType),
		UserAgent:   normalizedUserAgent,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if _, err = c.DB.Create(visit, c.GetCollectionName()); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil
		}

		return fmt.Errorf("visit insert failed: %w", err)
	}

	return nil
}

func (c *VisitController) ListVisits(r *http.Request) (map[string]interface{}, error) {
	page := 1
	if pageValue := strings.TrimSpace(r.URL.Query().Get("page")); pageValue != "" {
		parsedPage, err := strconv.Atoi(pageValue)
		if err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	query := bson.M{}
	projectType := enum.ParseProjectType(r.URL.Query().Get("project"))
	if projectType.IsValid() {
		query["project_type"] = string(projectType)
	}

	totalPages, currentPage, visits, err := c.DB.FindAllWithPagination(query, page, c.GetCollectionName())
	if err != nil {
		return nil, fmt.Errorf("find visits failed: %w", err)
	}

	return map[string]interface{}{
		"visits":       visits,
		"current_page": currentPage,
		"total_pages":  totalPages,
	}, nil
}

func GetClientIP(r *http.Request) string {
	forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	realIP := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if realIP != "" {
		return realIP
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}

	return strings.TrimSpace(r.RemoteAddr)
}

func GetUserAgent(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("User-Agent"))
}

func isPrivateIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return true
	}

	if parsedIP.IsLoopback() || parsedIP.IsPrivate() {
		return true
	}

	return false
}

func lookupIPCountry(ip string) (string, string) {
	type ipAPIResponse struct {
		Status      string `json:"status"`
		Country     string `json:"country"`
		CountryCode string `json:"countryCode"`
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,country,countryCode", ip)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("VisitController | lookupIPCountry | request creation failed: %v", err)
		return "Unknown", "XX"
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Printf("VisitController | lookupIPCountry | api call failed: %v", err)
		return "Unknown", "XX"
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		log.Printf("VisitController | lookupIPCountry | bad status code: %d", response.StatusCode)
		return "Unknown", "XX"
	}

	apiResponse := ipAPIResponse{}
	if err := json.NewDecoder(response.Body).Decode(&apiResponse); err != nil {
		log.Printf("VisitController | lookupIPCountry | decode failed: %v", err)
		return "Unknown", "XX"
	}

	if apiResponse.Status != "success" {
		return "Unknown", "XX"
	}

	if apiResponse.Country == "" {
		apiResponse.Country = "Unknown"
	}
	if apiResponse.CountryCode == "" {
		apiResponse.CountryCode = "XX"
	}

	return apiResponse.Country, apiResponse.CountryCode
}
