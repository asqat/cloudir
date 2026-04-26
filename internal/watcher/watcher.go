package watcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type EventType int

const (
	Create EventType = iota
	Write
	Remove
	Rename
)

type Event struct {
	Op   EventType
	Path string
}

type Watcher struct {
	fsWatcher *fsnotify.Watcher
	Events    chan Event
	ignored   []string
}

func NewWatcher(ignored []string) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		fsWatcher: fsWatcher,
		Events:    make(chan Event, 100),
		ignored:   ignored,
	}, nil
}

func (w *Watcher) Watch(root string) error {
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if w.isIgnored(path) {
				return filepath.SkipDir
			}
			return w.fsWatcher.Add(path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	go w.start()
	return nil
}

func (w *Watcher) isIgnored(path string) bool {
	for _, ignore := range w.ignored {
		if strings.Contains(path, ignore) {
			return true
		}
	}
	return false
}

func (w *Watcher) start() {
	// Debouncing logic
	timers := make(map[string]*time.Timer)
	const debounceDuration = 500 * time.Millisecond

	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			if w.isIgnored(event.Name) {
				continue
			}

			// Handle directory additions
			if event.Op&fsnotify.Create == fsnotify.Create {
				info, err := os.Stat(event.Name)
				if err == nil && info.IsDir() {
					w.fsWatcher.Add(event.Name)
				}
			}

			// Debounce
			if timer, ok := timers[event.Name]; ok {
				timer.Stop()
			}

			timers[event.Name] = time.AfterFunc(debounceDuration, func() {
				var op EventType
				switch {
				case event.Op&fsnotify.Create == fsnotify.Create:
					op = Create
				case event.Op&fsnotify.Write == fsnotify.Write:
					op = Write
				case event.Op&fsnotify.Remove == fsnotify.Remove:
					op = Remove
				case event.Op&fsnotify.Rename == fsnotify.Rename:
					op = Rename
				default:
					return
				}
				w.Events <- Event{Op: op, Path: event.Name}
			})

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

func (w *Watcher) Close() error {
	return w.fsWatcher.Close()
}
