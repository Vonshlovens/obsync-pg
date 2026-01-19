package db

import (
	"time"

	"github.com/google/uuid"
)

// VaultNote represents a markdown note in the vault
type VaultNote struct {
	ID            uuid.UUID              `db:"id"`
	Path          string                 `db:"path"`
	Filename      string                 `db:"filename"`
	Title         *string                `db:"title"`
	Tags          []string               `db:"tags"`
	Aliases       []string               `db:"aliases"`
	CreatedAt     *time.Time             `db:"created_at"`
	ModifiedAt    *time.Time             `db:"modified_at"`
	Publish       bool                   `db:"publish"`
	Frontmatter   map[string]interface{} `db:"frontmatter"`
	Body          string                 `db:"body"`
	RawContent    string                 `db:"raw_content"`
	ContentHash   string                 `db:"content_hash"`
	FileSizeBytes int64                  `db:"file_size_bytes"`
	SyncedAt      time.Time              `db:"synced_at"`
	OutgoingLinks []string               `db:"outgoing_links"`
}

// VaultAttachment represents a non-markdown file in the vault
type VaultAttachment struct {
	ID            uuid.UUID `db:"id"`
	Path          string    `db:"path"`
	Filename      string    `db:"filename"`
	Extension     *string   `db:"extension"`
	MimeType      *string   `db:"mime_type"`
	FileSizeBytes int64     `db:"file_size_bytes"`
	ContentHash   string    `db:"content_hash"`
	Data          []byte    `db:"data"`
	SyncedAt      time.Time `db:"synced_at"`
}

// SyncStatus represents the current sync status
type SyncStatus struct {
	Connected      bool
	LastSyncTime   *time.Time
	TotalNotes     int
	TotalAttach    int
	PendingChanges int
}
