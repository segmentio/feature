package feature

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/segmentio/fs"
)

// Store is similar to Cache, but automatically reloads when updates are made
// to the underlying file system.
type Store struct {
	cache  Cache
	once   sync.Once
	join   sync.WaitGroup
	done   chan struct{}
	notify chan string
}

// Close closes the store, releasing all associated resources.
func (s *Store) Close() error {
	s.once.Do(func() { close(s.done) })
	s.join.Wait()
	s.cache.Close()
	return nil
}

// GateOpened returns true if a gate is opened for a given id.
func (s *Store) GateOpen(family, gate, collection, id string) bool {
	return s.cache.GateOpen(family, gate, collection, id)
}

// LookupGates returns the list of open gates in a family for a given id.
func (s *Store) LookupGates(family, collection, id string) []string {
	return s.cache.LookupGates(family, collection, id)
}

// The Open method opens the features at the mount point it was called on,
// returning a Store object exposing the state.
//
// The returned store holds operating system resources and therefore must be
// closed when the program does not need it anymore.
func (path MountPoint) Open() (*Store, error) {
	notify := make(chan string)

	if err := fs.Notify(notify, string(path)); err != nil {
		return nil, err
	}

	c, err := path.Load()
	if err != nil {
		fs.Stop(notify)
		return nil, err
	}

	s := &Store{
		cache:  Cache{tiers: c.tiers},
		done:   make(chan struct{}),
		notify: notify,
	}

	s.join.Add(1)
	go path.watch(s)
	return s, nil
}

func (path MountPoint) watch(s *Store) {
	defer s.join.Done()
	defer fs.Stop(s.notify)
	log.Printf("NOTICE feature - %s - watching for changes on the feature database", path)

	for {
		select {
		case <-s.notify:
			log.Printf("INFO feature - %s - reloading feature database after detecting update", path)
			if err := fs.Notify(s.notify, string(path)); err != nil {
				log.Printf("CRIT feature - %s - %s", path, err)
			}
			start := time.Now()
			c, err := path.Load()
			if err != nil {
				log.Printf("ERROR feature - %s - %s", path, err)
			} else {
				log.Printf("NOTICE feature - %s - feature database reloaded in %gs", path, time.Since(start).Round(time.Millisecond).Seconds())
				c = s.cache.swap(c)
				c.Close()
			}
		case <-s.done:
			return
		}
	}
}

// Wait blocks until the path exists or ctx is cancelled.
func (path MountPoint) Wait(ctx context.Context) error {
	notify := make(chan string)
	defer fs.Stop(notify)
	for {
		if err := fs.Notify(notify, filepath.Dir(string(path))); err != nil {
			return err
		}
		_, err := os.Lstat(string(path))
		if err == nil {
			log.Printf("INFO feature - %s - feature database exists", path)
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
		log.Printf("NOTICE feature - %s - waiting for feature database to be created", path)
		select {
		case <-notify:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
