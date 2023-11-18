package storage

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

const (
	JSONFormat Format = "json"
	GobFormat  Format = "gob"
)

type Format string

// Storage is a simple in-memory key-value storage.
type Storage struct {
	storage map[string]Object
	mu      sync.RWMutex

	// wg is used to wait additional goroutines on the storage shutdown.
	// It's important to wait while saver goroutine saves the storage to the disk.
	wg sync.WaitGroup
}

// Object is a value stored in the storage.
type Object struct {
	Data    []byte
	Expires *time.Time
}

type options struct {
	janitor    *Janitor
	persistent *Persistent
}

type Janitor struct {
	// Interval is an interval between the storage cleanup.
	//
	// Default interval is 15 second.
	Interval time.Duration
}

// Persistent is a configuration for the storage persistence.
type Persistent struct {
	// Format is a format of the storage persistence.
	//
	// GOB format is used by default.
	//
	// The JSON format is also available, but it's a lot slower and consumes
	// 2 times more memory.
	Format Format

	// Interval is an interval between the storage persistence.
	//
	// Default interval is 15 seconds.
	Interval time.Duration

	// FileName is a name of the file where the storage will be saved.
	//
	// Default file name is "go-persista".
	Name string
}

type Option func(options *options)

// New creates a new storage.
func New(ctx context.Context, opts ...Option) *Storage {
	st := &Storage{
		storage: make(map[string]Object),
		mu:      sync.RWMutex{},
	}

	options := options{}
	for _, opt := range opts {
		opt(&options)
	}

	if options.janitor != nil {
		st.wg.Add(1)
		go st.janitor(ctx, options.janitor.Interval)
		log.Println("janitor enabled")
	}

	if options.persistent != nil {
		cnt, err := st.load(options.persistent.Name)
		if err != nil {
			log.Printf("load error: %v", err)
		} else {
			log.Printf("loaded %d objects from %s", cnt, options.persistent.Name)
		}

		st.wg.Add(1)
		go st.saver(ctx, *options.persistent)
		log.Printf("persistent to %s enabled in %s format every %s", options.persistent.Name, options.persistent.Format, options.persistent.Interval)
	}

	return st
}

// Shutdown waits while all goroutines are finished.
func (s *Storage) Shutdown() {
	s.wg.Wait()
}

// Put stores the data in the storage under the given key.
func (s *Storage) Put(key string, data []byte, expires *time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.storage[key] = Object{
		Data:    data,
		Expires: expires,
	}
}

// Get retrieves the data from the storage by the given key.
func (s *Storage) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	obj, ok := s.storage[key]
	if !ok {
		return nil, false
	}

	if obj.Expires != nil && obj.Expires.Before(time.Now()) {
		delete(s.storage, key)
		return nil, false
	}

	return obj.Data, true
}

// WithPersistent enables the persistence of the storage.
//
// The storage will be saved to the disk by the given interval and format.
func WithPersistent(persistent Persistent) Option {
	return func(options *options) {
		if persistent.Format == "" {
			persistent.Format = GobFormat
		}

		if persistent.Interval == 0 {
			persistent.Interval = 15 * time.Second
		}

		if persistent.Name == "" {
			persistent.Name = "go-persista"
		}

		options.persistent = &persistent
	}
}

func (s *Storage) saver(ctx context.Context, p Persistent) {
	defer s.wg.Done()

	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("tryna save your storage ... 😰")

			cnt, err := s.save(p.Name, p.Format)
			if err != nil {
				log.Printf("save on exit error: %v", err)
			} else {
				log.Printf("💪 saved %d objects to %s", cnt, p.Name)
			}

			return
		case <-ticker.C:
			cnt, err := s.save(p.Name, p.Format)
			if err != nil {
				log.Printf("save error: %v", err)
			} else {
				log.Printf("saved %d objects to %s", cnt, p.Name)
			}
		}
	}
}

func (s *Storage) save(name string, format Format) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, err := os.Create(fmt.Sprintf("%s.%s", name, format))
	if err != nil {
		return 0, fmt.Errorf("create file error: %w", err)
	}
	defer file.Close()

	switch format {
	case JSONFormat:
		if err := json.NewEncoder(file).Encode(s.storage); err != nil {
			return 0, fmt.Errorf("json encode error: %w", err)
		}
	case GobFormat:
		if err := gob.NewEncoder(file).Encode(s.storage); err != nil {
			return 0, fmt.Errorf("gob encode error: %w", err)
		}
	}

	return len(s.storage), nil
}

// load loads the storage from the disk.
//
// It's called on the storage initialization.
// It's trying to load gob format first, then json.
func (s *Storage) load(name string) (int, error) {
	format := GobFormat

	file, err := os.Open(fmt.Sprintf("%s.%s", name, format))
	if err != nil {
		if !os.IsNotExist(err) {
			return 0, fmt.Errorf("open gob file error: %w", err)
		}

		format = JSONFormat

		file, err = os.Open(fmt.Sprintf("%s.%s", name, format))
		if err != nil {
			if !os.IsNotExist(err) {
				return 0, fmt.Errorf("open json file error: %w", err)
			}

			return 0, nil
		}
	}
	defer file.Close()

	s.mu.Lock()
	defer s.mu.Unlock()

	switch format {
	case JSONFormat:
		if err := json.NewDecoder(file).Decode(&s.storage); err != nil {
			return 0, fmt.Errorf("json decode error: %w", err)
		}
	case GobFormat:
		if err := gob.NewDecoder(file).Decode(&s.storage); err != nil {
			return 0, fmt.Errorf("gob decode error: %w", err)
		}
	}

	return len(s.storage), nil
}

// WithJanitor enables the janitor that cleans up the expired objects
// by the given interval.
//
// The janitor is disabled by default. In this case, the expired objects
// will be cleaned up on the next Get call.
func WithJanitor(janitor Janitor) Option {
	return func(options *options) {
		if janitor.Interval == 0 {
			janitor.Interval = 15 * time.Second
		}

		options.janitor = &janitor
	}
}

func (s *Storage) janitor(ctx context.Context, interval time.Duration) {
	defer s.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			for key, obj := range s.storage {
				if obj.Expires != nil && obj.Expires.Before(time.Now()) {
					delete(s.storage, key)
				}
			}
			s.mu.Unlock()
		}
	}
}
