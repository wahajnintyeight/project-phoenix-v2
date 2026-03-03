package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
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
	color := 0xf1c40f // Yellow
	emoji := "📊"
	title := eventType

	// Format event type and set colors/emojis
	switch eventType {
	case "BOUNDARY_SIX":
		color = 0xe74c3c // Red
		emoji = "🚀"
		title = "Six!"
	case "BOUNDARY_FOUR":
		color = 0x2ecc71 // Green
		emoji = "🏏"
		title = "Four!"
	case "WICKET":
		color = 0x34495e // Dark
		emoji = "💥"
		title = "Wicket!"
	case "BATSMAN_ARRIVE":
		color = 0x3498db // Blue
		emoji = "🏃"
		title = "New Batsman"
	case "BOWLER_ARRIVE":
		color = 0x1abc9c // Teal
		emoji = "⚡"
		title = "New Bowler"
	case "BATSMAN_DEPART":
		color = 0x9b59b6 // Purple
		emoji = "🎯"
		title = "Wicket!"
	case "MILESTONE":
		color = 0xf39c12 // Orange/Gold
		emoji = "🎉"
		title = "Milestone!"
	case "RUNS":
		color = 0x95a5a6 // Gray
		emoji = "✅"
		title = "Runs"
	}

	// Build description with proper formatting
	description := commentary

	// Extract key information for better formatting
	if matchData != nil {
		// For BOWLER_ARRIVE - highlight the bowler name
		if eventType == "BOWLER_ARRIVE" {
			if bowlerName, ok := matchData["bowler_name"].(string); ok && bowlerName != "" {
				careerInfo := ""
				if matches, ok := matchData["career_matches"].(float64); ok {
					wickets, _ := matchData["bowler_wickets"].(float64)
					avg, _ := matchData["career_average"].(float64)
					careerInfo = fmt.Sprintf("\n\n**Career Stats:** %d matches • %d wickets • avg %.2f", int(matches), int(wickets), avg)
				}
				// Only override if commentary is empty or generic
				if commentary == "" {
					description = fmt.Sprintf("**%s** comes into the attack%s", bowlerName, careerInfo)
				}
			}
		}

		// For BATSMAN_ARRIVE - highlight the batsman name
		if eventType == "BATSMAN_ARRIVE" {
			if batsmanName, ok := matchData["batsman_name"].(string); ok && batsmanName != "" {
				careerInfo := ""
				if matches, ok := matchData["career_matches"].(float64); ok {
					runs, _ := matchData["career_runs"].(float64)
					avg, _ := matchData["career_average"].(float64)
					careerInfo = fmt.Sprintf("\n\n**Career Stats:** %d matches • %d runs • avg %.2f", int(matches), int(runs), avg)
				}
				// Only override if commentary is empty or generic
				if commentary == "" {
					description = fmt.Sprintf("**%s** walks to the crease%s", batsmanName, careerInfo)
				}
			}
		}

		// For BATSMAN_DEPART - highlight bowler and batsman
		if eventType == "BATSMAN_DEPART" {
			batsmanName, _ := matchData["batsman_name"].(string)
			bowlerName, _ := matchData["dismissal_bowler"].(string)
			runs, _ := matchData["batsman_runs"].(float64)
			balls, _ := matchData["batsman_balls"].(float64)
			strikeRate, _ := matchData["batsman_strike_rate"].(float64)
			dismissalType, _ := matchData["dismissal_type"].(string)
			fielderName, _ := matchData["dismissal_fielder"].(string)

			// Only override if commentary is empty
			if commentary == "" {
				if bowlerName != "" {
					description = fmt.Sprintf("**%s** strikes! 🎯\n\n", bowlerName)
				} else {
					description = ""
				}

				if batsmanName != "" {
					description += fmt.Sprintf("**%s** departs", batsmanName)
					if runs > 0 || balls > 0 {
						description += fmt.Sprintf(" - %d(%d)", int(runs), int(balls))
						if strikeRate > 0 {
							description += fmt.Sprintf(" SR: %.1f", strikeRate)
						}
					}
					description += "\n"
				}

				if dismissalType != "" {
					dismissalText := dismissalType
					if fielderName != "" && dismissalType == "caught" {
						dismissalText = fmt.Sprintf("caught by %s", fielderName)
					}
					if bowlerName != "" {
						dismissalText += fmt.Sprintf(", bowled by %s", bowlerName)
					}
					description += fmt.Sprintf("\n*%s*", dismissalText)
				}
			}
		}

		// For MILESTONE - keep the full commentary
		if eventType == "MILESTONE" {
			// Commentary is already the full text from LLM, keep it as is
			// Just ensure we have the basic info if commentary is empty
			if commentary == "" {
				batsmanName, _ := matchData["batsman_name"].(string)
				milestoneType, _ := matchData["milestone_type"].(string)
				runs, _ := matchData["milestone_runs"].(float64)
				balls, _ := matchData["batsman_balls"].(float64)
				strikeRate, _ := matchData["batsman_strike_rate"].(float64)

				if batsmanName != "" && milestoneType != "" {
					description = fmt.Sprintf("🎉 **%s** reaches his **%s**! 🎉\n\n", batsmanName, milestoneType)
					if runs > 0 && balls > 0 {
						description += fmt.Sprintf("%d* runs off %d balls", int(runs), int(balls))
						if strikeRate > 0 {
							description += fmt.Sprintf(" (SR: %.1f)", strikeRate)
						}
					}
				}
			}
		}
	}

	fields := []DiscordEmbedField{}

	// Add match score if available
	if matchData != nil {
		wickets, hasWickets := matchData["wickets"].(float64)
		totalRuns, hasRuns := matchData["total_runs"].(float64)
		overs, hasOvers := matchData["overs"].(float64)

		if hasWickets && hasRuns {
			scoreStr := fmt.Sprintf("%d/%d", int(wickets), int(totalRuns))
			if hasOvers && overs > 0 {
				scoreStr += fmt.Sprintf(" (%v ov)", overs)
			}
			fields = append(fields, DiscordEmbedField{Name: "Score", Value: scoreStr, Inline: true})
		}
	}

	payload := DiscordWebhookPayload{
		Username: "Cricket24 Bot",
		Embeds: []DiscordEmbed{
			{
				Title:       fmt.Sprintf("%s %s", emoji, title),
				Description: description,
				Color:       color,
				Timestamp:   time.Now().Format(time.RFC3339),
				Fields:      fields,
				Footer: &DiscordEmbedFooter{
					Text: "Cricket 24 Live Tracker",
				},
			},
		},
	}

	return n.Send(payload)
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
