# ATS Score API Usage Example

## Calculate ATS Score for Resume

This endpoint calculates a non-biased ATS compatibility score by comparing a resume against a job description.

### Endpoint
**POST** `/api/gollm/chat/completions`

### Request with ATS_SCAN Type

```bash
curl -X POST http://localhost:8080/api/gollm/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ATS_SCAN",
    "provider": "openai",
    "apiKey": "your-api-key-here",
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "{\n  \"resume_text\": \"John Doe\\nSoftware Engineer\\n\\nEXPERIENCE:\\n- Senior Backend Engineer at Tech Corp (2020-2023)\\n  * Built microservices using Python, FastAPI, and PostgreSQL\\n  * Reduced API response time by 40% through optimization\\n  * Led team of 5 engineers in cloud migration to AWS\\n  * Implemented CI/CD pipeline using Docker and Kubernetes\\n\\nSKILLS:\\nPython, FastAPI, PostgreSQL, Docker, AWS, REST APIs, Redis\",\n  \"job_description\": \"We are looking for a Senior Backend Engineer with 3-5 years of experience.\\n\\nRequired Skills:\\n- Python (3+ years)\\n- FastAPI or Django\\n- PostgreSQL or MySQL\\n- Docker\\n- AWS (EC2, Lambda, S3)\\n- REST API design\\n- Kubernetes\\n\\nPreferred Skills:\\n- GraphQL\\n- Redis\\n- Microservices architecture\"\n}"
      }
    ],
    "temperature": 0.7,
    "maxTokens": 2000
  }'
```

**Note:** The `type` field uses the `PromptType` enum. Available values:
- `ATS_SCAN` - For ATS resume scoring (defined in `internal/model/gollm-prompts.go`)

### Request Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | Yes | Must be "ATS_SCAN" to trigger ATS scoring |
| `provider` | string | Yes | LLM provider (e.g., "openai", "anthropic") |
| `apiKey` | string | Yes | API key for the provider |
| `model` | string | Yes | Model to use (e.g., "gpt-4", "claude-3-opus") |
| `messages` | array | Yes | Array with user message containing resume and job description |
| `temperature` | float | No | Sampling temperature (default: 0.7) |
| `maxTokens` | int | No | Max tokens to generate (default: 2000 for ATS_SCAN) |

### User Message Format

The user message content should be a JSON string with:
- `resume_text`: Full resume text
- `job_description`: Full job description text

```json
{
  "resume_text": "Full resume content here...",
  "job_description": "Full job description here..."
}
```

### Expected Response

```json
{
  "code": 1022,
  "data": {
    "id": "uuid-string",
    "model": "gpt-4",
    "message": {
      "role": "assistant",
      "content": "{\n  \"overall_score\": 85,\n  \"breakdown\": {\n    \"keyword_match\": {\n      \"score\": 27,\n      \"max_score\": 30,\n      \"details\": \"Found 18 out of 20 key technical terms\"\n    },\n    \"experience_relevance\": {\n      \"score\": 22,\n      \"max_score\": 25,\n      \"details\": \"5 years experience matches 3-5 years requirement\"\n    },\n    \"technical_skills\": {\n      \"score\": 18,\n      \"max_score\": 20,\n      \"details\": \"Strong match on primary technologies\"\n    },\n    \"education\": {\n      \"score\": 10,\n      \"max_score\": 10,\n      \"details\": \"Bachelor's degree meets requirement\"\n    },\n    \"resume_quality\": {\n      \"score\": 13,\n      \"max_score\": 15,\n      \"details\": \"Well-structured with quantifiable achievements\"\n    }\n  },\n  \"matched_keywords\": [\n    \"Python\", \"FastAPI\", \"PostgreSQL\", \"Docker\", \"AWS\", \"REST API\", \"Redis\", \"Kubernetes\"\n  ],\n  \"missing_keywords\": [\n    \"GraphQL\"\n  ],\n  \"strengths\": [\n    \"Strong backend development experience with Python and FastAPI\",\n    \"Relevant cloud platform experience with AWS\",\n    \"Quantifiable achievements (40% performance improvement)\",\n    \"Leadership experience managing team of 5\"\n  ],\n  \"gaps\": [\n    \"No mention of GraphQL experience\"\n  ],\n  \"recommendations\": [\n    \"Add GraphQL to skills if you have experience with it\",\n    \"Highlight specific AWS services used (EC2, Lambda, S3)\",\n    \"Mention any API design patterns or best practices followed\"\n  ],\n  \"match_level\": \"Good match\"\n}"
    },
    "usage": {
      "promptTokens": 450,
      "completionTokens": 320,
      "totalTokens": 770
    },
    "createdAt": "2024-01-01T00:00:00Z"
  }
}
```

### Parsing the Response

The `message.content` field contains a JSON string that needs to be parsed:

```javascript
const response = await fetch('/api/gollm/chat/completions', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(requestBody)
});

const data = await response.json();
const atsScore = JSON.parse(data.data.message.content);

console.log('Overall Score:', atsScore.overall_score);
console.log('Match Level:', atsScore.match_level);
console.log('Strengths:', atsScore.strengths);
console.log('Gaps:', atsScore.gaps);
```

### Score Interpretation

| Score Range | Match Level | Description |
|-------------|-------------|-------------|
| 90-100 | Excellent match | Strong candidate for the role |
| 75-89 | Good match | Meets most requirements |
| 60-74 | Fair match | Meets some requirements, gaps exist |
| 45-59 | Weak match | Significant gaps in qualifications |
| 0-44 | Poor match | Does not meet minimum requirements |

### Important Notes

1. The system prompt is automatically injected when `type: "ATS_SCAN"` is specified
2. The scoring is designed to be non-biased and objective
3. The API key is required when using a provider
4. The response includes actionable recommendations for improvement
5. Both matched and missing keywords are identified
6. The breakdown shows detailed scoring across 5 categories

### Integration Example (Go)

```go
import (
    "project-phoenix/v2/internal/model"
)

type ATSRequest struct {
    Type        model.PromptType       `json:"type"`
    Provider    string                 `json:"provider"`
    APIKey      string                 `json:"apiKey"`
    Model       string                 `json:"model"`
    Messages    []model.ChatMessage    `json:"messages"`
    Temperature float64                `json:"temperature"`
    MaxTokens   int                    `json:"maxTokens"`
}

// Prepare the request
userContent := fmt.Sprintf(`{
  "resume_text": "%s",
  "job_description": "%s"
}`, resumeText, jobDescription)

request := ATSRequest{
    Type:        model.ATS_SCAN, // Using enum instead of hardcoded string
    Provider:    "openai",
    APIKey:      os.Getenv("OPENAI_API_KEY"),
    Model:       "gpt-4",
    Messages: []model.ChatMessage{
        {
            Role:    "user",
            Content: userContent,
        },
    },
    Temperature: 0.7,
    MaxTokens:   2000,
}

// Make the API call
// ... (HTTP request code)
```

### Available Enums

**PromptType** (defined in `internal/model/gollm-prompts.go`):
- `model.ATS_SCAN` - ATS resume scoring

**ATSPromptType** (for specific ATS operations):
- `model.ANALYZE_RESUME` - Analyze resume for weak descriptions
- `model.ENHANCE_DESCRIPTION` - Add bullet points
- `model.REGENERATE_ITEM` - Rewrite descriptions
- `model.REGENERATE_SKILLS` - Rewrite skills section
- `model.ATS_SCORE` - Calculate ATS compatibility score
