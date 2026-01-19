package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// HashFile computes SHA256 hash of a file
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashContent computes SHA256 hash of content bytes
func HashContent(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

// HashString computes SHA256 hash of a string
func HashString(content string) string {
	return HashContent([]byte(content))
}
