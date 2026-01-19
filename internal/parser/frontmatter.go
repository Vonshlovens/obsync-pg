package parser

import (
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	// frontmatterRegex matches YAML frontmatter between --- delimiters
	frontmatterRegex = regexp.MustCompile(`(?s)^---\n(.+?)\n---\n?`)

	// Common date formats used in Obsidian
	dateFormats = []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
		"January 2, 2006",
		"Jan 2, 2006",
		"02-01-2006",
		"02/01/2006",
	}
)

// Frontmatter represents parsed YAML frontmatter from a note
type Frontmatter struct {
	Title    *string                `yaml:"title"`
	Tags     []string               `yaml:"tags"`
	Aliases  []string               `yaml:"aliases"`
	Created  *time.Time             `yaml:"created"`
	Modified *time.Time             `yaml:"modified"`
	Publish  *bool                  `yaml:"publish"`
	Extra    map[string]interface{} `yaml:"-"` // Capture unknown fields
}

// flexibleTime handles various date formats
type flexibleTime struct {
	time.Time
}

func (ft *flexibleTime) UnmarshalYAML(value *yaml.Node) error {
	var str string
	if err := value.Decode(&str); err != nil {
		return err
	}

	str = strings.TrimSpace(str)
	if str == "" {
		return nil
	}

	for _, format := range dateFormats {
		if t, err := time.Parse(format, str); err == nil {
			ft.Time = t
			return nil
		}
	}

	// Try parsing as Unix timestamp
	if t, err := time.Parse("2006", str); err == nil {
		ft.Time = t
		return nil
	}

	return nil // Don't fail on unparseable dates, just leave empty
}

// rawFrontmatter is used to capture all fields including unknown ones
type rawFrontmatter struct {
	Title    *string      `yaml:"title"`
	Tags     interface{}  `yaml:"tags"` // Can be string or []string
	Aliases  interface{}  `yaml:"aliases"`
	Created  flexibleTime `yaml:"created"`
	Modified flexibleTime `yaml:"modified"`
	Publish  *bool        `yaml:"publish"`
}

// ParseFrontmatter extracts and parses YAML frontmatter from content
func ParseFrontmatter(content string) (*Frontmatter, string, error) {
	fm := &Frontmatter{
		Extra: make(map[string]interface{}),
	}

	match := frontmatterRegex.FindStringSubmatch(content)
	if match == nil {
		// No frontmatter found
		return fm, content, nil
	}

	yamlContent := match[1]
	body := content[len(match[0]):]

	// First, parse into raw struct for known fields
	var raw rawFrontmatter
	if err := yaml.Unmarshal([]byte(yamlContent), &raw); err != nil {
		// If parsing fails, return empty frontmatter and full content
		return fm, content, nil
	}

	// Copy known fields
	fm.Title = raw.Title
	fm.Publish = raw.Publish

	if !raw.Created.IsZero() {
		t := raw.Created.Time
		fm.Created = &t
	}
	if !raw.Modified.IsZero() {
		t := raw.Modified.Time
		fm.Modified = &t
	}

	// Handle tags (can be string or []string)
	fm.Tags = normalizeStringArray(raw.Tags)

	// Handle aliases (can be string or []string)
	fm.Aliases = normalizeStringArray(raw.Aliases)

	// Parse all fields into Extra map
	var allFields map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &allFields); err == nil {
		// Remove known fields from Extra
		knownFields := map[string]bool{
			"title": true, "tags": true, "aliases": true,
			"created": true, "modified": true, "publish": true,
		}
		for k, v := range allFields {
			if !knownFields[k] {
				fm.Extra[k] = v
			}
		}
	}

	return fm, body, nil
}

// normalizeStringArray converts string or []string or []interface{} to []string
func normalizeStringArray(v interface{}) []string {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case string:
		if val == "" {
			return nil
		}
		return []string{val}
	case []string:
		return val
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

// HasFrontmatter checks if content has YAML frontmatter
func HasFrontmatter(content string) bool {
	return frontmatterRegex.MatchString(content)
}
