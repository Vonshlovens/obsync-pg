package watcher

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fsnotify/fsnotify"
)

// Watcher monitors a directory for file changes
type Watcher struct {
	rootPath       string
	watcher        *fsnotify.Watcher
	debouncer      *Debouncer
	ignorePatterns []string
	includePatterns []string
	stopCh         chan struct{}
}

// NewWatcher creates a new file watcher
func NewWatcher(rootPath string, debounceMs int, ignorePatterns, includePatterns []string) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		rootPath:        rootPath,
		watcher:         fsWatcher,
		debouncer:       NewDebouncer(debounceMs),
		ignorePatterns:  ignorePatterns,
		includePatterns: includePatterns,
		stopCh:          make(chan struct{}),
	}, nil
}

// Start begins watching the root directory and all subdirectories
func (w *Watcher) Start(ctx context.Context) error {
	// Add all directories recursively
	if err := w.addRecursive(w.rootPath); err != nil {
		return err
	}

	// Start event processing goroutine
	go w.processEvents(ctx)

	slog.Info("watcher started",
		"path", w.rootPath,
		"ignore_patterns", len(w.ignorePatterns))

	return nil
}

// Events returns the channel of debounced file events
func (w *Watcher) Events() <-chan FileEvent {
	return w.debouncer.Events()
}

// Stop stops the watcher
func (w *Watcher) Stop() error {
	close(w.stopCh)
	w.debouncer.Stop()
	return w.watcher.Close()
}

// addRecursive adds a directory and all subdirectories to the watcher
func (w *Watcher) addRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			slog.Warn("error walking path", "path", path, "error", err)
			return nil // Continue walking
		}

		// Skip if matches ignore pattern
		relPath, _ := filepath.Rel(w.rootPath, path)
		relPath = filepath.ToSlash(relPath) // Normalize to forward slashes

		if w.shouldIgnore(relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Only watch directories
		if info.IsDir() {
			if err := w.watcher.Add(path); err != nil {
				slog.Warn("failed to watch directory", "path", path, "error", err)
			}
		}

		return nil
	})
}

// processEvents handles fsnotify events
func (w *Watcher) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			relPath, err := filepath.Rel(w.rootPath, event.Name)
			if err != nil {
				continue
			}
			relPath = filepath.ToSlash(relPath)

			// Check ignore patterns
			if w.shouldIgnore(relPath) {
				continue
			}

			// Check include patterns (if specified)
			if !w.shouldInclude(relPath) {
				continue
			}

			// Handle the event
			w.handleEvent(event, relPath)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("watcher error", "error", err)
		}
	}
}

// handleEvent processes a single fsnotify event
func (w *Watcher) handleEvent(event fsnotify.Event, relPath string) {
	// Get file info for directory detection
	info, statErr := os.Stat(event.Name)

	switch {
	case event.Has(fsnotify.Create):
		// If it's a new directory, add it to watcher
		if statErr == nil && info.IsDir() {
			if err := w.addRecursive(event.Name); err != nil {
				slog.Warn("failed to add new directory", "path", event.Name, "error", err)
			}
			return // Don't emit events for directories
		}
		w.debouncer.Add(relPath, EventCreate)

	case event.Has(fsnotify.Write):
		if statErr == nil && info.IsDir() {
			return // Ignore directory modifications
		}
		w.debouncer.Add(relPath, EventModify)

	case event.Has(fsnotify.Remove):
		w.debouncer.Add(relPath, EventDelete)

	case event.Has(fsnotify.Rename):
		// Rename is treated as delete (the new name will trigger a create)
		w.debouncer.Add(relPath, EventDelete)

	case event.Has(fsnotify.Chmod):
		// Ignore chmod events
	}
}

// shouldIgnore checks if a path matches any ignore pattern
func (w *Watcher) shouldIgnore(relPath string) bool {
	for _, pattern := range w.ignorePatterns {
		matched, err := doublestar.Match(pattern, relPath)
		if err != nil {
			continue
		}
		if matched {
			return true
		}

		// Also check if any parent directory matches
		parts := strings.Split(relPath, "/")
		for i := 1; i <= len(parts); i++ {
			partial := strings.Join(parts[:i], "/")
			if matched, _ := doublestar.Match(pattern, partial); matched {
				return true
			}
		}
	}
	return false
}

// shouldInclude checks if a path matches include patterns (or returns true if no patterns)
func (w *Watcher) shouldInclude(relPath string) bool {
	if len(w.includePatterns) == 0 {
		return true // No include patterns means include everything
	}

	for _, pattern := range w.includePatterns {
		matched, err := doublestar.Match(pattern, relPath)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// Flush flushes all pending debounced events
func (w *Watcher) Flush() {
	w.debouncer.Flush()
}
