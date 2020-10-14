package feature

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher values are created by calls to feature.MountPoint.Watch to receive
// notifications when changes occur to a file system path.
//
// A typical use case is to use a watcher pairs with a cache, reconstructing
// the cache when an update is detected.
type Watcher struct {
	Events <-chan struct{}
	Errors <-chan error

	once sync.Once
	join sync.WaitGroup
	done chan struct{}
}

// Close stops the watcher.
func (w *Watcher) Close() error {
	w.once.Do(func() { close(w.done) })
	w.join.Wait()
	return nil
}

// Watch creates a watcher which gets notifications when changes are made to the
// mount point.
//
// The period passed as argument represents the minimal time interval between
// updates reported by the watcher. This is useful to avoid receiving too many
// notifications if the mount point is being modified repeatedly; all events
// occuring within a period will be merged into one.
func (path MountPoint) Watch(period time.Duration) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := w.Add(filepath.Dir(string(path))); err != nil {
		w.Close()
		return nil, err
	}

	evnch := make(chan struct{})
	errch := make(chan error)

	watcher := &Watcher{
		Events: evnch,
		Errors: errch,
		done:   make(chan struct{}),
	}

	watcher.join.Add(1)
	go func() {
		defer watcher.join.Done()
		defer w.Close()

		onReady := func() {
			select {
			case evnch <- struct{}{}:
			case <-watcher.done:
			}
		}

		onError := func(err error) {
			select {
			case errch <- err:
			case <-watcher.done:
			}
		}

		notify := false
		ticker := time.NewTicker(period)
		defer ticker.Stop()

		base := filepath.Base(string(path))
		for {
			select {
			case e := <-w.Events:
				if filepath.Base(e.Name) == base {
					notify = true
				}

			case e := <-w.Errors:
				onError(e)

			case <-ticker.C:
				if notify {
					onReady()
					notify = false
				}

			case <-watcher.done:
				return
			}
		}
	}()

	return watcher, nil
}

// Wait blocks until the path exists or ctx is cancelled.
func (path MountPoint) Wait(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		switch _, err := os.Stat(string(path)); {
		case err == nil:
			return nil
		case !os.IsNotExist(err):
			return err
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
