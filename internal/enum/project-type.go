package enum

import "strings"

type ProjectType string

const (
	ProjectUnknown ProjectType = ""
)

func (p ProjectType) IsValid() bool {
	return strings.TrimSpace(string(p)) != ""
}

func ParseProjectType(raw string) ProjectType {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return ProjectUnknown
	}

	return ProjectType(normalized)
}
