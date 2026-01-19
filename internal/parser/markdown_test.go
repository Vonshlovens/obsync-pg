package parser

import (
	"testing"
)

func TestExtractWikiLinks(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "simple link",
			content:  "Check out [[My Page]]",
			expected: []string{"My Page"},
		},
		{
			name:     "link with alias",
			content:  "See [[My Page|display text]]",
			expected: []string{"My Page"},
		},
		{
			name:     "multiple links",
			content:  "Link to [[Page One]] and [[Page Two]]",
			expected: []string{"Page One", "Page Two"},
		},
		{
			name:     "link with heading",
			content:  "[[Page Name#Heading]]",
			expected: []string{"Page Name"},
		},
		{
			name:     "nested path",
			content:  "[[folder/subfolder/page]]",
			expected: []string{"folder/subfolder/page"},
		},
		{
			name:     "no links",
			content:  "Just regular text without links",
			expected: nil,
		},
		{
			name:     "duplicate links",
			content:  "[[Page]] and [[Page]] again",
			expected: []string{"Page"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractWikiLinks(tt.content)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d links, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i, link := range result {
				if link != tt.expected[i] {
					t.Errorf("expected link %q, got %q", tt.expected[i], link)
				}
			}
		})
	}
}

func TestExtractInlineTags(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "simple tag",
			content:  "Some text #mytag here",
			expected: []string{"mytag"},
		},
		{
			name:     "tag at start",
			content:  "#starttag more text",
			expected: []string{"starttag"},
		},
		{
			name:     "multiple tags",
			content:  "#tag1 and #tag2 and #tag3",
			expected: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:     "tag with hyphen",
			content:  "#my-tag-here",
			expected: []string{"my-tag-here"},
		},
		{
			name:     "tag with slash",
			content:  "#parent/child",
			expected: []string{"parent/child"},
		},
		{
			name:     "ignore numbers",
			content:  "Issue #123 is fixed",
			expected: nil,
		},
		{
			name:     "ignore in inline code",
			content:  "Use `#hashtag` in code",
			expected: nil,
		},
		{
			name:     "ignore in code block",
			content:  "```\n#code-tag\n```",
			expected: nil,
		},
		{
			name:     "html entity should not match",
			content:  "&#123; encoded",
			expected: nil,
		},
		{
			name:     "duplicate tags",
			content:  "#tag and #tag again",
			expected: []string{"tag"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractInlineTags(tt.content)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d tags, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i, tag := range result {
				if tag != tt.expected[i] {
					t.Errorf("expected tag %q, got %q", tt.expected[i], tag)
				}
			}
		})
	}
}

func TestMergeTags(t *testing.T) {
	tests := []struct {
		name      string
		fm        []string
		inline    []string
		expected  int
	}{
		{
			name:     "no duplicates",
			fm:       []string{"tag1", "tag2"},
			inline:   []string{"tag3", "tag4"},
			expected: 4,
		},
		{
			name:     "with duplicates",
			fm:       []string{"tag1", "tag2"},
			inline:   []string{"tag2", "tag3"},
			expected: 3,
		},
		{
			name:     "case insensitive",
			fm:       []string{"Tag1"},
			inline:   []string{"tag1"},
			expected: 1,
		},
		{
			name:     "empty frontmatter",
			fm:       nil,
			inline:   []string{"tag1"},
			expected: 1,
		},
		{
			name:     "empty inline",
			fm:       []string{"tag1"},
			inline:   nil,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeTags(tt.fm, tt.inline)
			if len(result) != tt.expected {
				t.Errorf("expected %d tags, got %d: %v", tt.expected, len(result), result)
			}
		})
	}
}

func TestParserParseContent(t *testing.T) {
	p := NewParser()

	content := `---
title: My Note
tags:
  - frontmatter-tag
---
This is my note with [[Link One]] and [[Link Two]].

It also has #inline-tag and #another-tag.
`

	note, err := p.ParseContent(content, "test.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if note.Frontmatter.Title == nil || *note.Frontmatter.Title != "My Note" {
		t.Errorf("expected title 'My Note', got %v", note.Frontmatter.Title)
	}

	if len(note.OutgoingLinks) != 2 {
		t.Errorf("expected 2 outgoing links, got %d", len(note.OutgoingLinks))
	}

	if len(note.InlineTags) != 2 {
		t.Errorf("expected 2 inline tags, got %d", len(note.InlineTags))
	}

	if note.RawContent != content {
		t.Error("raw content doesn't match input")
	}
}

func TestIsValidUTF8(t *testing.T) {
	tests := []struct {
		content  string
		expected bool
	}{
		{"Hello World", true},
		{"日本語", true},
		{"", true},
		{string([]byte{0xff, 0xfe}), false},
	}

	for _, tt := range tests {
		result := IsValidUTF8(tt.content)
		if result != tt.expected {
			t.Errorf("IsValidUTF8(%q) = %v, want %v", tt.content, result, tt.expected)
		}
	}
}
