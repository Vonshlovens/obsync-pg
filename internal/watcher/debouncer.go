package watcher

import (
	"sync"
	"time"
)

// EventType represents the type of file event
type EventType int

const (
	EventCreate EventType = iota
	EventModify
	EventDelete
	EventRename
)

func (e EventType) String() string {
	switch e {
	case EventCreate:
		return "CREATE"
	case EventModify:
		return "MODIFY"
	case EventDelete:
		return "DELETE"
	case EventRename:
		return "RENAME"
	default:
		return "UNKNOWN"
	}
}

// FileEvent represents a debounced file event
type FileEvent struct {
	Path      string
	EventType EventType
	Timestamp time.Time
}

// Debouncer collects and coalesces rapid file events
type Debouncer struct {
	delay    time.Duration
	events   map[string]*pendingEvent
	mu       sync.Mutex
	output   chan FileEvent
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

type pendingEvent struct {
	event    FileEvent
	timer    *time.Timer
}

// NewDebouncer creates a new event debouncer
func NewDebouncer(delayMs int) *Debouncer {
	return &Debouncer{
		delay:  time.Duration(delayMs) * time.Millisecond,
		events: make(map[string]*pendingEvent),
		output: make(chan FileEvent, 100),
		stopCh: make(chan struct{}),
	}
}

// Events returns the channel of debounced events
func (d *Debouncer) Events() <-chan FileEvent {
	return d.output
}

// Add adds a new event to be debounced
func (d *Debouncer) Add(path string, eventType EventType) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if we're stopped
	select {
	case <-d.stopCh:
		return
	default:
	}

	event := FileEvent{
		Path:      path,
		EventType: eventType,
		Timestamp: time.Now(),
	}

	// Coalesce events for the same path
	if pending, exists := d.events[path]; exists {
		// Stop existing timer
		pending.timer.Stop()

		// Coalesce event types
		// DELETE always wins (file is gone)
		// CREATE + MODIFY = CREATE (new file modified)
		// MODIFY + MODIFY = MODIFY
		if eventType == EventDelete {
			pending.event.EventType = EventDelete
		} else if pending.event.EventType == EventCreate && eventType == EventModify {
			// Keep as CREATE
		} else if pending.event.EventType != EventDelete {
			pending.event.EventType = eventType
		}
		pending.event.Timestamp = event.Timestamp

		// Reset timer
		pending.timer = time.AfterFunc(d.delay, func() {
			d.emit(path)
		})
	} else {
		// New event
		d.events[path] = &pendingEvent{
			event: event,
			timer: time.AfterFunc(d.delay, func() {
				d.emit(path)
			}),
		}
	}
}

// emit sends an event to the output channel
func (d *Debouncer) emit(path string) {
	d.mu.Lock()
	pending, exists := d.events[path]
	if exists {
		delete(d.events, path)
	}
	d.mu.Unlock()

	if exists {
		select {
		case d.output <- pending.event:
		case <-d.stopCh:
		}
	}
}

// Flush immediately emits all pending events
func (d *Debouncer) Flush() {
	d.mu.Lock()
	paths := make([]string, 0, len(d.events))
	for path, pending := range d.events {
		pending.timer.Stop()
		paths = append(paths, path)
	}
	d.mu.Unlock()

	for _, path := range paths {
		d.emit(path)
	}
}

// Stop stops the debouncer and flushes remaining events
func (d *Debouncer) Stop() {
	close(d.stopCh)

	d.mu.Lock()
	for _, pending := range d.events {
		pending.timer.Stop()
	}
	d.events = make(map[string]*pendingEvent)
	d.mu.Unlock()

	close(d.output)
}

// PendingCount returns the number of pending events
func (d *Debouncer) PendingCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.events)
}
