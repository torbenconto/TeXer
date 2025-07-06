package watcher

import (
	"time"
)

type EventType uint32

// Rename/move events are split into a delete and create event
const (
	EventCreate EventType = 1 << iota
	EventModify
	EventDelete
	EventChmod
)

type Event struct {
	Path      string
	Type      EventType
	IsDir     bool
	Timestamp time.Time
}

type Watcher struct {
	interval time.Duration
	prev     map[string]*Node
	watched  map[string]struct{}
	ignore   func(string) bool
	events   chan Event
	errors   chan error
	stop     chan struct{}
}

func NewWatcher(interval time.Duration, ignore func(string) bool) *Watcher {
	return &Watcher{
		interval: interval,
		events:   make(chan Event, 1024),
		errors:   make(chan error, 16),
		stop:     make(chan struct{}),
		watched:  make(map[string]struct{}),
		prev:     make(map[string]*Node),
		ignore:   ignore,
	}
}

func (w *Watcher) Events() <-chan Event {
	return w.events
}

func (w *Watcher) Errors() <-chan error {
	return w.errors
}

func (w *Watcher) poll() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for root := range w.watched {
				new, err := buildTree(root, w.ignore)
				if err != nil {
					w.errors <- err
					continue
				}

				old := w.prev[root]
				if old == nil {
					recursiveEmit(new, EventCreate, w.events, true)
				} else {
					diff(old, new, w.events)
				}

				w.prev[root] = new
			}
		case <-w.stop:
			close(w.errors)
			close(w.events)
			return
		}
	}
}

func (w *Watcher) Start() error {
	go w.poll()
	return nil
}

func (w *Watcher) Close() error {
	close(w.stop)
	return nil
}

func (w *Watcher) Add(path string) error {
	w.watched[path] = struct{}{}

	return nil
}
