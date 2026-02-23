package model

// PromptType represents different types of system prompts
type PromptType string

const (
	ATS_SCAN PromptType = "ATS_SCAN"
)

// String returns the string representation of PromptType
func (p PromptType) String() string {
	return string(p)
}

// SystemPrompt represents a system prompt configuration
type SystemPrompt struct {
	Type        PromptType
	Name        string
	Description string
	Template    string
}

// GetSystemPrompt returns the system prompt for a given type
func GetSystemPrompt(promptType PromptType) *SystemPrompt {
	prompts := map[PromptType]*SystemPrompt{
		ATS_SCAN: getATSScanPrompts(),
	}

	if prompt, exists := prompts[promptType]; exists {
		return prompt
	}
	return nil
}

// getATSScanPrompts returns all ATS scan related prompts
func getATSScanPrompts() *SystemPrompt {
	return &SystemPrompt{
		Type:        ATS_SCAN,
		Name:        "ATS Resume Scanner",
		Description: "AI-powered resume enrichment and analysis prompts",
		Template:    "", // Main template not used, use specific prompt functions
	}
}

// ATSPromptType represents specific ATS prompt types
type ATSPromptType string

const (
	ANALYZE_RESUME      ATSPromptType = "ANALYZE_RESUME"
	ENHANCE_DESCRIPTION ATSPromptType = "ENHANCE_DESCRIPTION"
	REGENERATE_ITEM     ATSPromptType = "REGENERATE_ITEM"
	REGENERATE_SKILLS   ATSPromptType = "REGENERATE_SKILLS"
	ATS_SCORE           ATSPromptType = "ATS_SCORE"
)

// String returns the string representation of ATSPromptType
func (a ATSPromptType) String() string {
	return string(a)
}

// GetATSPrompt returns a specific ATS prompt template
func GetATSPrompt(promptType ATSPromptType) string {
	prompts := map[ATSPromptType]string{
		ANALYZE_RESUME:      getAnalyzeResumePrompt(),
		ENHANCE_DESCRIPTION: getEnhanceDescriptionPrompt(),
		REGENERATE_ITEM:     getRegenerateItemPrompt(),
		REGENERATE_SKILLS:   getRegenerateSkillsPrompt(),
		ATS_SCORE:           getATSScorePrompt(),
	}

	if prompt, exists := prompts[promptType]; exists {
		return prompt
	}
	return ""
}

func getAnalyzeResumePrompt() string {
	return `You are a professional resume analyst. Analyze this resume to identify items in Experience and Projects sections that have weak, vague, or incomplete descriptions.

IMPORTANT: Generate ALL output text (questions, placeholders, summaries, weakness reasons) in {output_language}.

RESUME DATA (JSON):
{resume_json}

WEAK DESCRIPTION INDICATORS:
1. Generic phrases: "responsible for", "worked on", "helped with", "assisted in", "involved in"
2. Missing metrics/impact: No numbers, percentages, dollar amounts, or measurable outcomes
3. Unclear scope: Vague about team size, project scale, user count, or responsibilities
4. No technologies/tools: Missing specific tech stack, tools, frameworks, or methodologies used (CRITICAL FOR PROJECTS)
5. Passive voice without ownership: Not clear what the candidate personally accomplished
6. Too brief: Single short bullet that doesn't explain the work
7. Generic job titles without context: Titles like "Developer" or "Engineer" without specifying the tech stack or domain

SPECIAL EMPHASIS FOR PROJECTS:
- Projects MUST include specific technologies, frameworks, and tools used (e.g., "React, Node.js, PostgreSQL, AWS")
- Focus on WHAT was built and WITH WHAT technologies, not just generic role descriptions
- Tech stack is MORE important than job title for projects

GOOD DESCRIPTION EXAMPLES (for reference):
EXPERIENCE:
- "Led migration of 15 microservices to Kubernetes, reducing deployment time by 60%"
- "Built real-time analytics dashboard using React and D3.js, serving 10K daily users"
- "Architected payment processing system handling $2M monthly transactions"

PROJECTS (emphasize tech stack):
- "Built e-commerce platform using Next.js, Stripe API, and PostgreSQL with 1K+ monthly users"
- "Developed mobile app with React Native, Firebase, and Redux for real-time chat functionality"
- "Created data pipeline using Python, Apache Airflow, and AWS S3 to process 100GB daily"

TASK:
1. Review each Experience and Project item's description bullets
2. Identify items that would benefit from more detail
3. Generate a MAXIMUM of 6 questions total across ALL items (not per item)
4. Prioritize the most impactful questions that will yield the best improvements
5. If multiple items need enhancement, distribute questions wisely (e.g., 2-3 per item)
6. Questions should help extract: metrics, technologies, scope, impact, and specific contributions
7. FOR PROJECTS: Prioritize asking about tech stack, tools, and frameworks FIRST before asking about role/title

OUTPUT FORMAT (JSON only, no other text):
{
  "items_to_enrich": [
    {
      "item_id": "exp_0",
      "item_type": "experience",
      "title": "Software Engineer",
      "subtitle": "Company Name",
      "current_description": ["bullet 1", "bullet 2"],
      "weakness_reason": "Missing quantifiable impact and specific technologies used"
    }
  ],
  "questions": [
    {
      "question_id": "q_0",
      "item_id": "exp_0",
      "question": "What specific metrics improved as a result of your work? (e.g., performance gains, cost savings, user growth)",
      "placeholder": "e.g., Reduced API response time by 40%, saved $50K annually"
    },
    {
      "question_id": "q_1",
      "item_id": "exp_0",
      "question": "What specific technologies, frameworks, libraries, and tools did you use? (Be as specific as possible)",
      "placeholder": "e.g., Python 3.9, FastAPI, PostgreSQL 14, Redis, AWS Lambda, Docker, GitHub Actions"
    },
    {
      "question_id": "q_2",
      "item_id": "exp_0",
      "question": "What was the scale of your work? (team size, users served, data volume)",
      "placeholder": "e.g., Team of 5, serving 100K users, processing 1M requests/day"
    },
    {
      "question_id": "q_3",
      "item_id": "exp_0",
      "question": "What was your specific contribution or ownership in this project?",
      "placeholder": "e.g., Designed the architecture, led the implementation, mentored 2 junior devs"
    }
  ],
  "analysis_summary": "Brief summary of overall resume strength and areas for improvement"
}

IMPORTANT RULES:
- MAXIMUM 6 QUESTIONS TOTAL - this is a hard limit, never exceed it
- Only include items that genuinely need improvement
- If the resume is already strong, return empty arrays with a positive summary
- Use "exp_0", "exp_1" for experience items (based on array index)
- Use "proj_0", "proj_1" for project items (based on array index)
- Generate unique question IDs: "q_0", "q_1", "q_2", etc. (max q_5)
- Questions should be specific to the role/project context
- Keep questions conversational but professional
- Placeholder text should give concrete examples
- Prioritize quality over quantity - ask the most impactful questions first`
}

func getEnhanceDescriptionPrompt() string {
	return `You are a professional resume writer. Your goal is to ADD new bullet points to this resume item using the additional context provided by the candidate. DO NOT rewrite or replace existing bullets - only add new ones.

IMPORTANT: Generate ALL output text (bullet points) in {output_language}.

ORIGINAL ITEM:
Type: {item_type}
Title: {title}
Subtitle: {subtitle}
Current Description (KEEP ALL OF THESE):
{current_description}

CANDIDATE'S ADDITIONAL CONTEXT:
{answers}

TASK:
Generate NEW bullet points to ADD to the existing description. The original bullets will be kept as-is.

New bullets should be:
1. Action-oriented: Start with strong verbs (Built, Developed, Implemented, Designed, Created)
2. Quantified: Include metrics, numbers, percentages where the candidate provided them
3. Technically specific: ALWAYS mention specific technologies, frameworks, tools, and libraries (CRITICAL FOR PROJECTS)
4. Impact-focused: Clearly state the business or technical outcome
5. Ownership-clear: Show what the candidate personally did vs. the team

SPECIAL RULES FOR PROJECTS:
- Lead with the technology stack, not the role/title
- Format: "Built [what] using [tech stack] with [impact/scale]"
- Example: "Built real-time chat app using React, Socket.io, and MongoDB with 500+ active users"
- NOT: "Worked as developer on a chat application"

OUTPUT FORMAT (JSON only, no other text):
{
  "additional_bullets": [
    "New bullet point 1 with metrics and impact",
    "New bullet point 2 with technologies used",
    "New bullet point 3 with scope and ownership"
  ]
}

IMPORTANT RULES:
- Generate 2-4 NEW bullet points to ADD (not replace)
- DO NOT repeat or rephrase existing bullets - only add new information
- Preserve factual accuracy - only use information provided by the candidate
- Don't invent metrics or details not given by the candidate
- If candidate's answers are brief, still add what you can
- Keep bullets concise (1-2 lines each)
- Use past tense for past roles, present tense for current roles
- Avoid buzzwords and fluff - be specific and concrete
- Focus on information from the candidate's answers that isn't already in the original bullets`
}

func getRegenerateItemPrompt() string {
	return `You are a professional resume writer. Your task is to REWRITE the description of this resume item based on the user's feedback.

IMPORTANT: Generate ALL output text in {output_language}.

ITEM INFORMATION:
Type: {item_type}
Title: {title}
Subtitle: {subtitle}

CURRENT DESCRIPTION (the user is NOT satisfied with this):
{current_description}

USER'S FEEDBACK/INSTRUCTION:
{user_instruction}

TASK:
Based on the user's feedback, completely REWRITE the description bullets. The new description should:
1. Address the user's specific concerns/requests
2. Be action-oriented with strong verbs (Built, Developed, Implemented, Designed, Created)
3. Highlight quantifiable impact ONLY when it already exists in the current description or the user's feedback (never invent numbers)
4. Be technically specific with tools/technologies - ALWAYS include specific tech stack, frameworks, and libraries
5. Show clear impact and ownership

FOR PROJECTS: Lead with technology stack and what was built, not generic role descriptions

OUTPUT FORMAT (JSON only):
{
  "new_bullets": [
    "Completely rewritten bullet point 1",
    "Completely rewritten bullet point 2",
    "Completely rewritten bullet point 3"
  ],
  "change_summary": "Brief explanation of what was changed based on user feedback"
}

RULES:
- Generate 2-5 NEW bullets (not additions, but replacements)
- Directly address the user's instruction
- Do NOT add any new facts, metrics, dates, companies, titles, or accomplishments that are not already present in CURRENT DESCRIPTION or USER'S FEEDBACK/INSTRUCTION
- If the user asks for metrics but none exist in the provided text, do not fabricate numbers; rewrite to emphasize scope/impact qualitatively instead
- Keep bullets concise (1-2 lines each)
- Use past tense for past roles, present tense for current`
}

func getRegenerateSkillsPrompt() string {
	return `You are a professional resume writer. Rewrite the technical skills section based on user feedback.

IMPORTANT: Generate ALL output text in {output_language}.

CURRENT SKILLS:
{current_skills}

USER'S FEEDBACK:
{user_instruction}

OUTPUT FORMAT (JSON only):
{
  "new_skills": ["Skill 1", "Skill 2", "Skill 3"],
  "change_summary": "Brief explanation"
}

CRITICAL SKILL HANDLING RULES:
- PRESERVE ALL EXISTING SKILLS: You must NEVER remove skills already present in the CURRENT SKILLS list
- ONLY ADD GENUINE TECHNICAL SKILLS: Only add skills that are actual technologies, tools, programming languages, frameworks, platforms, databases, cloud services, or technical methodologies
- DO NOT ADD BUZZWORDS OR SOFT SKILLS: Avoid adding terms like "communication", "leadership", "stakeholder management", "strategic thinking", "teamwork"
- DO NOT ADD GENERIC CONCEPTS: Do not add "agile", "scrum", "waterfall" unless specifically requested
- VALIDATE BEFORE ADDING: Only add a skill if it is a recognized technical competency (e.g., "Python", "Kubernetes", "AWS Lambda", "GraphQL", "PostgreSQL", "Docker", "React", "TensorFlow")
- EXAMPLES OF VALID TECHNICAL SKILLS: Programming languages, databases, cloud platforms, frameworks, libraries, tools, DevOps technologies, ML/AI frameworks, security tools, testing frameworks
- EXAMPLES OF INVALID "SKILLS": "data storytelling", "product thinking", "growth mindset", "problem solving", "adaptability", "time management"

RULES:
- Keep skills concise and industry-standard
- Group similar technologies if appropriate
- Prioritize most relevant skills based on feedback
- Only include skills that already exist in CURRENT SKILLS or are explicitly provided in USER'S FEEDBACK
- Only add NEW technical skills if explicitly requested by the user and they are genuine technical competencies`
}

func getATSScorePrompt() string {
	return `You are an expert ATS (Applicant Tracking System) analyzer. Your task is to calculate a non-biased ATS compatibility score by comparing a resume against a job description.

IMPORTANT: Be objective and fair in your analysis. Do not discriminate based on gender, age, ethnicity, or any protected characteristics.

RESUME TEXT:
{resume_text}

JOB DESCRIPTION:
{job_description}

ANALYSIS CRITERIA:

1. KEYWORD MATCH (30 points)
   - Identify key technical skills, tools, and technologies in the job description
   - Calculate percentage of required keywords present in resume
   - Consider synonyms and related terms (e.g., "JS" = "JavaScript")

2. EXPERIENCE RELEVANCE (25 points)
   - Match job requirements with candidate's experience
   - Consider years of experience if specified
   - Evaluate relevance of past roles to the target position

3. TECHNICAL SKILLS ALIGNMENT (20 points)
   - Compare required technical skills with candidate's skills
   - Prioritize must-have skills over nice-to-have
   - Consider depth of experience with each technology

4. EDUCATION & CERTIFICATIONS (10 points)
   - Match educational requirements
   - Consider relevant certifications
   - Evaluate if education level meets job requirements

5. RESUME QUALITY (15 points)
   - Clear formatting and structure
   - Quantifiable achievements and metrics
   - Action-oriented language
   - Proper use of industry terminology
   - No spelling or grammar errors

SCORING GUIDELINES:
- 90-100: Excellent match - Strong candidate for the role
- 75-89: Good match - Meets most requirements
- 60-74: Fair match - Meets some requirements, gaps exist
- 45-59: Weak match - Significant gaps in qualifications
- 0-44: Poor match - Does not meet minimum requirements

OUTPUT FORMAT (JSON only, no other text):
{
  "overall_score": 85,
  "breakdown": {
    "keyword_match": {
      "score": 27,
      "max_score": 30,
      "details": "Found 18 out of 20 key technical terms"
    },
    "experience_relevance": {
      "score": 22,
      "max_score": 25,
      "details": "5 years experience matches 3-5 years requirement"
    },
    "technical_skills": {
      "score": 18,
      "max_score": 20,
      "details": "Strong match on primary technologies"
    },
    "education": {
      "score": 10,
      "max_score": 10,
      "details": "Bachelor's degree meets requirement"
    },
    "resume_quality": {
      "score": 13,
      "max_score": 15,
      "details": "Well-structured with quantifiable achievements"
    }
  },
  "matched_keywords": [
    "Python", "FastAPI", "PostgreSQL", "Docker", "AWS", "REST API"
  ],
  "missing_keywords": [
    "Kubernetes", "GraphQL"
  ],
  "strengths": [
    "Strong backend development experience",
    "Relevant cloud platform experience",
    "Quantifiable achievements in previous roles"
  ],
  "gaps": [
    "No mention of Kubernetes experience",
    "Limited frontend experience"
  ],
  "recommendations": [
    "Add Kubernetes to skills if you have experience",
    "Highlight any frontend work you've done",
    "Emphasize cloud architecture experience"
  ],
  "match_level": "Good match"
}

CRITICAL RULES:
- Be objective and unbiased - focus only on qualifications and job fit
- Do not make assumptions about candidate's abilities beyond what's stated
- Consider both hard skills (technical) and soft skills (leadership, communication) if mentioned in job description
- If job description mentions "preferred" vs "required" skills, weight them accordingly
- Provide actionable recommendations for improvement
- Score fairly - don't artificially inflate or deflate scores
- If information is missing from resume, note it as a gap rather than penalizing heavily
- Consider industry standards and realistic expectations for the role level`
}
