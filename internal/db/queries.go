package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// UpsertNote inserts or updates a note in the database
func (db *DB) UpsertNote(ctx context.Context, note *VaultNote) error {
	frontmatterJSON, err := json.Marshal(note.Frontmatter)
	if err != nil {
		return fmt.Errorf("failed to marshal frontmatter: %w", err)
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO vault_notes (
			path, filename, title, tags, aliases, created_at, modified_at,
			publish, frontmatter, body, raw_content, content_hash,
			file_size_bytes, outgoing_links
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
		ON CONFLICT (path) DO UPDATE SET
			filename = EXCLUDED.filename,
			title = EXCLUDED.title,
			tags = EXCLUDED.tags,
			aliases = EXCLUDED.aliases,
			created_at = EXCLUDED.created_at,
			modified_at = EXCLUDED.modified_at,
			publish = EXCLUDED.publish,
			frontmatter = EXCLUDED.frontmatter,
			body = EXCLUDED.body,
			raw_content = EXCLUDED.raw_content,
			content_hash = EXCLUDED.content_hash,
			file_size_bytes = EXCLUDED.file_size_bytes,
			outgoing_links = EXCLUDED.outgoing_links,
			synced_at = NOW()
	`,
		note.Path, note.Filename, note.Title, note.Tags, note.Aliases,
		note.CreatedAt, note.ModifiedAt, note.Publish, frontmatterJSON,
		note.Body, note.RawContent, note.ContentHash, note.FileSizeBytes,
		note.OutgoingLinks,
	)

	return err
}

// UpsertAttachment inserts or updates an attachment in the database
func (db *DB) UpsertAttachment(ctx context.Context, att *VaultAttachment) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO vault_attachments (
			path, filename, extension, mime_type, file_size_bytes,
			content_hash, data
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
		ON CONFLICT (path) DO UPDATE SET
			filename = EXCLUDED.filename,
			extension = EXCLUDED.extension,
			mime_type = EXCLUDED.mime_type,
			file_size_bytes = EXCLUDED.file_size_bytes,
			content_hash = EXCLUDED.content_hash,
			data = EXCLUDED.data,
			synced_at = NOW()
	`,
		att.Path, att.Filename, att.Extension, att.MimeType,
		att.FileSizeBytes, att.ContentHash, att.Data,
	)

	return err
}

// DeleteNote removes a note from the database
func (db *DB) DeleteNote(ctx context.Context, path string) error {
	_, err := db.Pool.Exec(ctx, "DELETE FROM vault_notes WHERE path = $1", path)
	return err
}

// DeleteAttachment removes an attachment from the database
func (db *DB) DeleteAttachment(ctx context.Context, path string) error {
	_, err := db.Pool.Exec(ctx, "DELETE FROM vault_attachments WHERE path = $1", path)
	return err
}

// GetNoteByPath retrieves a note by its path
func (db *DB) GetNoteByPath(ctx context.Context, path string) (*VaultNote, error) {
	note := &VaultNote{}
	var frontmatterJSON []byte

	err := db.Pool.QueryRow(ctx, `
		SELECT id, path, filename, title, tags, aliases, created_at,
			modified_at, publish, frontmatter, body, raw_content,
			content_hash, file_size_bytes, synced_at, outgoing_links
		FROM vault_notes WHERE path = $1
	`, path).Scan(
		&note.ID, &note.Path, &note.Filename, &note.Title, &note.Tags,
		&note.Aliases, &note.CreatedAt, &note.ModifiedAt, &note.Publish,
		&frontmatterJSON, &note.Body, &note.RawContent, &note.ContentHash,
		&note.FileSizeBytes, &note.SyncedAt, &note.OutgoingLinks,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if len(frontmatterJSON) > 0 {
		if err := json.Unmarshal(frontmatterJSON, &note.Frontmatter); err != nil {
			return nil, fmt.Errorf("failed to unmarshal frontmatter: %w", err)
		}
	}

	return note, nil
}

// GetAttachmentByPath retrieves an attachment by its path
func (db *DB) GetAttachmentByPath(ctx context.Context, path string) (*VaultAttachment, error) {
	att := &VaultAttachment{}

	err := db.Pool.QueryRow(ctx, `
		SELECT id, path, filename, extension, mime_type, file_size_bytes,
			content_hash, data, synced_at
		FROM vault_attachments WHERE path = $1
	`, path).Scan(
		&att.ID, &att.Path, &att.Filename, &att.Extension, &att.MimeType,
		&att.FileSizeBytes, &att.ContentHash, &att.Data, &att.SyncedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return att, nil
}

// GetAllNoteHashes returns a map of path -> content_hash for all notes
func (db *DB) GetAllNoteHashes(ctx context.Context) (map[string]string, error) {
	rows, err := db.Pool.Query(ctx, "SELECT path, content_hash FROM vault_notes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hashes := make(map[string]string)
	for rows.Next() {
		var path, hash string
		if err := rows.Scan(&path, &hash); err != nil {
			return nil, err
		}
		hashes[path] = hash
	}

	return hashes, rows.Err()
}

// GetAllAttachmentHashes returns a map of path -> content_hash for all attachments
func (db *DB) GetAllAttachmentHashes(ctx context.Context) (map[string]string, error) {
	rows, err := db.Pool.Query(ctx, "SELECT path, content_hash FROM vault_attachments")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hashes := make(map[string]string)
	for rows.Next() {
		var path, hash string
		if err := rows.Scan(&path, &hash); err != nil {
			return nil, err
		}
		hashes[path] = hash
	}

	return hashes, rows.Err()
}

// GetAllNotePaths returns all note paths in the database
func (db *DB) GetAllNotePaths(ctx context.Context) ([]string, error) {
	rows, err := db.Pool.Query(ctx, "SELECT path FROM vault_notes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}

	return paths, rows.Err()
}

// GetAllAttachmentPaths returns all attachment paths in the database
func (db *DB) GetAllAttachmentPaths(ctx context.Context) ([]string, error) {
	rows, err := db.Pool.Query(ctx, "SELECT path FROM vault_attachments")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}

	return paths, rows.Err()
}

// GetAllNotes returns all notes from the database (for pull command)
func (db *DB) GetAllNotes(ctx context.Context) ([]*VaultNote, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, path, filename, title, tags, aliases, created_at,
			modified_at, publish, frontmatter, body, raw_content,
			content_hash, file_size_bytes, synced_at, outgoing_links
		FROM vault_notes
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*VaultNote
	for rows.Next() {
		note := &VaultNote{}
		var frontmatterJSON []byte

		if err := rows.Scan(
			&note.ID, &note.Path, &note.Filename, &note.Title, &note.Tags,
			&note.Aliases, &note.CreatedAt, &note.ModifiedAt, &note.Publish,
			&frontmatterJSON, &note.Body, &note.RawContent, &note.ContentHash,
			&note.FileSizeBytes, &note.SyncedAt, &note.OutgoingLinks,
		); err != nil {
			return nil, err
		}

		if len(frontmatterJSON) > 0 {
			if err := json.Unmarshal(frontmatterJSON, &note.Frontmatter); err != nil {
				return nil, fmt.Errorf("failed to unmarshal frontmatter: %w", err)
			}
		}

		notes = append(notes, note)
	}

	return notes, rows.Err()
}

// GetAllAttachments returns all attachments from the database (for pull command)
func (db *DB) GetAllAttachments(ctx context.Context) ([]*VaultAttachment, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, path, filename, extension, mime_type, file_size_bytes,
			content_hash, data, synced_at
		FROM vault_attachments
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []*VaultAttachment
	for rows.Next() {
		att := &VaultAttachment{}

		if err := rows.Scan(
			&att.ID, &att.Path, &att.Filename, &att.Extension, &att.MimeType,
			&att.FileSizeBytes, &att.ContentHash, &att.Data, &att.SyncedAt,
		); err != nil {
			return nil, err
		}

		attachments = append(attachments, att)
	}

	return attachments, rows.Err()
}

// BatchDeleteNotes deletes multiple notes by path
func (db *DB) BatchDeleteNotes(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	_, err := db.Pool.Exec(ctx,
		"DELETE FROM vault_notes WHERE path = ANY($1)",
		paths,
	)
	return err
}

// BatchDeleteAttachments deletes multiple attachments by path
func (db *DB) BatchDeleteAttachments(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	_, err := db.Pool.Exec(ctx,
		"DELETE FROM vault_attachments WHERE path = ANY($1)",
		paths,
	)
	return err
}
