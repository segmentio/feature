package feature

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
func (path MountPoint) Watch() (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := w.Add(string(path)); err != nil {
		w.Close()
		return nil, err
	}

	target, err := filepath.EvalSymlinks(string(path))
	if err != nil {
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

		for {
			select {
			case e := <-w.Events:
				if strings.HasSuffix(e.Name, target) {
					newTarget, err := filepath.EvalSymlinks(string(path))
					if err != nil {
						onError(err)
						continue
					}

					if newTarget == target {
						continue
					}

					if err := w.Add(string(path)); err != nil {
						onError(err)
						continue
					}

					target = newTarget
					onReady()
				}

			case e := <-w.Errors:
				onError(e)

			case <-watcher.done:
				return
			}
		}
	}()

	return watcher, nil
}

// Wait blocks until the path exists or ctx is cancelled.
func (path MountPoint) Wait(ctx context.Context) error {
	const minDelay = 100 * time.Millisecond
	const maxDelay = 1 * time.Second
	delay := minDelay

	timer := time.NewTimer(delay)
	defer timer.Stop()

	for {
		switch _, err := os.Lstat(string(path)); {
		case err == nil:
			return nil
		case !os.IsNotExist(err):
			return err
		}

		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}

		if delay += minDelay; delay > maxDelay {
			delay = maxDelay
		}

		timer.Reset(delay)
	}
}
