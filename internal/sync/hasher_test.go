package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashString(t *testing.T) {
	// Known SHA256 hash of "hello"
	expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	result := HashString("hello")

	if result != expected {
		t.Errorf("HashString(\"hello\") = %q, want %q", result, expected)
	}
}

func TestHashContent(t *testing.T) {
	content := []byte("test content")
	hash1 := HashContent(content)
	hash2 := HashContent(content)

	// Same content should produce same hash
	if hash1 != hash2 {
		t.Errorf("same content produced different hashes: %q != %q", hash1, hash2)
	}

	// Different content should produce different hash
	different := HashContent([]byte("different content"))
	if hash1 == different {
		t.Error("different content should produce different hash")
	}

	// Hash should be 64 characters (SHA256 hex)
	if len(hash1) != 64 {
		t.Errorf("hash length should be 64, got %d", len(hash1))
	}
}

func TestHashFile(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	content := "file content for hashing"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	hash, err := HashFile(tmpFile)
	if err != nil {
		t.Fatalf("HashFile failed: %v", err)
	}

	// Should match HashString of same content
	expected := HashString(content)
	if hash != expected {
		t.Errorf("HashFile result %q doesn't match HashString result %q", hash, expected)
	}
}

func TestHashFile_NotFound(t *testing.T) {
	_, err := HashFile("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestHashFile_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "empty.txt")

	if err := os.WriteFile(tmpFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	hash, err := HashFile(tmpFile)
	if err != nil {
		t.Fatalf("HashFile failed: %v", err)
	}

	// SHA256 of empty string
	expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if hash != expected {
		t.Errorf("empty file hash = %q, want %q", hash, expected)
	}
}
