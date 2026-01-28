package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vonshlovens/obsync-pg/internal/config"
)

// FileState represents the sync state of a single file
type FileState struct {
	Hash         string    `json:"hash"`
	LastSynced   time.Time `json:"last_synced"`
	LastModified time.Time `json:"last_modified"`
	SizeBytes    int64     `json:"size_bytes"`
}

// SyncState represents the local sync state
type SyncState struct {
	VaultPath    string                `json:"vault_path"`
	LastFullSync *time.Time            `json:"last_full_sync,omitempty"`
	Files        map[string]*FileState `json:"files"`
}

// StateTracker manages local sync state
type StateTracker struct {
	state    *SyncState
	filePath string
	mu       sync.RWMutex
	dirty    bool
}

// NewStateTracker creates a new state tracker
func NewStateTracker(vaultPath string) (*StateTracker, error) {
	stateDir, err := config.GetStateDir()
	if err != nil {
		return nil, err
	}

	// Create a unique state file based on vault path hash
	vaultHash := HashString(vaultPath)[:12]
	filePath := filepath.Join(stateDir, "state-"+vaultHash+".json")

	st := &StateTracker{
		filePath: filePath,
		state: &SyncState{
			VaultPath: vaultPath,
			Files:     make(map[string]*FileState),
		},
	}

	// Try to load existing state
	if err := st.load(); err != nil && !os.IsNotExist(err) {
		// Log warning but continue with empty state
	}

	// Verify vault path matches
	if st.state.VaultPath != vaultPath {
		st.state = &SyncState{
			VaultPath: vaultPath,
			Files:     make(map[string]*FileState),
		}
	}

	return st, nil
}

// load reads state from disk
func (st *StateTracker) load() error {
	data, err := os.ReadFile(st.filePath)
	if err != nil {
		return err
	}

	state := &SyncState{}
	if err := json.Unmarshal(data, state); err != nil {
		return err
	}

	if state.Files == nil {
		state.Files = make(map[string]*FileState)
	}

	st.state = state
	return nil
}

// Save persists state to disk
func (st *StateTracker) Save() error {
	st.mu.Lock()
	defer st.mu.Unlock()

	if !st.dirty {
		return nil
	}

	data, err := json.MarshalIndent(st.state, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(st.filePath, data, 0644); err != nil {
		return err
	}

	st.dirty = false
	return nil
}

// GetFileState returns the state for a specific file
func (st *StateTracker) GetFileState(path string) *FileState {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.state.Files[path]
}

// SetFileState updates the state for a specific file
func (st *StateTracker) SetFileState(path string, state *FileState) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.state.Files[path] = state
	st.dirty = true
}

// RemoveFileState removes state for a file
func (st *StateTracker) RemoveFileState(path string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	delete(st.state.Files, path)
	st.dirty = true
}

// GetAllPaths returns all tracked file paths
func (st *StateTracker) GetAllPaths() []string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	paths := make([]string, 0, len(st.state.Files))
	for path := range st.state.Files {
		paths = append(paths, path)
	}
	return paths
}

// SetLastFullSync updates the last full sync time
func (st *StateTracker) SetLastFullSync(t time.Time) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.state.LastFullSync = &t
	st.dirty = true
}

// GetLastFullSync returns the last full sync time
func (st *StateTracker) GetLastFullSync() *time.Time {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.state.LastFullSync
}

// NeedsSync checks if a file needs to be synced based on hash comparison
func (st *StateTracker) NeedsSync(path string, currentHash string) bool {
	st.mu.RLock()
	defer st.mu.RUnlock()

	state, exists := st.state.Files[path]
	if !exists {
		return true
	}

	return state.Hash != currentHash
}

// Clear removes all state
func (st *StateTracker) Clear() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.state.Files = make(map[string]*FileState)
	st.state.LastFullSync = nil
	st.dirty = true
}

// FileCount returns the number of tracked files
func (st *StateTracker) FileCount() int {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return len(st.state.Files)
}
