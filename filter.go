package uniq

import (
	"crypto/sha1"
	"sync"
)

// filter is the interface implemented by types that evaluate a list of file
// paths looking for files with duplicate checksums.
//
// Uniq, Dup, and Err will be closed once all files have been evaluated, or if
// Cancel is called.
type filter interface {
	Start()
	Uniq() <-chan string // Outgoing file paths with previously-unseen checksums.
	Dup() <-chan string  // Outgoing file paths with previously-seen checksums.
	Err() <-chan error   // Outgoing errors.
	Sums() *Sums
	Cancel()
}

// chanFilter is an implementation of the filter interface for file paths read
// from a channel. It coordinates a set of worker goroutines that handle
// channel I/O, file checksums, and errors.
type chanFilter struct {
	opts *Options

	sums      *Sums
	bufs      *bufferPool
	numProcs  int            // Number of worker goroutines to start.
	busyProcs sync.WaitGroup // Coordinate active worker goroutines.

	in     <-chan string // Incoming file paths.
	uniq   chan string
	dup    chan string
	err    chan error
	cancel *signal // Signal cancellation.
}

var _ filter = (*chanFilter)(nil)

func newChanFilter(in <-chan string, numProcs int, opts *Options) *chanFilter {
	f := new(chanFilter)
	f.opts = opts
	f.sums = NewSums()
	f.bufs = newBufferPool()
	f.numProcs = numProcs
	f.in = in
	f.uniq = make(chan string, f.numProcs)
	f.dup = make(chan string, f.numProcs)
	f.err = make(chan error)
	f.cancel = newSignal()
	return f
}

func (f *chanFilter) Uniq() <-chan string { return f.uniq }

func (f *chanFilter) Dup() <-chan string { return f.dup }

func (f *chanFilter) Err() <-chan error { return f.err }

func (f *chanFilter) Sums() *Sums { return f.sums }

// Start launches worker goroutines and begins handling values received from
// f.in. Not to be called more than once on the same instance.
func (f *chanFilter) Start() {
	f.busyProcs.Add(f.numProcs)
	for i := 0; i < f.numProcs; i++ {
		go f.worker()
	}
	go func() {
		f.busyProcs.Wait()
		close(f.dup)
		close(f.err)
	}()
}

// Cancel signals worker goroutines to return the first time it is called.
// Subsequent calls to Cancel have no effect.
func (f *chanFilter) Cancel() {
	f.cancel.Once()
	f.busyProcs.Wait()
}

func (f *chanFilter) worker() {
	defer f.busyProcs.Done()
	for {
		select {
		case <-f.cancel.C():
			return
		case path, ok := <-f.in:
			if !ok { // f.in was closed: stop working.
				return
			}
			f.handle(path)
		}
	}
}

// handle reads the file located at path, computes and stores its checksum, and
// sends its path on f.Uniq or f.Dup, depending on whether its checksum has
// been previously seen.
func (f *chanFilter) handle(path string) {
	info, path, err := lstat(f.opts.fs, path, f.opts.FollowSymlinks)
	if err != nil {
		f.emitErr(err)
		return
	}
	if info.IsDir() {
		return
	}

	file, err := f.opts.fs.Open(path)
	if err != nil {
		f.emitErr(err)
		return
	}
	defer file.Close()

	buf := f.bufs.Get()
	defer f.bufs.Put(buf)

	_, err = buf.ReadFrom(file)
	if err != nil {
		f.emitErr(err)
		return
	}

	sum := sha1.Sum(buf.Bytes())
	dup := f.sums.Append(sum, &File{Path: path, Info: info})
	if dup {
		f.emitDup(path)
	} else {
		f.emitUniq(path)
	}
}

func (f *chanFilter) emitDup(path string) {
	select {
	case <-f.cancel.C():
	case f.dup <- path:
	}
}

func (f *chanFilter) emitUniq(path string) {
	select {
	case <-f.cancel.C():
	case f.uniq <- path:
	}
}

func (f *chanFilter) emitErr(err error) {
	select {
	case <-f.cancel.C():
	case f.err <- err:
	}
}

// dirFilter is an implementation of the filter interface for file paths read
// from a directory. It coordinates a dirReader and a chanFilter: it configures
// the output of the former as the input of the latter and forwards errors
// emitted by either on Err.
type dirFilter struct {
	r   *dirReader
	f   *chanFilter
	err <-chan error
}

var _ filter = (*dirFilter)(nil)

func newDirFilter(path string, opts *Options) *dirFilter {
	d := new(dirFilter)
	d.r = newDirReader(path, ratioMaxProcs(1, 4), opts)
	d.f = newChanFilter(d.r.out, ratioMaxProcs(3, 4), opts)
	d.err = mergeErrors(d.r.err, d.f.err)
	return d
}

func (d *dirFilter) Uniq() <-chan string { return d.f.Uniq() }

func (d *dirFilter) Dup() <-chan string { return d.f.Dup() }

func (d *dirFilter) Err() <-chan error { return d.err }

func (d *dirFilter) Sums() *Sums { return d.f.Sums() }

// Start instructs the dirReader and chanFilter managed by d to start. Not to
// be called more than once on the same instance.
func (d *dirFilter) Start() {
	d.r.Start()
	d.f.Start()
}

// Cancel interrupts the dirReader and chanFilter managed by d and waits for
// both to return.
func (d *dirFilter) Cancel() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		d.r.Cancel()
	}()
	go func() {
		defer wg.Done()
		d.f.Cancel()
	}()
	wg.Wait()
}
