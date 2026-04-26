package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DiscordNotifier handles sending notifications to Discord via webhooks
type DiscordNotifier struct {
	WebhookURL string
	HTTPClient *http.Client
}

// DiscordWebhookPayload represents the structure of a Discord webhook message
type DiscordWebhookPayload struct {
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Content   string         `json:"content,omitempty"`
	Embeds    []DiscordEmbed `json:"embeds,omitempty"`
}

// DiscordEmbed represents a Discord message embed
type DiscordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	URL         string              `json:"url,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
	Footer      *DiscordEmbedFooter `json:"footer,omitempty"`
	Image       *DiscordEmbedImage  `json:"image,omitempty"`
	Thumbnail   *DiscordEmbedImage  `json:"thumbnail,omitempty"`
	Author      *DiscordEmbedAuthor `json:"author,omitempty"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
}

type DiscordEmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

type DiscordEmbedImage struct {
	URL string `json:"url"`
}

type DiscordEmbedAuthor struct {
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// NewDiscordNotifier creates a new Discord notifier
func NewDiscordNotifier(webhookURL string) *DiscordNotifier {
	return &DiscordNotifier{
		WebhookURL: webhookURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send sends a generic payload to the Discord webhook
func (n *DiscordNotifier) Send(payload DiscordWebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal discord payload: %w", err)
	}

	req, err := http.NewRequest("POST", n.WebhookURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send discord notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord API returned status: %s", resp.Status)
	}

	return nil
}

// SendScreenshotAnalysis sends a formatted screenshot analysis alert
func (n *DiscordNotifier) SendScreenshotAnalysis(deviceName, appName, activity, summary, category, productivity string, confidence float64) error {
	color := 0x3498db // Default Blue

	switch category {
	case "development":
		color = 0x2ecc71 // Green
	case "gaming":
		color = 0x9b59b6 // Purple
	case "entertainment":
		color = 0xe67e22 // Orange
	case "browsing":
		color = 0x34495e // Dark Blue
	}

	payload := DiscordWebhookPayload{
		Username: "Phoenix Activity Tracker",
		Embeds: []DiscordEmbed{
			{
				Title:       fmt.Sprintf("Activity Detected: %s", appName),
				Description: summary,
				Color:       color,
				Timestamp:   time.Now().Format(time.RFC3339),
				Fields: []DiscordEmbedField{
					{Name: "Device", Value: deviceName, Inline: true},
					{Name: "Activity", Value: activity, Inline: true},
					{Name: "Category", Value: category, Inline: true},
					{Name: "Productivity", Value: productivity, Inline: true},
					{Name: "Confidence", Value: fmt.Sprintf("%.2f", confidence), Inline: true},
				},
				Footer: &DiscordEmbedFooter{
					Text: "Project Phoenix v2",
				},
			},
		},
	}

	return n.Send(payload)
}

// SendCricketEvent sends a formatted cricket event alert
func (n *DiscordNotifier) SendCricketEvent(eventType, commentary string, matchData map[string]interface{}) error {
	style := cricketEventStyleFor(eventType)
	title := fmt.Sprintf("%s %s", style.Badge, style.Label)

	scoreLine := buildScoreLine(matchData)
	description := strings.TrimSpace(commentary)
	if description == "" {
		description = fallbackCricketDescription(eventType, matchData)
	}

	if scoreLine != "" {
		description = fmt.Sprintf("`%s`\n\n%s", scoreLine, description)
	}

	fields := buildCricketFields(eventType, matchData)

	payload := DiscordWebhookPayload{
		Username: "Cricket24 Bot",
		Embeds: []DiscordEmbed{
			{
				Title:       title,
				Description: strings.TrimSpace(description),
				Color:       style.Color,
				Timestamp:   time.Now().Format(time.RFC3339),
				Fields:      fields,
				Footer: &DiscordEmbedFooter{
					Text: "Cricket 24 Live Tracker | Phoenix",
				},
			},
		},
	}

	return n.Send(payload)
}

type cricketEventStyle struct {
	Color int
	Label string
	Badge string
}

func cricketEventStyleFor(eventType string) cricketEventStyle {
	switch eventType {
	case "BOUNDARY_SIX":
		return cricketEventStyle{Color: 0xe74c3c, Label: "Six", Badge: "[SIX]"}
	case "BOUNDARY_FOUR":
		return cricketEventStyle{Color: 0x2ecc71, Label: "Four", Badge: "[FOUR]"}
	case "WICKET", "BATSMAN_DEPART":
		return cricketEventStyle{Color: 0x34495e, Label: "Wicket", Badge: "[OUT]"}
	case "BATSMAN_ARRIVE":
		return cricketEventStyle{Color: 0x3498db, Label: "New Batter", Badge: "[BAT]"}
	case "BOWLER_ARRIVE":
		return cricketEventStyle{Color: 0x1abc9c, Label: "New Bowler", Badge: "[BOWL]"}
	case "MILESTONE":
		return cricketEventStyle{Color: 0xf39c12, Label: "Milestone", Badge: "[MILESTONE]"}
	case "TEAM_MILESTONE":
		return cricketEventStyle{Color: 0xf1c40f, Label: "Team Milestone", Badge: "[TEAM]"}
	case "CHASE_UPDATE":
		return cricketEventStyle{Color: 0x2980b9, Label: "Chase Update", Badge: "[CHASE]"}
	case "MATCH_WON":
		return cricketEventStyle{Color: 0x27ae60, Label: "Match Result", Badge: "[WIN]"}
	case "RUNS":
		return cricketEventStyle{Color: 0x95a5a6, Label: "Runs", Badge: "[RUNS]"}
	default:
		return cricketEventStyle{Color: 0x95a5a6, Label: eventType, Badge: "[CRICKET]"}
	}
}

func buildScoreLine(matchData map[string]interface{}) string {
	if matchData == nil {
		return ""
	}

	wickets, hasWickets := toInt(matchData["wickets"])
	totalRuns, hasRuns := toInt(matchData["total_runs"])
	overs, hasOvers := toFloat(matchData["overs"])

	if !hasWickets || !hasRuns {
		return ""
	}

	score := fmt.Sprintf("SCORE %d/%d", wickets, totalRuns)
	if hasOvers {
		score += fmt.Sprintf(" | %.1f ov", overs)
	}
	return score
}

func buildCricketFields(eventType string, matchData map[string]interface{}) []DiscordEmbedField {
	if matchData == nil {
		return nil
	}

	fields := make([]DiscordEmbedField, 0, 6)

	if batsman := toStr(matchData["batsman_name"]); batsman != "" {
		fields = append(fields, DiscordEmbedField{Name: "Batter", Value: batsman, Inline: true})
	}
	if bowler := firstNonEmpty(toStr(matchData["bowler_name"]), toStr(matchData["dismissal_bowler"])); bowler != "" {
		fields = append(fields, DiscordEmbedField{Name: "Bowler", Value: bowler, Inline: true})
	}
	if speed := toStr(matchData["delivery_speed"]); speed != "" {
		fields = append(fields, DiscordEmbedField{Name: "Speed", Value: speed, Inline: true})
	}

	if needRuns, okRuns := toInt(matchData["need_runs"]); okRuns {
		if needBalls, okBalls := toInt(matchData["need_balls"]); okBalls && needRuns > 0 && needBalls > 0 {
			fields = append(fields, DiscordEmbedField{
				Name:   "Equation",
				Value:  fmt.Sprintf("%d needed from %d balls", needRuns, needBalls),
				Inline: true,
			})
		}
	}

	if eventType == "MATCH_WON" {
		winner := toStr(matchData["match_winner"])
		margin, hasMargin := toInt(matchData["match_win_margin"])
		unit := toStr(matchData["match_win_type"])
		if winner != "" && hasMargin && unit != "" {
			fields = append(fields, DiscordEmbedField{
				Name:   "Result",
				Value:  fmt.Sprintf("%s won by %d %s", winner, margin, strings.ToLower(unit)),
				Inline: false,
			})
		}
	}

	return fields
}

func fallbackCricketDescription(eventType string, matchData map[string]interface{}) string {
	if matchData == nil {
		return eventType
	}

	switch eventType {
	case "MATCH_WON":
		winner := toStr(matchData["match_winner"])
		margin, hasMargin := toInt(matchData["match_win_margin"])
		unit := toStr(matchData["match_win_type"])
		if winner != "" && hasMargin && unit != "" {
			return fmt.Sprintf("%s won by %d %s.", winner, margin, strings.ToLower(unit))
		}
	case "CHASE_UPDATE":
		needRuns, hasRuns := toInt(matchData["need_runs"])
		needBalls, hasBalls := toInt(matchData["need_balls"])
		if hasRuns && hasBalls && needRuns > 0 && needBalls > 0 {
			return fmt.Sprintf("Chase equation: %d required from %d balls.", needRuns, needBalls)
		}
	}

	if payload := toStr(matchData["payload"]); payload != "" {
		return payload
	}
	return eventType
}

func toStr(v interface{}) string {
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	default:
		return ""
	}
}

func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int32:
		return int(val), true
	case int64:
		return int(val), true
	case float32:
		return int(val), true
	case float64:
		return int(val), true
	case string:
		if strings.TrimSpace(val) == "" {
			return 0, false
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(val))
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float32:
		return float64(val), true
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		if strings.TrimSpace(val) == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// SendSystemAlert sends a formatted system alert
func (n *DiscordNotifier) SendSystemAlert(title, message, level string) error {
	color := 0x95a5a6 // Gray

	switch level {
	case "info":
		color = 0x3498db // Blue
	case "success":
		color = 0x2ecc71 // Green
	case "warning":
		color = 0xf1c40f // Yellow
	case "error":
		color = 0xe74c3c // Red
	}

	payload := DiscordWebhookPayload{
		Username: "Phoenix System",
		Embeds: []DiscordEmbed{
			{
				Title:       title,
				Description: message,
				Color:       color,
				Timestamp:   time.Now().Format(time.RFC3339),
				Footer: &DiscordEmbedFooter{
					Text: "System Monitor",
				},
			},
		},
	}

	return n.Send(payload)
}

// SendAPIKeyValidation sends a formatted notification when an API key is validated
func (n *DiscordNotifier) SendAPIKeyValidation(provider, status string, credits map[string]interface{}, stats map[string]int, webScraperURL string) error {
	color := 0x95a5a6 // Gray
	badge := "🔑"
	statusLabel := status

	switch status {
	case "Valid":
		color = 0x2ecc71 // Green
		badge = "✅"
		statusLabel = "Valid Key Found"
	case "ValidNoCredits":
		color = 0xf39c12 // Orange
		badge = "⚠️"
		statusLabel = "Valid (No Credits)"
	case "Invalid":
		color = 0xe74c3c // Red
		badge = "❌"
		statusLabel = "Invalid Key"
	case "Error":
		color = 0x95a5a6 // Gray
		badge = "⚡"
		statusLabel = "Validation Error"
	}

	title := fmt.Sprintf("%s %s API Key - %s", badge, provider, statusLabel)

	description := fmt.Sprintf("A %s API key has been validated.", provider)

	// Add credits info for OpenRouter keys
	if credits != nil && len(credits) > 0 {
		if totalCredits, ok := credits["total_credits"].(float64); ok {
			if totalUsage, ok := credits["total_usage"].(float64); ok {
				description += fmt.Sprintf("\n\n💰 **Credits:** $%.2f available | $%.2f used", totalCredits, totalUsage)
			}
		}
	}

	// Add web scraper URL if provided
	if webScraperURL != "" {
		description += fmt.Sprintf("\n\n🌐 **All keys available at:** %s", webScraperURL)
	}

	fields := []DiscordEmbedField{
		{Name: "Provider", Value: provider, Inline: true},
		{Name: "Status", Value: status, Inline: true},
	}

	// Add stats if provided
	if stats != nil {
		if processed, ok := stats["processed"]; ok {
			fields = append(fields, DiscordEmbedField{
				Name:   "Total Processed",
				Value:  fmt.Sprintf("%d keys", processed),
				Inline: true,
			})
		}
		if valid, ok := stats["valid"]; ok {
			fields = append(fields, DiscordEmbedField{
				Name:   "Valid Keys",
				Value:  fmt.Sprintf("%d keys", valid),
				Inline: true,
			})
		}
		if invalid, ok := stats["invalid"]; ok {
			fields = append(fields, DiscordEmbedField{
				Name:   "Invalid Keys",
				Value:  fmt.Sprintf("%d keys", invalid),
				Inline: true,
			})
		}
	}

	payload := DiscordWebhookPayload{
		Username:  "Phoenix Key Verifier",
		AvatarURL: "https://cdn.discordapp.com/emojis/1234567890.png", // Optional: Add a custom avatar
		Embeds: []DiscordEmbed{
			{
				Title:       title,
				Description: description,
				Color:       color,
				Timestamp:   time.Now().Format(time.RFC3339),
				Fields:      fields,
				Footer: &DiscordEmbedFooter{
					Text: "Project Phoenix v2 | Key Validator",
				},
			},
		},
	}

	return n.Send(payload)
}
