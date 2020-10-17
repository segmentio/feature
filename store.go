package feature

import (
	"log"
)

// Store is similar to Cache, but automatically reloads when updates are made
// to the underlying file system.
type Store struct {
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
	w, err := path.Watch()
	if err != nil {
		return nil, err
	}

	c, err := path.Load()
	if err != nil {
		w.Close()
		return nil, err
	}

	s := &Store{
		watch: w,
		cache: Cache{
			tiers: c.tiers,
		},
	}

	go path.watch(s)
	return s, nil
}

func (path MountPoint) watch(s *Store) {
	for {
		select {
		case _, ok := <-s.watch.Events:
			if !ok {
				return
			}
			log.Printf("NOTICE feature - %s - reloading feature gates", path)

			c, err := path.Load()
			if err != nil {
				log.Printf("ERROR feature - %s - %s", path, err)
			} else {
				s.cache.mutex.Lock()
				s.cache.tiers, c.tiers = c.tiers, s.cache.tiers
				s.cache.cache.clear()
				s.cache.mutex.Unlock()
				c.Close()
			}

		case err, ok := <-s.watch.Errors:
			if !ok {
				return
			}
			log.Printf("ERROR feature - %s - %s", path, err)
		}
	}
}
