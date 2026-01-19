package parser

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	// wikiLinkRegex matches [[Page Name]] and [[Page Name|Alias]]
	wikiLinkRegex = regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)

	// inlineTagRegex matches #tag-name (but not #123 or inside code blocks)
	inlineTagRegex = regexp.MustCompile(`(?:^|[^&\w])#([a-zA-Z][a-zA-Z0-9_/-]*)`)

	// codeBlockRegex matches fenced code blocks
	codeBlockRegex = regexp.MustCompile("(?s)```.*?```")

	// inlineCodeRegex matches inline code
	inlineCodeRegex = regexp.MustCompile("`[^`]+`")
)

// ParsedNote represents a fully parsed markdown note
type ParsedNote struct {
	Frontmatter   *Frontmatter
	Body          string
	RawContent    string
	OutgoingLinks []string
	InlineTags    []string
}

// Parser handles parsing of markdown notes
type Parser struct{}

// NewParser creates a new Parser instance
func NewParser() *Parser {
	return &Parser{}
}

// ParseFile reads and parses a markdown file
func (p *Parser) ParseFile(path string) (*ParsedNote, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return p.ParseContent(string(content), path)
}

// ParseContent parses markdown content
func (p *Parser) ParseContent(content string, path string) (*ParsedNote, error) {
	note := &ParsedNote{
		RawContent: content,
	}

	// Parse frontmatter
	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		return nil, err
	}
	note.Frontmatter = fm
	note.Body = body

	// Extract wikilinks from body
	note.OutgoingLinks = extractWikiLinks(body)

	// Extract inline tags from body (excluding code blocks)
	note.InlineTags = extractInlineTags(body)

	// If title not in frontmatter, try to use filename
	if fm.Title == nil || *fm.Title == "" {
		filename := filepath.Base(path)
		title := strings.TrimSuffix(filename, filepath.Ext(filename))
		fm.Title = &title
	}

	return note, nil
}

// extractWikiLinks finds all [[wikilinks]] in the content
func extractWikiLinks(content string) []string {
	matches := wikiLinkRegex.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	var links []string

	for _, match := range matches {
		if len(match) > 1 {
			link := strings.TrimSpace(match[1])
			// Handle nested paths and anchors
			// [[folder/page#heading]] -> folder/page
			if idx := strings.Index(link, "#"); idx != -1 {
				link = link[:idx]
			}
			link = strings.TrimSpace(link)

			if link != "" && !seen[link] {
				seen[link] = true
				links = append(links, link)
			}
		}
	}

	return links
}

// extractInlineTags finds all #tags in the content, excluding code blocks
func extractInlineTags(content string) []string {
	// Remove code blocks to avoid matching tags in code
	cleanContent := codeBlockRegex.ReplaceAllString(content, "")
	cleanContent = inlineCodeRegex.ReplaceAllString(cleanContent, "")

	matches := inlineTagRegex.FindAllStringSubmatch(cleanContent, -1)
	seen := make(map[string]bool)
	var tags []string

	for _, match := range matches {
		if len(match) > 1 {
			tag := strings.ToLower(strings.TrimSpace(match[1]))
			if tag != "" && !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}

	return tags
}

// MergeTags combines frontmatter tags and inline tags, removing duplicates
func MergeTags(frontmatterTags, inlineTags []string) []string {
	seen := make(map[string]bool)
	var merged []string

	for _, tag := range frontmatterTags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" && !seen[tag] {
			seen[tag] = true
			merged = append(merged, tag)
		}
	}

	for _, tag := range inlineTags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" && !seen[tag] {
			seen[tag] = true
			merged = append(merged, tag)
		}
	}

	return merged
}

// GetFileTimestamps returns created and modified times from file stats
func GetFileTimestamps(path string) (created *time.Time, modified *time.Time, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}

	modTime := info.ModTime()
	modified = &modTime

	// Note: Getting creation time is platform-specific
	// On most systems, we only have access to modification time reliably
	// The creation time would require platform-specific code
	// For now, we use modification time as a fallback for created
	created = &modTime

	return created, modified, nil
}

// IsValidUTF8 checks if content is valid UTF-8
func IsValidUTF8(content string) bool {
	return utf8.ValidString(content)
}
