package sync

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/schollz/progressbar/v3"

	"github.com/vonshlovens/obsync-pg/internal/config"
	"github.com/vonshlovens/obsync-pg/internal/db"
	"github.com/vonshlovens/obsync-pg/internal/parser"
	"github.com/vonshlovens/obsync-pg/internal/watcher"
)

// Engine handles file synchronization logic
type Engine struct {
	db              *db.DB
	config          *config.Config
	state           *StateTracker
	parser          *parser.Parser
	retryQueue      map[string]int // path -> retry count
	maxBinarySize   int64
}

// NewEngine creates a new sync engine
func NewEngine(database *db.DB, cfg *config.Config) (*Engine, error) {
	state, err := NewStateTracker(cfg.VaultPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create state tracker: %w", err)
	}

	return &Engine{
		db:            database,
		config:        cfg,
		state:         state,
		parser:        parser.NewParser(),
		retryQueue:    make(map[string]int),
		maxBinarySize: int64(cfg.Sync.MaxBinarySizeMB) * 1024 * 1024,
	}, nil
}

// SyncFile syncs a single file based on event type
func (e *Engine) SyncFile(ctx context.Context, relPath string, eventType watcher.EventType) error {
	start := time.Now()

	switch eventType {
	case watcher.EventDelete:
		return e.RemoveFile(ctx, relPath)
	case watcher.EventCreate, watcher.EventModify:
		return e.upsertFile(ctx, relPath)
	default:
		return nil
	}

	slog.Debug("sync completed", "path", relPath, "duration_ms", time.Since(start).Milliseconds())
	return nil
}

// upsertFile creates or updates a file in the database
func (e *Engine) upsertFile(ctx context.Context, relPath string) error {
	absPath := filepath.Join(e.config.VaultPath, relPath)

	// Get file info
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File no longer exists
		}
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return nil // Skip directories
	}

	// Check if file should be ignored
	if e.shouldIgnore(relPath) {
		return nil
	}

	// Compute hash
	hash, err := HashFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to hash file: %w", err)
	}

	// Check if sync is needed
	if !e.state.NeedsSync(relPath, hash) {
		slog.Debug("file unchanged, skipping", "path", relPath)
		return nil
	}

	// Determine file type and sync accordingly
	if strings.HasSuffix(strings.ToLower(relPath), ".md") {
		if err := e.syncNote(ctx, relPath, absPath, hash, info.Size()); err != nil {
			return err
		}
	} else {
		if err := e.syncAttachment(ctx, relPath, absPath, hash, info.Size()); err != nil {
			return err
		}
	}

	// Update state
	e.state.SetFileState(relPath, &FileState{
		Hash:         hash,
		LastSynced:   time.Now(),
		LastModified: info.ModTime(),
		SizeBytes:    info.Size(),
	})

	slog.Info("file synced", "path", relPath, "hash", hash[:8])
	return nil
}

// syncNote parses and syncs a markdown note
func (e *Engine) syncNote(ctx context.Context, relPath, absPath, hash string, size int64) error {
	parsed, err := e.parser.ParseFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to parse note: %w", err)
	}

	// Get file timestamps
	created, modified, _ := parser.GetFileTimestamps(absPath)

	// Use frontmatter dates if available
	if parsed.Frontmatter.Created != nil {
		created = parsed.Frontmatter.Created
	}
	if parsed.Frontmatter.Modified != nil {
		modified = parsed.Frontmatter.Modified
	}

	// Merge tags
	allTags := parser.MergeTags(parsed.Frontmatter.Tags, parsed.InlineTags)

	// Build note struct
	note := &db.VaultNote{
		Path:          relPath,
		Filename:      filepath.Base(relPath),
		Title:         parsed.Frontmatter.Title,
		Tags:          allTags,
		Aliases:       parsed.Frontmatter.Aliases,
		CreatedAt:     created,
		ModifiedAt:    modified,
		Publish:       parsed.Frontmatter.Publish != nil && *parsed.Frontmatter.Publish,
		Frontmatter:   parsed.Frontmatter.Extra,
		Body:          parsed.Body,
		RawContent:    parsed.RawContent,
		ContentHash:   hash,
		FileSizeBytes: size,
		OutgoingLinks: parsed.OutgoingLinks,
	}

	return e.db.UpsertNote(ctx, note)
}

// syncAttachment syncs a binary/attachment file
func (e *Engine) syncAttachment(ctx context.Context, relPath, absPath, hash string, size int64) error {
	// Skip if too large
	if size > e.maxBinarySize {
		slog.Warn("attachment too large, skipping",
			"path", relPath,
			"size_mb", size/(1024*1024),
			"max_mb", e.config.Sync.MaxBinarySizeMB)
		return nil
	}

	// Read file content
	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read attachment: %w", err)
	}

	// Detect mime type
	mimeType := http.DetectContentType(data)
	ext := filepath.Ext(relPath)

	att := &db.VaultAttachment{
		Path:          relPath,
		Filename:      filepath.Base(relPath),
		Extension:     &ext,
		MimeType:      &mimeType,
		FileSizeBytes: size,
		ContentHash:   hash,
		Data:          data,
	}

	return e.db.UpsertAttachment(ctx, att)
}

// RemoveFile removes a file from the database
func (e *Engine) RemoveFile(ctx context.Context, relPath string) error {
	if strings.HasSuffix(strings.ToLower(relPath), ".md") {
		if err := e.db.DeleteNote(ctx, relPath); err != nil {
			return err
		}
	} else {
		if err := e.db.DeleteAttachment(ctx, relPath); err != nil {
			return err
		}
	}

	e.state.RemoveFileState(relPath)
	slog.Info("file removed", "path", relPath)
	return nil
}

// FullReconcile performs a full sync of the vault
func (e *Engine) FullReconcile(ctx context.Context) error {
	slog.Info("starting full reconciliation")
	start := time.Now()

	// Collect all local files
	var localFiles []string
	localHashes := make(map[string]string)

	err := filepath.WalkDir(e.config.VaultPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		relPath, _ := filepath.Rel(e.config.VaultPath, path)
		relPath = filepath.ToSlash(relPath)

		if e.shouldIgnore(relPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		localFiles = append(localFiles, relPath)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk vault: %w", err)
	}

	// Get DB hashes
	dbNoteHashes, err := e.db.GetAllNoteHashes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get note hashes: %w", err)
	}

	dbAttachHashes, err := e.db.GetAllAttachmentHashes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get attachment hashes: %w", err)
	}

	// Merge DB hashes
	dbHashes := make(map[string]string)
	for k, v := range dbNoteHashes {
		dbHashes[k] = v
	}
	for k, v := range dbAttachHashes {
		dbHashes[k] = v
	}

	// Compute local hashes and find files to sync
	var toSync []string

	bar := progressbar.NewOptions(len(localFiles),
		progressbar.OptionSetDescription("Scanning files"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionClearOnFinish(),
	)

	for _, relPath := range localFiles {
		bar.Add(1)

		absPath := filepath.Join(e.config.VaultPath, relPath)
		hash, err := HashFile(absPath)
		if err != nil {
			slog.Warn("failed to hash file", "path", relPath, "error", err)
			continue
		}
		localHashes[relPath] = hash

		// Check if file needs sync
		if dbHash, exists := dbHashes[relPath]; !exists || dbHash != hash {
			toSync = append(toSync, relPath)
		}
	}
	bar.Finish()

	// Find files to delete from DB (exist in DB but not locally)
	var toDelete []string
	for dbPath := range dbHashes {
		if _, exists := localHashes[dbPath]; !exists {
			toDelete = append(toDelete, dbPath)
		}
	}

	// Sync changed/new files
	if len(toSync) > 0 {
		bar = progressbar.NewOptions(len(toSync),
			progressbar.OptionSetDescription("Syncing files"),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWidth(40),
		)

		for _, relPath := range toSync {
			if err := e.upsertFile(ctx, relPath); err != nil {
				slog.Error("failed to sync file", "path", relPath, "error", err)
				// Add to retry queue
				e.retryQueue[relPath] = 0
			}
			bar.Add(1)
		}
		bar.Finish()
	}

	// Delete removed files
	if len(toDelete) > 0 {
		var notesToDelete, attachmentsToDelete []string
		for _, path := range toDelete {
			if strings.HasSuffix(strings.ToLower(path), ".md") {
				notesToDelete = append(notesToDelete, path)
			} else {
				attachmentsToDelete = append(attachmentsToDelete, path)
			}
		}

		if len(notesToDelete) > 0 {
			if err := e.db.BatchDeleteNotes(ctx, notesToDelete); err != nil {
				slog.Error("failed to batch delete notes", "error", err)
			}
		}
		if len(attachmentsToDelete) > 0 {
			if err := e.db.BatchDeleteAttachments(ctx, attachmentsToDelete); err != nil {
				slog.Error("failed to batch delete attachments", "error", err)
			}
		}

		for _, path := range toDelete {
			e.state.RemoveFileState(path)
		}

		slog.Info("deleted removed files", "count", len(toDelete))
	}

	// Update state
	e.state.SetLastFullSync(time.Now())
	if err := e.state.Save(); err != nil {
		slog.Warn("failed to save state", "error", err)
	}

	slog.Info("full reconciliation completed",
		"synced", len(toSync),
		"deleted", len(toDelete),
		"duration_s", time.Since(start).Seconds())

	return nil
}

// PullFromDB downloads files from database to local vault (for new device setup)
func (e *Engine) PullFromDB(ctx context.Context) error {
	slog.Info("pulling files from database to local vault")
	start := time.Now()

	// Get all notes
	notes, err := e.db.GetAllNotes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get notes: %w", err)
	}

	// Get all attachments
	attachments, err := e.db.GetAllAttachments(ctx)
	if err != nil {
		return fmt.Errorf("failed to get attachments: %w", err)
	}

	totalFiles := len(notes) + len(attachments)
	if totalFiles == 0 {
		slog.Info("no files in database to pull")
		return nil
	}

	bar := progressbar.NewOptions(totalFiles,
		progressbar.OptionSetDescription("Pulling files"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
	)

	// Write notes
	for _, note := range notes {
		absPath := filepath.Join(e.config.VaultPath, note.Path)

		// Check if file already exists with same hash
		if existingHash, err := HashFile(absPath); err == nil && existingHash == note.ContentHash {
			bar.Add(1)
			continue
		}

		// Create directory if needed
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			slog.Error("failed to create directory", "dir", dir, "error", err)
			bar.Add(1)
			continue
		}

		// Write file
		if err := os.WriteFile(absPath, []byte(note.RawContent), 0644); err != nil {
			slog.Error("failed to write note", "path", note.Path, "error", err)
		} else {
			slog.Info("pulled note", "path", note.Path)
		}
		bar.Add(1)
	}

	// Write attachments
	for _, att := range attachments {
		absPath := filepath.Join(e.config.VaultPath, att.Path)

		// Check if file already exists with same hash
		if existingHash, err := HashFile(absPath); err == nil && existingHash == att.ContentHash {
			bar.Add(1)
			continue
		}

		// Create directory if needed
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			slog.Error("failed to create directory", "dir", dir, "error", err)
			bar.Add(1)
			continue
		}

		// Write file
		if err := os.WriteFile(absPath, att.Data, 0644); err != nil {
			slog.Error("failed to write attachment", "path", att.Path, "error", err)
		} else {
			slog.Info("pulled attachment", "path", att.Path)
		}
		bar.Add(1)
	}

	bar.Finish()

	slog.Info("pull completed",
		"notes", len(notes),
		"attachments", len(attachments),
		"duration_s", time.Since(start).Seconds())

	return nil
}

// RetryFailed retries failed sync operations
func (e *Engine) RetryFailed(ctx context.Context) {
	maxRetries := e.config.Sync.RetryAttempts

	for path, count := range e.retryQueue {
		if count >= maxRetries {
			slog.Error("max retries exceeded", "path", path)
			delete(e.retryQueue, path)
			continue
		}

		e.retryQueue[path] = count + 1
		if err := e.upsertFile(ctx, path); err != nil {
			slog.Warn("retry failed", "path", path, "attempt", count+1, "error", err)
		} else {
			delete(e.retryQueue, path)
			slog.Info("retry succeeded", "path", path)
		}
	}
}

// SaveState persists the current state to disk
func (e *Engine) SaveState() error {
	return e.state.Save()
}

// shouldIgnore checks if a path should be ignored
func (e *Engine) shouldIgnore(relPath string) bool {
	for _, pattern := range e.config.IgnorePatterns {
		matched, err := doublestar.Match(pattern, relPath)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}

	// Check include patterns
	if len(e.config.IncludePatterns) > 0 {
		for _, pattern := range e.config.IncludePatterns {
			matched, err := doublestar.Match(pattern, relPath)
			if err != nil {
				continue
			}
			if matched {
				return false
			}
		}
		return true // Didn't match any include pattern
	}

	return false
}

// GetPendingRetries returns count of files pending retry
func (e *Engine) GetPendingRetries() int {
	return len(e.retryQueue)
}
