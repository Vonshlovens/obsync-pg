package parser

import (
	"testing"
	"time"
)

func TestParseFrontmatter_Basic(t *testing.T) {
	content := `---
title: Test Note
tags:
  - tag1
  - tag2
aliases:
  - alias1
publish: true
---
This is the body content.
`

	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fm.Title == nil || *fm.Title != "Test Note" {
		t.Errorf("expected title 'Test Note', got %v", fm.Title)
	}

	if len(fm.Tags) != 2 || fm.Tags[0] != "tag1" || fm.Tags[1] != "tag2" {
		t.Errorf("expected tags [tag1, tag2], got %v", fm.Tags)
	}

	if len(fm.Aliases) != 1 || fm.Aliases[0] != "alias1" {
		t.Errorf("expected aliases [alias1], got %v", fm.Aliases)
	}

	if fm.Publish == nil || !*fm.Publish {
		t.Errorf("expected publish true, got %v", fm.Publish)
	}

	expected := "This is the body content.\n"
	if body != expected {
		t.Errorf("expected body %q, got %q", expected, body)
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := "Just some content without frontmatter."

	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fm.Title != nil {
		t.Errorf("expected nil title, got %v", fm.Title)
	}

	if body != content {
		t.Errorf("expected body %q, got %q", content, body)
	}
}

func TestParseFrontmatter_TagsAsString(t *testing.T) {
	content := `---
tags: single-tag
---
Body
`

	fm, _, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fm.Tags) != 1 || fm.Tags[0] != "single-tag" {
		t.Errorf("expected tags [single-tag], got %v", fm.Tags)
	}
}

func TestParseFrontmatter_Dates(t *testing.T) {
	content := `---
created: 2024-01-15
modified: 2024-01-15T10:30:00
---
Body
`

	fm, _, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fm.Created == nil {
		t.Fatal("expected created date")
	}

	expectedYear := 2024
	expectedMonth := time.January
	expectedDay := 15

	if fm.Created.Year() != expectedYear || fm.Created.Month() != expectedMonth || fm.Created.Day() != expectedDay {
		t.Errorf("expected created 2024-01-15, got %v", fm.Created)
	}

	if fm.Modified == nil {
		t.Fatal("expected modified date")
	}

	if fm.Modified.Hour() != 10 || fm.Modified.Minute() != 30 {
		t.Errorf("expected modified time 10:30, got %v", fm.Modified)
	}
}

func TestParseFrontmatter_ExtraFields(t *testing.T) {
	content := `---
title: Test
custom_field: custom_value
nested:
  key: value
---
Body
`

	fm, _, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if val, ok := fm.Extra["custom_field"]; !ok || val != "custom_value" {
		t.Errorf("expected custom_field 'custom_value', got %v", fm.Extra["custom_field"])
	}

	if _, ok := fm.Extra["nested"]; !ok {
		t.Error("expected nested field to be captured")
	}
}

func TestParseFrontmatter_EmptyTags(t *testing.T) {
	content := `---
tags:
---
Body
`

	fm, _, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fm.Tags != nil && len(fm.Tags) != 0 {
		t.Errorf("expected empty tags, got %v", fm.Tags)
	}
}

func TestHasFrontmatter(t *testing.T) {
	tests := []struct {
		content  string
		expected bool
	}{
		{"---\ntitle: test\n---\nbody", true},
		{"no frontmatter here", false},
		{"---\ntitle: test\n---", true},
		{"--- not frontmatter", false},
	}

	for _, tt := range tests {
		result := HasFrontmatter(tt.content)
		if result != tt.expected {
			t.Errorf("HasFrontmatter(%q) = %v, want %v", tt.content[:min(20, len(tt.content))], result, tt.expected)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
