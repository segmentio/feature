package feature

import (
	"log"
	"time"
)

// Store is similar to Cache, but automatically reloads when updates are made
// to the underlying file system.
type Store struct {
	path  MountPoint
	cache Cache
	watch *Watcher
}

// Close closes the store, releasing all associated resources.
func (s *Store) Close() error {
	err := s.watch.Close()
	s.cache.Close()
	return err
}

// GateOpened returns true if a gate is opened for a given id.
func (s *Store) GateOpen(family, gate, collection, id string) bool {
	return s.cache.GateOpen(family, gate, collection, id)
}

// AppendGates appends the list of open gates in a family for a given id.
func (s *Store) AppendGates(gates []string, family, collection, id string) []string {
	return s.cache.AppendGates(gates, family, collection, id)
}

// LookupGates returns the list of open gates in a family for a given id.
func (s *Store) LookupGates(family, collection, id string) []string {
	return s.cache.LookupGates(family, collection, id)
}

func (s *Store) run() {
	for {
		select {
		case _, ok := <-s.watch.Events:
			if !ok {
				return
			}
			log.Printf("NOTICE feature - %s - reloading feature gates", s.path)

			c, err := s.path.Load()
			if err != nil {
				log.Printf("ERROR feature - %s - %s", s.path, err)
			} else {
				s.cache.mutex.Lock()
				s.cache.tiers, c.tiers = c.tiers, s.cache.tiers
				s.cache.mutex.Unlock()
				c.Close()
			}

		case err, ok := <-s.watch.Errors:
			if !ok {
				return
			}
			log.Printf("ERROR feature - %s - %s", s.path, err)
		}
	}
}

// The Open method opens the features at the mount point it was called on,
// returning a Store object exposing the state.
//
// The returned store holds operating system resources and therefore must be
// closed when the program does not need it anymore.
func (path MountPoint) Open(period time.Duration) (*Store, error) {
	w, err := path.Watch(period)
	if err != nil {
		return nil, err
	}

	c, err := path.Load()
	if err != nil {
		w.Close()
		return nil, err
	}

	s := &Store{
		path:  path,
		watch: w,
		cache: Cache{
			tiers: c.tiers,
		},
	}

	go s.run()
	return s, nil
}
