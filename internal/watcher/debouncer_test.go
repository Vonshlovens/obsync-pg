package watcher

import (
	"testing"
	"time"
)

func TestDebouncer_SingleEvent(t *testing.T) {
	d := NewDebouncer(50) // 50ms debounce
	defer d.Stop()

	d.Add("test.md", EventCreate)

	select {
	case event := <-d.Events():
		if event.Path != "test.md" {
			t.Errorf("expected path 'test.md', got %q", event.Path)
		}
		if event.EventType != EventCreate {
			t.Errorf("expected EventCreate, got %v", event.EventType)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timed out waiting for event")
	}
}

func TestDebouncer_CoalesceWrites(t *testing.T) {
	d := NewDebouncer(100) // 100ms debounce
	defer d.Stop()

	// Rapid writes to same file
	d.Add("test.md", EventModify)
	d.Add("test.md", EventModify)
	d.Add("test.md", EventModify)

	// Should only get one event
	eventCount := 0
	timeout := time.After(300 * time.Millisecond)

loop:
	for {
		select {
		case <-d.Events():
			eventCount++
		case <-timeout:
			break loop
		}
	}

	if eventCount != 1 {
		t.Errorf("expected 1 coalesced event, got %d", eventCount)
	}
}

func TestDebouncer_DeleteWins(t *testing.T) {
	d := NewDebouncer(100)
	defer d.Stop()

	// Create then delete
	d.Add("test.md", EventCreate)
	d.Add("test.md", EventDelete)

	select {
	case event := <-d.Events():
		if event.EventType != EventDelete {
			t.Errorf("expected EventDelete to win, got %v", event.EventType)
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("timed out waiting for event")
	}
}

func TestDebouncer_CreateThenModify(t *testing.T) {
	d := NewDebouncer(100)
	defer d.Stop()

	// Create then modify should stay as Create
	d.Add("test.md", EventCreate)
	d.Add("test.md", EventModify)

	select {
	case event := <-d.Events():
		if event.EventType != EventCreate {
			t.Errorf("expected EventCreate (create+modify), got %v", event.EventType)
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("timed out waiting for event")
	}
}

func TestDebouncer_MultipleFiles(t *testing.T) {
	d := NewDebouncer(50)
	defer d.Stop()

	d.Add("file1.md", EventCreate)
	d.Add("file2.md", EventModify)

	received := make(map[string]bool)
	timeout := time.After(200 * time.Millisecond)

loop:
	for {
		select {
		case event := <-d.Events():
			received[event.Path] = true
			if len(received) == 2 {
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	if !received["file1.md"] || !received["file2.md"] {
		t.Errorf("expected both files, got %v", received)
	}
}

func TestDebouncer_Flush(t *testing.T) {
	d := NewDebouncer(5000) // Long debounce
	defer d.Stop()

	d.Add("test.md", EventCreate)

	// Pending should be 1
	if d.PendingCount() != 1 {
		t.Errorf("expected 1 pending, got %d", d.PendingCount())
	}

	// Flush should emit immediately
	d.Flush()

	select {
	case event := <-d.Events():
		if event.Path != "test.md" {
			t.Errorf("expected path 'test.md', got %q", event.Path)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("flush should emit immediately")
	}

	if d.PendingCount() != 0 {
		t.Errorf("expected 0 pending after flush, got %d", d.PendingCount())
	}
}

func TestEventType_String(t *testing.T) {
	tests := []struct {
		event    EventType
		expected string
	}{
		{EventCreate, "CREATE"},
		{EventModify, "MODIFY"},
		{EventDelete, "DELETE"},
		{EventRename, "RENAME"},
		{EventType(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if tt.event.String() != tt.expected {
			t.Errorf("EventType(%d).String() = %q, want %q", tt.event, tt.event.String(), tt.expected)
		}
	}
}
