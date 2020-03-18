package uniq

import (
	"path/filepath"
	"sync"
)

// dirReader concurrently reads the directory located at root and sends file
// paths on out, errors on err.
type dirReader struct {
	root string // Path of directory to be read.
	opts *Options

	numProcs  int            // Number of worker goroutines to start.
	busyProcs sync.WaitGroup // Coordinate active worker goroutines.

	queue    chan string    // Paths of directories to be read.
	busyDirs sync.WaitGroup // Coordinate active directories.
	out      chan string    // Outgoing file paths.
	err      chan error     // Outgoing errors.
	done     chan struct{}  // Signal worker goroutines to return.
	cancel   *signal        // Signal cancellation.
}

func newDirReader(path string, numProcs int, opts *Options) *dirReader {
	r := new(dirReader)
	r.root = path
	r.opts = opts
	r.numProcs = numProcs
	r.queue = make(chan string, r.numProcs)
	r.out = make(chan string, r.numProcs)
	r.err = make(chan error)
	r.done = make(chan struct{})
	r.cancel = newSignal()
	return r
}

// Start launches worker goroutines and begins reading the configured
// root directory. Not to be called more than once on the same instance.
func (r *dirReader) Start() {
	r.busyProcs.Add(r.numProcs)
	for i := 0; i < r.numProcs; i++ {
		go r.worker()
	}

	go func() {
		r.enqueue(r.root)
		r.busyDirs.Wait()

		close(r.done)      // r.queue is empty: signal worker goroutines to return
		r.busyProcs.Wait() // and wait for them.

		close(r.queue)
		close(r.out)
		close(r.err)
	}()
}

// Cancel signals worker goroutines to return immediately the first time it is
// called. Subsequent calls to Cancel have no effect.
func (r *dirReader) Cancel() {
	r.cancel.Once()
	r.busyDirs.Wait()
	r.busyProcs.Wait()
}

func (r *dirReader) worker() {
	defer r.busyProcs.Done()
	for {
		select {
		case <-r.cancel.C():
			return
		case <-r.done:
			return
		case path := <-r.queue:
			r.handle(path)
		}
	}
}

func (r *dirReader) enqueue(path string) {
	r.busyDirs.Add(1)

	select {
	case <-r.cancel.C():
		r.busyDirs.Done()
	case r.queue <- path:
	default: // r.queue is full: visit path synchronously.
		r.handle(path)
	}
}

// handle reads file names from the directory located at path and sends file
// paths on r.out. If path is "/dir" and a file is named "file1", "/dir/file1"
// is sent on r.out. If r.recursive is true and a sub-directory is encountered,
// it is enqueued for reading. If path is the location of a regular file
// instead of a directory, that file is sent on r.out and handle returns.
func (r *dirReader) handle(path string) {
	defer r.busyDirs.Done()

	info, path, err := lstat(r.opts.fs, path, r.opts.FollowSymlinks)
	if err != nil {
		r.emitErr(err)
		return
	}
	if !info.IsDir() {
		r.emit(path)
		return
	}

	names, err := r.opts.fs.Readdirnames(path)
	if err != nil {
		r.emitErr(err)
		return
	}

	for _, name := range names {
		select {
		case <-r.cancel.C():
			return
		default:
		}

		fullPath := filepath.Join(path, name)
		info, fullPath, err = lstat(r.opts.fs, fullPath, r.opts.FollowSymlinks)
		if err != nil {
			r.emitErr(err)
			continue
		}
		if !info.IsDir() {
			r.emit(fullPath)
		} else if r.opts.Recursive {
			r.enqueue(fullPath)
		}
	}
}

func (r *dirReader) emit(path string) {
	select {
	case <-r.cancel.C():
	case r.out <- path:
	}
}

func (r *dirReader) emitErr(err error) {
	select {
	case <-r.cancel.C():
	case r.err <- err:
	}
}
