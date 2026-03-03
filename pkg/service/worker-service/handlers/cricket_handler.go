package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"log"
	"os"
	"project-phoenix/v2/internal/model"
	"project-phoenix/v2/internal/notifier"
	"project-phoenix/v2/internal/service"
	"strings"
)

type CricketHandler struct {
	llmService *service.LLMService
	notifier   *notifier.DiscordNotifier
}

type CricketEvent struct {
	Type      string                 `json:"type"`
	Payload   string                 `json:"payload"`
	Raw       string                 `json:"raw"`
	MatchData map[string]interface{} `json:"match_data,omitempty"`
}

type CricketImagePayload struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	ImageData []byte `json:"image_data"`
}

func NewCricketHandler(notifier *notifier.DiscordNotifier) *CricketHandler {
	return &CricketHandler{
		llmService: service.NewLLMService(),
		notifier:   notifier,
	}
}

// Process handles cricket events from the queue
func (h *CricketHandler) Process(data map[string]interface{}) error {
	log.Println("Processing cricket event:", data)

	// Beautify names in match data if present
	if matchData, ok := data["match_data"].(map[string]interface{}); ok {
		nameFields := []string{"batsman_name", "bowler_name", "batsman_left", "batsman_right", "dismissal_bowler", "dismissal_fielder"}
		for _, field := range nameFields {
			if val, ok := matchData[field].(string); ok && val != "" {
				matchData[field] = beautifyName(val)
			}
		}
	}

	// Parse event type
	eventType, ok := data["type"].(string)
	if !ok {
		return fmt.Errorf("invalid event type")
	}

	// Check if this is an image payload for LLM OCR
	if eventType == "CRICKET_SCOREBOARD" {
		return h.processScoreboardImage(data)
	}

	// Handle regular cricket events (from local OCR)
	// Generate colorful commentary using LLM
	commentary, err := h.generateCommentary(eventType, data)
	if err != nil {
		log.Printf("Failed to generate commentary: %v", err)
		// Fallback to basic message
		payload, _ := data["payload"].(string)
		commentary = payload
	}

	// Publish to Discord
	if err := h.publishToDiscord(eventType, commentary, data); err != nil {
		return fmt.Errorf("failed to publish to Discord: %w", err)
	}

	log.Printf("Cricket event processed: %s - %s", eventType, commentary)
	return nil
}

// processScoreboardImage handles LLM OCR mode - analyzes scoreboard image with vision model
func (h *CricketHandler) processScoreboardImage(data map[string]interface{}) error {
	log.Println("Processing scoreboard image with LLM OCR")

	// Extract image data
	imageData, ok := data["image_data"].([]byte)
	if !ok {
		// Try to decode from interface{} array
		if imageArray, ok := data["image_data"].([]interface{}); ok {
			imageData = make([]byte, len(imageArray))
			for i, v := range imageArray {
				if b, ok := v.(float64); ok {
					imageData[i] = byte(b)
				}
			}
		} else {
			return fmt.Errorf("invalid image_data format")
		}
	}

	// Decode PNG image
	img, err := png.Decode(bytes.NewReader(imageData))
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	log.Printf("Decoded scoreboard image: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())

	// Encode image to base64 for LLM
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return fmt.Errorf("failed to encode image: %w", err)
	}
	base64Image := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Analyze with vision model
	scoreboardData, err := h.analyzeScoreboardWithLLM(base64Image)
	if err != nil {
		return fmt.Errorf("failed to analyze scoreboard: %w", err)
	}

	log.Printf("Scoreboard analysis: %+v", scoreboardData)

	// Detect events from scoreboard data
	// TODO: Implement event detection logic based on previous state
	// For now, just log the extracted data

	return nil
}

// analyzeScoreboardWithLLM uses OpenRouter vision model to extract scoreboard data
func (h *CricketHandler) analyzeScoreboardWithLLM(base64Image string) (map[string]interface{}, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY not configured")
	}

	// Use a vision-capable model (e.g., GPT-4 Vision, Claude 3)
	visionModel := getEnvOrDefault("OPENROUTER_VISION_MODEL", "anthropic/claude-3.5-sonnet")

	prompt := `Analyze this Cricket 24 game scoreboard image and extract the following information in JSON format:

{
  "team_score": {
    "wickets": <number>,
    "runs": <number>,
    "overs": <number>
  },
  "batsman": {
    "name": "<string>",
    "runs": <number>,
    "balls": <number>
  },
  "bowler": {
    "name": "<string>",
    "overs": <number>,
    "economy": "<string>"
  },
  "delivery_speed": "<string>",
  "event_type": "<BOUNDARY_SIX|BOUNDARY_FOUR|WICKET|RUNS|NONE>"
}

Extract all visible information. If any field is not visible, use null.`

	// Create chat completion request with image
	req := model.ChatCompletionRequest{
		Provider:    "openrouter",
		Model:       visionModel,
		APIKey:      apiKey,
		MaxTokens:   500,
		Temperature: 0.1, // Low temperature for accurate extraction
		Messages: []model.ChatMessage{
			{
				Role:    "user",
				Content: prompt,
				// Note: Image handling depends on the gollm library's support for vision
				// You may need to modify this based on the library's API
			},
		},
	}

	// Send request
	response, err := h.llmService.SendChatCompletion(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM response: %w", err)
	}

	// Parse JSON response
	var scoreboardData map[string]interface{}
	if err := json.Unmarshal([]byte(response.Message.Content), &scoreboardData); err != nil {
		// Try to extract JSON from response
		jsonStr := extractJSON(response.Message.Content)
		if err := json.Unmarshal([]byte(jsonStr), &scoreboardData); err != nil {
			return nil, fmt.Errorf("failed to parse scoreboard data: %w", err)
		}
	}

	return scoreboardData, nil
}

// generateCommentary uses LLM to create engaging cricket commentary
func (h *CricketHandler) generateCommentary(eventType string, data map[string]interface{}) (string, error) {
	var prompt string
	payload, _ := data["payload"].(string)
	raw, _ := data["raw"].(string)
	matchData, _ := data["match_data"].(map[string]interface{})

	batsmanName := ""
	bowlerName := ""
	if matchData != nil {
		batsmanName, _ = matchData["batsman_name"].(string)
		bowlerName, _ = matchData["bowler_name"].(string)
	}

	switch eventType {
	case "BOUNDARY_SIX":
		prompt = fmt.Sprintf("You are a professional cricket commentator. Batsman %s just hit a SIX! Score info: %s. Raw data: %s. Generate an exciting 1-2 sentence commentary. ALWAYS highlight the batsman's name in bold (e.g., **%s**) to glorify them. Do not highlight the bowler.", batsmanName, payload, raw, batsmanName)
	case "BOUNDARY_FOUR":
		prompt = fmt.Sprintf("You are a professional cricket commentator. Batsman %s just hit a FOUR! Score info: %s. Raw data: %s. Generate an exciting 1-2 sentence commentary. ALWAYS highlight the batsman's name in bold (e.g., **%s**) to glorify them. Do not highlight the bowler.", batsmanName, payload, raw, batsmanName)
	case "WICKET":
		prompt = fmt.Sprintf("You are a professional cricket commentator. A wicket just fell! Batsman: %s, Bowler: %s. Score info: %s. Generate a dramatic 1-2 sentence commentary.", batsmanName, bowlerName, payload)
	case "BATSMAN_DEPART":
		prompt = fmt.Sprintf("You are a professional cricket commentator. Batsman %s has been dismissed! Details: %s. Generate an exciting commentary that CELEBRATES the bowler %s's success and describes the dismissal. Keep it to 2-3 sentences.", batsmanName, payload, bowlerName)
	case "BATSMAN_ARRIVE":
		prompt = fmt.Sprintf("You are a professional cricket commentator. A new batsman %s is walking to the crease. Details: %s. Generate a welcoming commentary introducing the batsman with their career stats if available. Keep it to 2-3 sentences.", batsmanName, payload)
	case "BOWLER_ARRIVE":
		prompt = fmt.Sprintf("You are a professional cricket commentator. A new bowler %s is coming into the attack. Details: %s. Generate an exciting commentary introducing the bowler with their career stats and bowling style if available. Keep it to 2-3 sentences.", bowlerName, payload)
	case "MILESTONE":
		prompt = fmt.Sprintf("You are a professional cricket commentator. Batsman %s has reached a major milestone! Details: %s. Generate an exciting, celebratory commentary praising the batsman's achievement. Keep it to 2-3 sentences.", batsmanName, payload)
	case "RUNS":
		prompt = fmt.Sprintf("You are a professional cricket commentator. %s. Generate a brief commentary.", payload)
	default:
		return payload, nil
	}

	commentary, err := h.llmService.GenerateText(prompt)
	if err != nil {
		return "", err
	}

	return commentary, nil
}

// publishToDiscord sends the commentary to Discord webhook
func (h *CricketHandler) publishToDiscord(eventType, commentary string, data map[string]interface{}) error {
	if h.notifier == nil {
		log.Printf("[DISCORD-MOCK] %s: %s", eventType, commentary)
		return nil
	}

	matchData, _ := data["match_data"].(map[string]interface{})
	return h.notifier.SendCricketEvent(eventType, commentary, matchData)
}

// Helper functions

func beautifyName(name string) string {
	if name == "" {
		return ""
	}
	// Replace underscores and dashes with space
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")

	// Trim multiple spaces and leading/trailing spaces
	name = strings.Join(strings.Fields(name), " ")

	return name
}
