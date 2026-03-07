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
	"time"
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
		errStr := strings.ToLower(err.Error())
		if h.llmService.IsCreditExhaustedError(err) {
			log.Printf("LLM credits exhausted; using static template fallback commentary")
		} else if strings.Contains(errStr, "status 429") || strings.Contains(errStr, "rate limit") {
			log.Printf("LLM rate-limited; using static template fallback commentary")
		} else {
			log.Printf("Failed to generate commentary: %v", err)
		}

		commentary = formatStaticTemplateFallbackCommentary(eventType, data)
		if strings.TrimSpace(commentary) == "" {
			commentary = formatFallbackCommentary(eventType, data)
		}
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
	case "TEAM_MILESTONE":
		prompt = fmt.Sprintf("You are a professional cricket commentator. The batting team has reached a team-score milestone. Details: %s. Generate an energetic 1-2 sentence team-focused commentary mentioning momentum and scoreboard pressure.", payload)
	case "CHASE_UPDATE":
		prompt = fmt.Sprintf("You are a professional cricket commentator. This is a live run chase update: %s. Generate a concise 1-2 sentence pressure-focused commentary. If runs needed are less than balls or within 10%% gap, highlight that momentum.", payload)
	case "MATCH_WON":
		prompt = fmt.Sprintf("You are a professional cricket commentator. The match has finished with this result: %s. Generate a celebratory 1-2 sentence match summary mentioning the winning margin.", payload)
	case "RUNS":
		prompt = fmt.Sprintf("You are a professional cricket commentator. %s. Generate a brief commentary.", payload)
	default:
		return payload, nil
	}

	//append a dummy text to prompt
	prompt += " Throw in some humor and sarcasm as well. We need to entertain the users on discord, also with some hard hitting facts."
	
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

func formatStaticTemplateFallbackCommentary(eventType string, data map[string]interface{}) string {
	payload, _ := data["payload"].(string)
	matchData, _ := data["match_data"].(map[string]interface{})

	getString := func(key string) string {
		if matchData == nil {
			return ""
		}
		if v, ok := matchData[key].(string); ok {
			return v
		}
		return ""
	}
	getInt := func(key string) int {
		if matchData == nil {
			return 0
		}
		switch v := matchData[key].(type) {
		case int:
			return v
		case int32:
			return int(v)
		case int64:
			return int(v)
		case float64:
			return int(v)
		default:
			return 0
		}
	}

	batsmanName := getString("batsman_name")
	bowlerName := getString("bowler_name")
	milestoneType := getString("milestone_type")
	milestoneRuns := getInt("milestone_runs")
	teamMilestoneRuns := getInt("team_milestone_runs")
	totalRuns := getInt("total_runs")
	wickets := getInt("wickets")
	needRuns := getInt("need_runs")
	needBalls := getInt("need_balls")
	matchWinner := getString("match_winner")
	matchWinType := getString("match_win_type")
	matchWinMargin := getInt("match_win_margin")

	templates := map[string][]string{
		"BOUNDARY_SIX": {
			"SIX! **%s** clears the ropes with authority.",
			"That is massive from **%s**. Six more.",
			"Crowd erupts as **%s** sends it into the stands.",
		},
		"BOUNDARY_FOUR": {
			"FOUR! **%s** times that beautifully.",
			"Threaded through the gap by **%s** for four.",
			"Classy boundary from **%s**.",
		},
		"WICKET": {
			"WICKET! %s strikes and removes %s.",
			"Breakthrough for %s. %s has to walk back.",
			"Big moment. %s gets %s out.",
		},
		"MILESTONE": {
			"Milestone moment. **%s** brings up %d (%s).",
			"Outstanding batting by **%s**. %d up, and counting (%s).",
			"Landmark alert for **%s**. %d reached (%s).",
			"What a knock from **%s**. %d reached in style (%s).",
			"Raise the bat for **%s**. %d completed (%s).",
		},
		"TEAM_MILESTONE": {
			"Team milestone up: **%d** on the board (%d/%d).",
			"Scoreboard pressure building, the batting side reaches **%d** (%d/%d).",
			"Another landmark for the innings: **%d** reached (%d/%d).",
			"Runs keep flowing. Team brings up **%d** at %d/%d.",
			"Big team effort: **%d** posted at %d/%d.",
			"Momentum with the batting side as they hit **%d** (%d/%d).",
			"The innings touches **%d** with the board at %d/%d.",
		},
		"CHASE_UPDATE": {
			"Chase update: **%d** needed from **%d** balls (%s).",
			"Pressure check: **%d** from **%d**. Situation: %s.",
			"Equation now **%d** off **%d** balls, %s.",
			"Run chase alive: **%d** to get from **%d** deliveries (%s).",
			"Scoreboard says **%d** required in **%d** balls. %s.",
			"Chase math: **%d** from **%d**. %s.",
		},
		"MATCH_WON": {
			"Match result: **%s** won by **%d %s**.",
			"Full-time: **%s** seal it by **%d %s**.",
			"Game over. **%s** take the match by **%d %s**.",
		},
	}

	options := templates[eventType]
	if len(options) == 0 {
		return ""
	}

	idx := int(time.Now().UnixNano() % int64(len(options)))
	if idx < 0 {
		idx = -idx
	}
	chosen := options[idx]

	switch eventType {
	case "BOUNDARY_SIX", "BOUNDARY_FOUR":
		if batsmanName == "" {
			batsmanName = "the batter"
		}
		return fmt.Sprintf(chosen, batsmanName)
	case "WICKET":
		if bowlerName == "" {
			bowlerName = "the bowler"
		}
		if batsmanName == "" {
			batsmanName = "the batter"
		}
		return fmt.Sprintf(chosen, bowlerName, batsmanName)
	case "MILESTONE":
		if batsmanName == "" {
			batsmanName = "the batter"
		}
		if milestoneType == "" {
			milestoneType = "milestone"
		}
		if milestoneRuns == 0 {
			milestoneRuns = getInt("batsman_runs")
		}
		if milestoneRuns == 0 {
			milestoneRuns = 50
		}
		return fmt.Sprintf(chosen, batsmanName, milestoneRuns, milestoneType)
	case "TEAM_MILESTONE":
		if teamMilestoneRuns == 0 {
			teamMilestoneRuns = getCrossedTeamMilestone(totalRuns)
		}
		if teamMilestoneRuns == 0 {
			teamMilestoneRuns = 50
		}
		return fmt.Sprintf(chosen, teamMilestoneRuns, totalRuns, wickets)
	case "CHASE_UPDATE":
		if needRuns <= 0 || needBalls <= 0 {
			return ""
		}
		return fmt.Sprintf(chosen, needRuns, needBalls, getChasePressureTag(needRuns, needBalls))
	case "MATCH_WON":
		if matchWinner == "" || matchWinMargin <= 0 || matchWinType == "" {
			return ""
		}
		return fmt.Sprintf(chosen, matchWinner, matchWinMargin, strings.ToLower(matchWinType))
	}

	if strings.TrimSpace(payload) != "" {
		return payload
	}
	return ""
}
func formatFallbackCommentary(eventType string, data map[string]interface{}) string {
	payload, _ := data["payload"].(string)
	matchData, _ := data["match_data"].(map[string]interface{})

	getString := func(key string) string {
		if matchData == nil {
			return ""
		}
		if v, ok := matchData[key].(string); ok {
			return v
		}
		return ""
	}
	getInt := func(key string) int {
		if matchData == nil {
			return 0
		}
		switch v := matchData[key].(type) {
		case int:
			return v
		case int32:
			return int(v)
		case int64:
			return int(v)
		case float64:
			return int(v)
		default:
			return 0
		}
	}
	getFloat := func(key string) float64 {
		if matchData == nil {
			return 0
		}
		switch v := matchData[key].(type) {
		case float32:
			return float64(v)
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		default:
			return 0
		}
	}

	batsmanName := getString("batsman_name")
	batsmanLeft := getString("batsman_left")
	batsmanRight := getString("batsman_right")
	bowlerName := getString("bowler_name")
	dismissalBowler := getString("dismissal_bowler")
	dismissalFielder := getString("dismissal_fielder")
	dismissalType := getString("dismissal_type")
	totalRuns := getInt("total_runs")
	wickets := getInt("wickets")
	overs := getFloat("overs")
	deliverySpeed := getString("delivery_speed")
	matchWinner := getString("match_winner")
	matchWinType := getString("match_win_type")
	matchWinMargin := getInt("match_win_margin")

	scoreSuffix := ""
	if totalRuns != 0 || wickets != 0 {
		scoreSuffix = fmt.Sprintf(" (%d/%d", totalRuns, wickets)
		if overs != 0 {
			scoreSuffix += fmt.Sprintf(", %.1f ov", overs)
		}
		scoreSuffix += ")"
	}
	if deliverySpeed != "" {
		scoreSuffix += fmt.Sprintf(" [%s]", deliverySpeed)
	}

	switch eventType {
	case "BOUNDARY_SIX":
		if batsmanName != "" {
			return fmt.Sprintf("SIX! **%s** launches it into the stands%s", batsmanName, scoreSuffix)
		}
		return fmt.Sprintf("SIX!%s", scoreSuffix)
	case "BOUNDARY_FOUR":
		if batsmanName != "" {
			return fmt.Sprintf("FOUR! **%s** finds the boundary%s", batsmanName, scoreSuffix)
		}
		return fmt.Sprintf("FOUR!%s", scoreSuffix)
	case "WICKET":
		if batsmanName != "" && bowlerName != "" {
			return fmt.Sprintf("WICKET! %s is out, %s strikes%s", batsmanName, bowlerName, scoreSuffix)
		}
		if batsmanName != "" {
			return fmt.Sprintf("WICKET! %s is out%s", batsmanName, scoreSuffix)
		}
		return payload
	case "BATSMAN_DEPART":
		name := batsmanName
		if name == "" {
			name = payload
		}
		detail := ""
		if dismissalType != "" {
			detail = dismissalType
		}
		if dismissalBowler != "" {
			if detail != "" {
				detail += " "
			}
			detail += fmt.Sprintf("(b %s", dismissalBowler)
			if dismissalFielder != "" {
				detail += fmt.Sprintf(", f %s", dismissalFielder)
			}
			detail += ")"
		}
		if detail != "" {
			return fmt.Sprintf("Wicket! %s departs %s%s", name, detail, scoreSuffix)
		}
		if strings.TrimSpace(payload) != "" {
			return payload
		}
		return fmt.Sprintf("Wicket! %s is dismissed%s", name, scoreSuffix)
	case "BATSMAN_ARRIVE":
		name := batsmanName
		if name == "" {
			if batsmanLeft != "" && batsmanRight == "" {
				name = batsmanLeft
			} else if batsmanRight != "" && batsmanLeft == "" {
				name = batsmanRight
			}
		}
		careerMatches := getInt("career_matches")
		careerRuns := getInt("career_runs")
		careerAvg := getFloat("career_average")
		if name != "" {
			if careerMatches != 0 || careerRuns != 0 || careerAvg != 0 {
				avgStr := ""
				if careerAvg != 0 {
					avgStr = fmt.Sprintf(", avg %.2f", careerAvg)
				}
				return fmt.Sprintf("New batter: **%s** (%d matches, %d runs%s)%s", name, careerMatches, careerRuns, avgStr, scoreSuffix)
			}
			return fmt.Sprintf("New batter: **%s** walks in%s", name, scoreSuffix)
		}
		return payload
	case "BOWLER_ARRIVE":
		name := bowlerName
		careerMatches := getInt("career_matches")
		bowlerWickets := getInt("bowler_wickets")
		if name != "" {
			if careerMatches != 0 || bowlerWickets != 0 {
				return fmt.Sprintf("Bowling change: %s into the attack (%d wickets in %d matches)%s", name, bowlerWickets, careerMatches, scoreSuffix)
			}
			return fmt.Sprintf("Bowling change: %s into the attack%s", name, scoreSuffix)
		}
		return payload
	case "MILESTONE":
		mType := getString("milestone_type")
		mRuns := getInt("milestone_runs")
		if batsmanName != "" {
			if mType != "" && mRuns != 0 {
				return fmt.Sprintf("Milestone! **%s** brings up %d (%s)%s", batsmanName, mRuns, mType, scoreSuffix)
			}
			return fmt.Sprintf("Milestone! **%s** reaches a landmark%s", batsmanName, scoreSuffix)
		}
		return payload
	case "TEAM_MILESTONE":
		tRuns := getInt("team_milestone_runs")
		if tRuns == 0 {
			tRuns = getCrossedTeamMilestone(totalRuns)
		}
		if tRuns > 0 {
			return fmt.Sprintf("Team milestone! %d reached%s", tRuns, scoreSuffix)
		}
		if strings.TrimSpace(payload) != "" {
			return payload
		}
		return fmt.Sprintf("Team milestone reached%s", scoreSuffix)
	case "MATCH_WON":
		if matchWinner != "" && matchWinMargin > 0 && matchWinType != "" {
			return fmt.Sprintf("Match Result: **%s** won by **%d %s**%s", matchWinner, matchWinMargin, strings.ToLower(matchWinType), scoreSuffix)
		}
		if strings.TrimSpace(payload) != "" {
			return payload
		}
		return fmt.Sprintf("Match completed%s", scoreSuffix)
	case "RUNS":
		if strings.TrimSpace(payload) != "" {
			return payload
		}
		return fmt.Sprintf("Update%s", scoreSuffix)
	default:
		if strings.TrimSpace(payload) != "" {
			return payload
		}
		return eventType
	}
}

func getCrossedTeamMilestone(totalRuns int) int {
	if totalRuns < 50 {
		return 0
	}
	return (totalRuns / 50) * 50
}

func getChasePressureTag(needRuns, needBalls int) string {
	if needBalls <= 0 {
		return "no balls left"
	}
	if needRuns <= needBalls {
		return "ahead of the equation"
	}
	gapPct := (float64(needRuns-needBalls) / float64(needBalls)) * 100
	if gapPct <= 10 {
		return "within 10% gap"
	}
	return "behind the equation"
}
