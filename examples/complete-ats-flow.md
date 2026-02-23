# Complete ATS Scanning Flow Example

This document demonstrates the complete flow of scanning a resume against a job description using the GoLLM API.

## Prerequisites

- API server running on `http://localhost:8080`
- Valid API key for your chosen provider (OpenAI, Anthropic, etc.)

## Step 1: Prepare Your Data

### Resume Text
```text
John Doe
Senior Backend Engineer

EXPERIENCE:
- Senior Backend Engineer at Tech Corp (2020-2023)
  * Built microservices architecture using Python, FastAPI, and PostgreSQL
  * Reduced API response time by 40% through query optimization and caching
  * Led team of 5 engineers in cloud migration to AWS (EC2, Lambda, S3)
  * Implemented CI/CD pipeline using Docker, Kubernetes, and GitHub Actions
  * Designed and deployed RESTful APIs serving 100K+ daily active users

- Backend Developer at StartupXYZ (2018-2020)
  * Developed backend services using Node.js and Express
  * Integrated third-party APIs and payment gateways
  * Optimized database queries reducing load time by 30%

SKILLS:
Python, FastAPI, PostgreSQL, Docker, AWS, REST APIs, Redis, Kubernetes, 
Node.js, Express, Git, CI/CD, Microservices
```

### Job Description
```text
Senior Backend Engineer

We are looking for a Senior Backend Engineer with 3-5 years of experience 
to join our growing team.

Required Skills:
- Python (3+ years)
- FastAPI or Django
- PostgreSQL or MySQL
- Docker
- AWS (EC2, Lambda, S3)
- REST API design
- Kubernetes
- Experience with microservices architecture

Preferred Skills:
- GraphQL
- Redis
- Message queues (RabbitMQ, Kafka)
- Monitoring tools (Prometheus, Grafana)

Responsibilities:
- Design and implement scalable backend services
- Optimize database performance
- Lead technical initiatives
- Mentor junior developers
```

## Step 2: Make API Request

### Using cURL

```bash
curl -X POST http://localhost:8080/api/gollm/ats/scan \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ATS_SCAN",
    "provider": "openai",
    "apiKey": "sk-your-openai-api-key-here",
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "{\"resume_text\": \"John Doe\\nSenior Backend Engineer\\n\\nEXPERIENCE:\\n- Senior Backend Engineer at Tech Corp (2020-2023)\\n  * Built microservices architecture using Python, FastAPI, and PostgreSQL\\n  * Reduced API response time by 40% through query optimization and caching\\n  * Led team of 5 engineers in cloud migration to AWS (EC2, Lambda, S3)\\n  * Implemented CI/CD pipeline using Docker, Kubernetes, and GitHub Actions\\n  * Designed and deployed RESTful APIs serving 100K+ daily active users\\n\\n- Backend Developer at StartupXYZ (2018-2020)\\n  * Developed backend services using Node.js and Express\\n  * Integrated third-party APIs and payment gateways\\n  * Optimized database queries reducing load time by 30%\\n\\nSKILLS:\\nPython, FastAPI, PostgreSQL, Docker, AWS, REST APIs, Redis, Kubernetes, Node.js, Express, Git, CI/CD, Microservices\", \"job_description\": \"Senior Backend Engineer\\n\\nWe are looking for a Senior Backend Engineer with 3-5 years of experience to join our growing team.\\n\\nRequired Skills:\\n- Python (3+ years)\\n- FastAPI or Django\\n- PostgreSQL or MySQL\\n- Docker\\n- AWS (EC2, Lambda, S3)\\n- REST API design\\n- Kubernetes\\n- Experience with microservices architecture\\n\\nPreferred Skills:\\n- GraphQL\\n- Redis\\n- Message queues (RabbitMQ, Kafka)\\n- Monitoring tools (Prometheus, Grafana)\\n\\nResponsibilities:\\n- Design and implement scalable backend services\\n- Optimize database performance\\n- Lead technical initiatives\\n- Mentor junior developers\"}"
      }
    ],
    "temperature": 0.7,
    "maxTokens": 2000
  }'
```

### Using JavaScript/Node.js

```javascript
const axios = require('axios');

const resumeText = `John Doe
Senior Backend Engineer

EXPERIENCE:
- Senior Backend Engineer at Tech Corp (2020-2023)
  * Built microservices architecture using Python, FastAPI, and PostgreSQL
  * Reduced API response time by 40% through query optimization and caching
  * Led team of 5 engineers in cloud migration to AWS (EC2, Lambda, S3)
  * Implemented CI/CD pipeline using Docker, Kubernetes, and GitHub Actions
  * Designed and deployed RESTful APIs serving 100K+ daily active users

SKILLS:
Python, FastAPI, PostgreSQL, Docker, AWS, REST APIs, Redis, Kubernetes`;

const jobDescription = `Senior Backend Engineer

Required Skills:
- Python (3+ years)
- FastAPI or Django
- PostgreSQL or MySQL
- Docker
- AWS (EC2, Lambda, S3)
- REST API design
- Kubernetes
- Experience with microservices architecture`;

async function scanResume() {
  try {
    const response = await axios.post('http://localhost:8080/api/gollm/ats/scan', {
      type: 'ATS_SCAN',
      provider: 'openai',
      apiKey: process.env.OPENAI_API_KEY,
      model: 'gpt-4',
      messages: [
        {
          role: 'user',
          content: JSON.stringify({
            resume_text: resumeText,
            job_description: jobDescription
          })
        }
      ],
      temperature: 0.7,
      maxTokens: 2000
    });

    console.log('ATS Score:', response.data.data.ats_score.overall_score);
    console.log('Match Level:', response.data.data.ats_score.match_level);
    console.log('Strengths:', response.data.data.ats_score.strengths);
    console.log('Gaps:', response.data.data.ats_score.gaps);
    console.log('Recommendations:', response.data.data.ats_score.recommendations);
  } catch (error) {
    console.error('Error:', error.response?.data || error.message);
  }
}

scanResume();
```

### Using Go

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"project-phoenix/v2/internal/model"
)

func main() {
	resumeText := `John Doe
Senior Backend Engineer

EXPERIENCE:
- Senior Backend Engineer at Tech Corp (2020-2023)
  * Built microservices architecture using Python, FastAPI, and PostgreSQL
  * Reduced API response time by 40% through query optimization and caching
  * Led team of 5 engineers in cloud migration to AWS (EC2, Lambda, S3)

SKILLS:
Python, FastAPI, PostgreSQL, Docker, AWS, REST APIs, Redis, Kubernetes`

	jobDescription := `Senior Backend Engineer

Required Skills:
- Python (3+ years)
- FastAPI or Django
- PostgreSQL or MySQL
- Docker
- AWS (EC2, Lambda, S3)
- REST API design
- Kubernetes`

	// Prepare user message content
	userContent := map[string]string{
		"resume_text":     resumeText,
		"job_description": jobDescription,
	}
	userContentJSON, _ := json.Marshal(userContent)

	// Prepare request
	request := model.ChatCompletionRequest{
		Type:     model.ATS_SCAN,
		Provider: "openai",
		APIKey:   os.Getenv("OPENAI_API_KEY"),
		Model:    "gpt-4",
		Messages: []model.ChatMessage{
			{
				Role:    "user",
				Content: string(userContentJSON),
			},
		},
		Temperature: 0.7,
		MaxTokens:   2000,
	}

	// Make API call
	requestBody, _ := json.Marshal(request)
	resp, err := http.Post(
		"http://localhost:8080/api/gollm/ats/scan",
		"application/json",
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Parse response
	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	json.Unmarshal(body, &response)

	// Extract ATS score
	data := response["data"].(map[string]interface{})
	atsScore := data["ats_score"].(map[string]interface{})

	fmt.Printf("Overall Score: %.0f\n", atsScore["overall_score"])
	fmt.Printf("Match Level: %s\n", atsScore["match_level"])
	fmt.Println("\nStrengths:")
	for _, strength := range atsScore["strengths"].([]interface{}) {
		fmt.Printf("  - %s\n", strength)
	}
	fmt.Println("\nGaps:")
	for _, gap := range atsScore["gaps"].([]interface{}) {
		fmt.Printf("  - %s\n", gap)
	}
}
```

## Step 3: Expected Response

```json
{
  "code": 1022,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "model": "gpt-4",
    "ats_score": {
      "overall_score": 88,
      "breakdown": {
        "keyword_match": {
          "score": 28,
          "max_score": 30,
          "details": "Found 19 out of 20 key technical terms"
        },
        "experience_relevance": {
          "score": 23,
          "max_score": 25,
          "details": "5 years experience exceeds 3-5 years requirement"
        },
        "technical_skills": {
          "score": 19,
          "max_score": 20,
          "details": "Excellent match on all primary technologies"
        },
        "education": {
          "score": 8,
          "max_score": 10,
          "details": "Education not explicitly mentioned"
        },
        "resume_quality": {
          "score": 14,
          "max_score": 15,
          "details": "Well-structured with strong quantifiable achievements"
        }
      },
      "matched_keywords": [
        "Python",
        "FastAPI",
        "PostgreSQL",
        "Docker",
        "AWS",
        "EC2",
        "Lambda",
        "S3",
        "REST API",
        "Kubernetes",
        "Redis",
        "Microservices",
        "CI/CD"
      ],
      "missing_keywords": [
        "GraphQL",
        "RabbitMQ",
        "Kafka",
        "Prometheus",
        "Grafana"
      ],
      "strengths": [
        "Strong backend development experience with Python and FastAPI (3+ years)",
        "Extensive AWS experience with specific services mentioned (EC2, Lambda, S3)",
        "Proven microservices architecture experience",
        "Quantifiable achievements (40% performance improvement)",
        "Leadership experience managing team of 5 engineers",
        "Kubernetes and Docker expertise clearly demonstrated",
        "Redis experience matches preferred skills"
      ],
      "gaps": [
        "No mention of GraphQL experience",
        "Missing message queue experience (RabbitMQ, Kafka)",
        "No monitoring tools mentioned (Prometheus, Grafana)",
        "Education background not specified"
      ],
      "recommendations": [
        "Add GraphQL to skills if you have any experience with it",
        "Mention any message queue experience (RabbitMQ, Kafka, SQS)",
        "Include monitoring/observability tools you've used",
        "Add education section with degree information",
        "Highlight any mentoring or leadership responsibilities in more detail"
      ],
      "match_level": "Good match"
    },
    "usage": {
      "promptTokens": 520,
      "completionTokens": 380,
      "totalTokens": 900
    },
    "createdAt": "2024-01-15T10:30:00Z"
  }
}
```

## Step 4: Interpret Results

### Score Breakdown
- **Overall Score: 88/100** - Good match
- **Keyword Match: 28/30** - Excellent keyword coverage
- **Experience: 23/25** - Experience level exceeds requirements
- **Technical Skills: 19/20** - Strong technical alignment
- **Education: 8/10** - Minor gap (not mentioned)
- **Resume Quality: 14/15** - Well-written with metrics

### Key Insights
1. **Strong Candidate**: 88% match indicates this is a strong candidate
2. **Core Requirements Met**: All required skills are present
3. **Minor Gaps**: Missing some preferred skills (GraphQL, message queues)
4. **Action Items**: Add missing skills if applicable, mention education

## Alternative: Using General Chat Endpoint

You can also use the general chat endpoint with `type: "ATS_SCAN"`:

```bash
curl -X POST http://localhost:8080/api/gollm/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ATS_SCAN",
    "provider": "openai",
    "apiKey": "sk-...",
    "model": "gpt-4",
    "messages": [{
      "role": "user",
      "content": "{\"resume_text\": \"...\", \"job_description\": \"...\"}"
    }]
  }'
```

The system will automatically inject the ATS_SCORE prompt when it detects `type: "ATS_SCAN"`.

## Error Handling

### Missing API Key
```json
{
  "code": 1001,
  "data": "API key is required when provider is specified"
}
```

### Invalid Provider
```json
{
  "code": 1001,
  "data": "LLM service error: unsupported provider: invalid_provider"
}
```

### Invalid Request Format
```json
{
  "code": 1001,
  "data": "Invalid message format. Expected JSON with resume_text and job_description"
}
```

## Tips for Best Results

1. **Use GPT-4**: More accurate scoring than GPT-3.5
2. **Clean Text**: Remove special formatting from resume/job description
3. **Complete Data**: Include all relevant information
4. **Adjust Temperature**: Lower (0.3-0.5) for more consistent scoring
5. **Increase MaxTokens**: Use 2000+ for detailed analysis
