package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// FileExtension represents a file extension to search for API keys
// Extensions are processed sequentially to distribute GitHub API load
type FileExtension struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Extension      string             `bson:"extension" json:"extension"`                                   // e.g., "js", "ts", "env", "py"
	Priority       int                `bson:"priority" json:"priority"`                                     // Lower number = higher priority (1 is highest)
	Enabled        bool               `bson:"enabled" json:"enabled"`                                       // Whether to search this extension
	LastSearchedAt *time.Time         `bson:"last_searched_at,omitempty" json:"last_searched_at,omitempty"` // Last time this extension was searched
	ResultCount    int                `bson:"result_count" json:"result_count"`                             // Total results found in last search
	KeysFound      int                `bson:"keys_found" json:"keys_found"`                                 // Total keys found in last search
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time          `bson:"updated_at" json:"updated_at"`
}

// Common file extensions for API key searches
// Ordered by likelihood of containing API keys
var DefaultFileExtensions = []struct {
	Extension string
	Priority  int
}{
	{"env", 1},         // .env files - highest priority
	{"js", 2},          // JavaScript
	{"ts", 3},          // TypeScript
	{"py", 4},          // Python
	{"json", 5},        // JSON config files
	{"yaml", 6},        // YAML config files
	{"yml", 7},         // YAML alternative extension
	{"php", 8},         // PHP
	{"go", 9},          // Go
	{"java", 10},       // Java
	{"rb", 11},         // Ruby
	{"cs", 12},         // C#
	{"cpp", 13},        // C++
	{"swift", 14},      // Swift
	{"kt", 15},         // Kotlin
	{"rs", 16},         // Rust
	{"sh", 17},         // Shell scripts
	{"bash", 18},       // Bash scripts
	{"config", 19},     // Config files
	{"conf", 20},       // Config files alternative
	{"xml", 21},        // XML
	{"properties", 22}, // Java properties
	{"toml", 23},       // TOML config
	{"ini", 24},        // INI config
	{"txt", 25},        // Text files
}
